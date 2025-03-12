package workshopstate

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"time"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/overlord/cmdstate"
	"github.com/canonical/workshop/internal/overlord/healthstate"
	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/ifacestate"
	"github.com/canonical/workshop/internal/overlord/sdkstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
)

const (
	// mark the last task in a taskset after which refresh becomes irreversible (i.e. the following tasks
	// will not be possible to undo, e.g. removing an old workshop copy)
	EdgeRefreshLastTaskBeforeCleanup = state.TaskSetEdge("last-before-irreversible")

	// mark the tasks that denote irreversible clean up logic for refresh (e.g.
	// removing state storage and the old workshop copy)
	EdgeRefreshFirstCleanupTask = state.TaskSetEdge("refresh-cleanup")
)

var checkHealthTimeout = 5 * time.Second

var (
	systemSetup = sdk.Setup{Name: sdk.System.String(), Revision: sdk.R(-1)}
	isSystem    = func(s sdk.Setup) bool { return s.Name == sdk.System.String() }
)

func ensureSystemFirst[T string | sdk.Setup](items []T, match func(T) bool, systemSdk T) []T {
	idx := slices.IndexFunc(items, match)
	if idx != -1 {
		items[0], items[idx] = items[idx], items[0]
	} else {
		items = slices.Insert(items, 0, systemSdk)
	}
	return items
}

func (w *WorkshopManager) loadProject(ctx context.Context, id string) (workshop.Project, error) {
	username, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return workshop.Project{}, fmt.Errorf("context key user not found")
	}

	projects, err := w.backend.Projects(ctx)
	if err != nil {
		return workshop.Project{}, err
	}

	idx := slices.IndexFunc(projects[username], func(p workshop.Project) bool { return p.ProjectId == id })
	if idx == -1 {
		return workshop.Project{}, fmt.Errorf("no project found with \"id\" %v", id)
	}
	return projects[username][idx], nil
}

func (w *WorkshopManager) LaunchMany(ctx context.Context, names []string, projectId string) ([]*state.TaskSet, error) {
	sto := sdk.StoreService(w.state)

	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	taskset := make([]*state.TaskSet, 0, len(names))
	var sdks []sdk.SdkResult
	for _, name := range names {
		// Make sure the workshop doesn't exist.
		_, err := w.Workshop(ctx, name, projectId)
		if err == nil {
			return nil, fmt.Errorf("cannot launch %q: workshop exists", name)
		} else if !errors.Is(err, workshop.ErrWorkshopNotLaunched) {
			return nil, fmt.Errorf("cannot launch %q, failed to check whether the workshop exists: %w", name, err)
		}

		file, err := project.Workshop(name)
		if err != nil {
			return nil, fmt.Errorf("cannot launch %q: %w", name, err)
		}

		sdks, err = sdkStoreInfo(sto, ctx, projectId, file)
		if err != nil {
			return nil, err
		}

		candidates := make([]sdk.Setup, 0, len(sdks))
		for _, s := range sdks {
			candidates = append(candidates, sdk.Setup{Name: s.Name, Channel: s.Channel, Revision: s.Revision})
		}
		candidates = ensureSystemFirst(candidates, isSystem, systemSetup)

		tasks := launch(w.state, file, candidates, project)
		taskset = append(taskset, tasks)
	}
	return taskset, nil
}

func sdkStoreInfo(sto sdk.Store, ctx context.Context, projectid string, file *workshop.File) ([]sdk.SdkResult, error) {
	acts := []sdk.SdkAction{}
	for _, sd := range file.Sdks {
		// "system" SDK is bootstrapped and installed by Workshop locally in a
		// separate task.
		if sd.Name == sdk.System.String() {
			continue
		}
		act := sdk.SdkAction{ProjectId: projectid, Workshop: file.Name, Name: sd.Name, Base: file.Base, Channel: sd.Channel, Action: sdk.Install}
		acts = append(acts, act)
	}
	return sto.SdkAction(ctx, acts)
}

func retrieveSdks(st *state.State, sdks []sdk.Setup) (*state.TaskSet, map[string]string) {
	retrieve := state.NewTaskSet()
	retrieveMap := map[string]string{}
	for _, s := range sdks {
		if s.Channel != "" {
			r := sdkstate.Retrieve(st, s)
			retrieve.AddTask(r)
			retrieveMap[s.Name] = r.ID()
		}
	}
	return retrieve, retrieveMap
}

func installSdks(st *state.State, sdks []sdk.Setup, retrieveTasks map[string]string) *state.TaskSet {
	var prevInstall *state.TaskSet
	all := state.NewTaskSet()
	addTaskSet := func(ts *state.TaskSet) {
		if len(ts.Tasks()) == 0 {
			return
		}

		if prevInstall != nil {
			ts.WaitAll(prevInstall)
		}
		prevInstall = ts

		all.AddAll(ts)
	}

	for _, setup := range sdks {
		// The install task sets must not run concurrently as exec ops are not
		// allowed by LXD to be run concurrently and in general case we cannot
		// guarantee safety of concurrent installations.
		var install *state.TaskSet
		if setup.Channel == "" {
			install = sdkstate.InstallLocalSdk(st, setup)
		} else {
			install = sdkstate.Install(st, setup.Name, retrieveTasks[setup.Name])
		}

		addTaskSet(install)
	}
	return all
}

func launchWorkshop(st *state.State, file *workshop.File, project workshop.Project) *state.TaskSet {
	construct := state.NewTaskSet()

	var prev *state.Task
	addTask := func(t *state.Task) {
		construct.AddTask(t)
		if prev != nil {
			t.WaitFor(prev)
		}
		prev = t
	}

	base := st.NewTask("download-base", fmt.Sprintf("Download %q base image", file.Base))
	base.Set("workshop-base", file.Base)
	addTask(base)

	create := st.NewTask("create-workshop", fmt.Sprintf("Create new %q workshop", file.Name))
	addTask(create)
	create.Set("workshop-file", file)

	// We're rebuilding the workshop from base, which means, the mounts will
	// be removed too.
	mountProject := st.NewTask("mount-project", fmt.Sprintf("Mount project directory %q", project.Path))
	addTask(mountProject)

	mountAptCache := st.NewTask("mount-apt-cache", fmt.Sprintf("Mount apt cache directory %q", dirs.AptCachePath))
	addTask(mountAptCache)

	start := st.NewTask("start-workshop", fmt.Sprintf("Start %q workshop", file.Name))
	addTask(start)

	return construct
}

func rebuildWorkshop(st *state.State, file *workshop.File, sdkSnapshot string) *state.TaskSet {
	construct := state.NewTaskSet()

	var prev *state.Task
	addTask := func(t *state.Task) {
		construct.AddTask(t)
		if prev != nil {
			t.WaitFor(prev)
		}
		prev = t
	}

	base := st.NewTask("download-base", fmt.Sprintf("Download %q base image", file.Base))
	base.Set("workshop-base", file.Base)
	addTask(base)

	create := st.NewTask("create-workshop", fmt.Sprintf("Create new %q workshop", file.Name))
	addTask(create)
	create.Set("workshop-file", file)

	if sdkSnapshot != "" {
		create.Set("sdk-snapshot", sdkSnapshot)
	}

	start := st.NewTask("start-workshop", fmt.Sprintf("Start %q workshop", file.Name))
	addTask(start)

	return construct
}

func launch(st *state.State, file *workshop.File, sdks []sdk.Setup, project workshop.Project) *state.TaskSet {
	var prevInstall *state.TaskSet
	all := state.NewTaskSet()

	addTaskSet := func(ts *state.TaskSet) {
		if len(ts.Tasks()) == 0 {
			return
		}

		if prevInstall != nil {
			ts.WaitAll(prevInstall)
		}
		prevInstall = ts

		all.AddAll(ts)
	}

	retrieve, rmap := retrieveSdks(st, sdks)
	addTaskSet(retrieve)

	createAptCache := st.NewTask("create-apt-cache", fmt.Sprintf("Create apt cache for %q", file.Name))
	addTaskSet(state.NewTaskSet(createAptCache))

	create := launchWorkshop(st, file, project)
	addTaskSet(create)

	install := installSdks(st, sdks, rmap)
	addTaskSet(install)

	setup := runHooks(st, project.ProjectId, file.Name, sdks, 0, hookstate.SetupBase)
	addTaskSet(setup)

	connect := autoconnectSdks(st, sdks)
	addTaskSet(connect)

	checkHealth := runHooks(st, project.ProjectId, file.Name, sdks, checkHealthTimeout, hookstate.CheckHealth)
	addTaskSet(checkHealth)

	for _, task := range all.Tasks() {
		task.Set("workshop", file.Name)
		task.Set("project", project)
	}

	return all
}

type refreshWorkshopReq struct {
	// Existing workshop that will be refreshed.
	w *workshop.Workshop
	// Up to date workshop definitions from the project directory.
	file *workshop.File
}

func (w *WorkshopManager) RefreshMany(ctx context.Context, projectId string, names []string) ([]*state.TaskSet, error) {
	sto := sdk.StoreService(w.state)

	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	all, err := w.Workshops(ctx, projectId)
	if err != nil {
		return nil, err
	}

	refreshReqs := make([]refreshWorkshopReq, 0, len(all))
	allowed := []healthstate.Status{healthstate.ReadyStatus}
	for _, name := range names {
		idx := slices.IndexFunc(all, func(w *workshop.Workshop) bool { return w.Name == name })
		if idx == -1 {
			return nil, fmt.Errorf("cannot refresh %q: %w", name, workshop.ErrWorkshopNotLaunched)
		}

		if err = healthstate.CheckWorkshopHealth(w.state, all[idx], allowed); err != nil {
			return nil, fmt.Errorf("cannot refresh %q: %w", name, err)
		}

		file, err := project.Workshop(name)
		if err != nil {
			return nil, fmt.Errorf("cannot refresh %q: %w", name, err)
		}
		refreshReqs = append(refreshReqs, refreshWorkshopReq{w: all[idx], file: file})
	}

	taskset := make([]*state.TaskSet, 0, len(refreshReqs))
	for _, req := range refreshReqs {
		w.state.Unlock()
		plan, err := resolveRefresh(ctx, sto, req.w, req.file)
		w.state.Lock()
		if err != nil {
			return nil, err
		}

		tasks, err := refresh(ctx, w.state, plan, req.w, req.file)
		if err != nil {
			return nil, fmt.Errorf("cannot refresh %q: %w", req.w.Name, err)
		}
		if len(tasks.Tasks()) == 0 {
			continue
		}
		taskset = append(taskset, tasks)
	}

	for _, ts := range taskset {
		cleanup := ts.MaybeEdge(EdgeRefreshFirstCleanupTask)
		if cleanup == nil {
			continue
		}

		// We will iterate over other refreshes and make sure that the cleanup
		// task of our refresh will wait until all the other refresh operations
		// finished. This will ensure that we start to remove the workshops'
		// previous copies once all the refresh operations were successful (at
		// this stage, we only need to remove a stashed copy, the newly refreshed
		// workshop is already up and running). Thus, every CleanupEdge will
		// wait for ALL the LastBeforeRefreshIrreversibleEdge tasks of all the
		// other changes before execution.
		for _, otherts := range taskset {
			if ts != otherts {
				last, err := otherts.Edge(EdgeRefreshLastTaskBeforeCleanup)
				if err != nil {
					return nil, err
				}
				cleanup.WaitFor(last)
			}
		}
	}

	return taskset, nil
}

func maybeSketch(ctx context.Context, pid, wp string) (bool, error) {
	username, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return false, fmt.Errorf("context key user not found")
	}

	usr, env, err := osutil.UserAndEnv(username)
	if err != nil {
		return false, err
	}

	userDataDir := workshop.UserDataRootDir(usr.HomeDir, env)
	sketchdir := workshop.SketchSdkCurrent(userDataDir, pid, wp)

	recs, err := os.ReadDir(sketchdir)
	// no Sketch SDK exists for the workshop and it is not an error.
	if len(recs) == 0 || osutil.IsDirNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func maybeRefresh(installed sdk.Setup, candidate sdk.Setup) bool {
	return installed.Channel != candidate.Channel || installed.Revision != candidate.Revision
}

type refreshPlan struct {
	install []sdk.Setup
	intact  []sdk.Setup
	refresh []sdk.Setup
	remove  []sdk.Setup

	sdkSnapshot  string
	installOrder []string
	fullRefresh  bool
}

func (p refreshPlan) ordered(setups ...[]sdk.Setup) []sdk.Setup {
	ordered := []sdk.Setup{}

	for _, sk := range p.installOrder {
		for _, setup := range setups {
			contains := func(sp sdk.Setup) bool { return sk == sp.Name }

			idx := slices.IndexFunc(setup, contains)
			if idx != -1 {
				ordered = append(ordered, setup[idx])
			}
		}
	}
	return ordered
}

func (p refreshPlan) InstallOrRefresh() []sdk.Setup {
	return p.ordered(p.install, p.refresh)
}

func (p refreshPlan) IntactOrRemove() []sdk.Setup {
	ordered := p.ordered(p.intact)
	ordered = append(ordered, p.remove...)
	return ordered
}

func (p refreshPlan) InstallIntactOrRefresh() []sdk.Setup {
	return p.ordered(p.install, p.refresh, p.intact)
}

func (p refreshPlan) Refresh() []sdk.Setup {
	return p.ordered(p.refresh)
}

func (p refreshPlan) Remove() []sdk.Setup {
	return p.remove
}

func resolveRefresh(ctx context.Context, sto sdk.Store, w *workshop.Workshop, file *workshop.File) (*refreshPlan, error) {
	plan := &refreshPlan{
		install:      make([]sdk.Setup, 0),
		refresh:      make([]sdk.Setup, 0),
		remove:       make([]sdk.Setup, 0),
		installOrder: make([]string, 0),
	}

	infos, err := sdkStoreInfo(sto, ctx, w.Project.ProjectId, file)
	if err != nil {
		return nil, err
	}

	candidates := make([]sdk.Setup, 0, len(infos))
	for _, s := range infos {
		candidates = append(candidates, sdk.Setup{Name: s.Name, Channel: s.Channel, Revision: s.Revision})
	}
	candidates = ensureSystemFirst(candidates, isSystem, systemSetup)

	// Restore the order of SDKs installed in the running workshop.
	prevOrder := []string{}
	for _, s := range w.File.Sdks {
		prevOrder = append(prevOrder, s.Name)
	}
	prevOrder = ensureSystemFirst(prevOrder,
		func(s string) bool { return s == sdk.System.String() },
		sdk.System.String())

	// Determine if a workshop can be partially updated.
	lastIntact := 0
	for ni, s := range candidates {
		// Do we have this SDK in the same order as in the running workshop?
		if ni < len(prevOrder) && prevOrder[ni] == s.Name {
			// Has this SDK had any updates?
			if !maybeRefresh(w.Sdks[s.Name], s) {
				plan.intact = append(plan.intact, s)
				// No updates to the SDK - reuse its snapshot and keep looking.
				// Otherwise, break the loop as the rest require to be reinstalled.
				plan.sdkSnapshot = s.Name
				lastIntact = ni
				continue
			}
			break
		}
	}

	for _, s := range candidates[lastIntact+1:] {
		if installed, exist := w.Sdks[s.Name]; exist {
			plan.refresh = append(plan.refresh, s)
			plan.remove = append(plan.remove, installed)
		} else {
			plan.install = append(plan.install, s)
		}
	}

	// SDKs that only exist in the previous workshop are to be removed.
	for _, rec := range w.Sdks {
		if !slices.ContainsFunc(candidates, func(r sdk.Setup) bool {
			return r.Name == rec.Name
		}) {
			plan.remove = append(plan.remove, rec)
		}
	}

	if w.Base != file.Base {
		plan.fullRefresh = true
	}

	if len(plan.refresh) == 0 && len(plan.remove) == 0 && len(plan.install) == 0 {
		plugsSlotsChanged := func(s1, s2 workshop.SdkRecord) bool {
			return reflect.DeepEqual(s1.Plugs, s2.Plugs) &&
				reflect.DeepEqual(s1.Slots, s2.Slots)
		}
		if !slices.EqualFunc(w.File.Sdks, file.Sdks, plugsSlotsChanged) {
			plan.fullRefresh = true
		}
	}

	// Establish SDK installation order.
	for _, s := range candidates {
		plan.installOrder = append(plan.installOrder, s.Name)
	}

	sketchFound, err := maybeSketch(ctx, w.Project.ProjectId, w.Name)
	if err != nil {
		return nil, err
	}
	if sketchFound {
		plan.installOrder = append(plan.installOrder, sdk.Sketch)

		if installed, exist := w.Sdks[sdk.Sketch]; exist {
			plan.refresh = append(plan.refresh, sdk.Setup{
				Name:     sdk.Sketch,
				Revision: sdk.Revision{N: installed.Revision.N - 1},
			})
		} else {
			plan.install = append(plan.install,
				sdk.Setup{Name: sdk.Sketch, Revision: sdk.Revision{N: -1}})
		}
	}

	// No appropriate snapshots found means the workshop needs to be rebuilt
	// from scratch.
	if plan.fullRefresh || plan.sdkSnapshot == "" {
		plan.refresh = append(plan.refresh, plan.intact...)
		plan.remove = append(plan.remove, plan.intact...)
		plan.intact = plan.intact[:0]
		plan.sdkSnapshot = ""
	}
	return plan, nil
}

func refresh(ctx context.Context, st *state.State, plan *refreshPlan, w *workshop.Workshop, file *workshop.File) (*state.TaskSet, error) {
	refresh := state.NewTaskSet()
	prev := (*state.TaskSet)(nil)
	addTaskSet := func(ts *state.TaskSet) {
		if len(ts.Tasks()) == 0 {
			return
		}

		if prev != nil {
			ts.WaitAll(prev)
		}
		refresh.AddAll(ts)
		prev = ts
	}

	retrieve, rmap := retrieveSdks(st, plan.InstallOrRefresh())
	addTaskSet(retrieve)

	if len(plan.Refresh()) > 0 {
		stateStorage := st.NewTask("create-state-storage", "Create SDK state storage")
		addTaskSet(state.NewTaskSet(stateStorage))
	}

	// Call save-state hooks for the SDKs that are installed and will not be
	// removed after this refresh.
	saveState := runHooks(st, w.Project.ProjectId, file.Name, plan.Refresh(), 0, hookstate.SaveState)
	addTaskSet(saveState)

	disconnect := disconnectSdks(st, plan.IntactOrRemove())
	addTaskSet(disconnect)

	// Unlink and remove SDKs from interfaces repository. If refresh fails, the
	// SDKs will be linked back and returned to the repository. This is the
	// reason unlink has to happen before stashing. As the workshop, that will
	// be reconstructed after stashing, will link and add their SDKs to the
	// repository in `installSdks` step.
	unlink := unlinkSdks(st, plan.Remove())
	addTaskSet(unlink)

	stash := st.NewTask("stash-workshop", fmt.Sprintf("Stash previous %q workshop", file.Name))
	addTaskSet(state.NewTaskSet(stash))

	rebuild := rebuildWorkshop(st, file, plan.sdkSnapshot)
	addTaskSet(rebuild)

	// Detach (uninstall) volumes of the SDKs from the restored snapshot. If the
	// refresh fails, a copy of the workshop will be restored that had SDKs
	// installed previously.
	// We do not uninstall the SDKs that were installed locally and are to be
	// removed here as those will not present in the recovered snapshot (as
	// those are simply copied over into the workshop). These tasks will uninstall
	// SDKs that were installed from the store as SDK volumes.
	uninstall := uninstallSdks(st, plan.Remove())
	addTaskSet(uninstall)

	install := installSdks(st, plan.InstallOrRefresh(), rmap)
	addTaskSet(install)

	setup := runHooks(st, w.Project.ProjectId, file.Name, plan.InstallOrRefresh(), 0, hookstate.SetupBase)
	addTaskSet(setup)

	connect := autoconnectSdks(st, plan.InstallIntactOrRefresh())
	addTaskSet(connect)

	restoreState := runHooks(st, w.Project.ProjectId, file.Name, plan.Refresh(), 0, hookstate.RestoreState)
	addTaskSet(restoreState)

	checkHealth := runHooks(st, w.Project.ProjectId, file.Name, plan.InstallOrRefresh(), 0, hookstate.CheckHealth)
	addTaskSet(checkHealth)

	length := len(refresh.Tasks())
	last := refresh.Tasks()[length-1]
	refresh.MarkEdge(last, EdgeRefreshLastTaskBeforeCleanup)

	cleanupLane := st.NewLane()

	if len(plan.Refresh()) > 0 {
		removeStateStorage := st.NewTask("remove-state-storage", "Remove SDK state storage")
		removeStateStorage.WaitFor(last)
		removeStateStorage.JoinLane(cleanupLane)
		refresh.MarkEdge(removeStateStorage, EdgeRefreshFirstCleanupTask)

		refresh.AddTask(removeStateStorage)
	}

	// remove the workshop from stash after the state storage was detached
	removeStash := st.NewTask("remove-workshop-stash", fmt.Sprintf("Remove %q workshop from stash", file.Name))
	// if the change was aborted during the cleanup stage execution,
	// there is a chance that some of the workshop copies that had
	// been created during the refresh were already deleted. If we
	// start to Undo those workshops' refresh progress we will
	// endup deleting the workshops that finished their refresh.
	// Given that they have no copy already, the undo logic
	// (stash-workshop) will delete the existing workshop
	// and fail to restore from the copy. We don't want that. Hence,
	// all the cleanup tasks are extracted into a separate lane. If
	// any problem happens, the workshops that had finished their
	// refresh will not be affected.
	removeStash.JoinLane(cleanupLane)
	removeStash.WaitFor(last)

	if refresh.MaybeEdge(EdgeRefreshFirstCleanupTask) == nil {
		refresh.MarkEdge(removeStash, EdgeRefreshFirstCleanupTask)
	}

	refresh.AddTask(removeStash)

	for _, task := range refresh.Tasks() {
		task.Set("workshop", file.Name)
		task.Set("project", w.Project)
	}

	return refresh, nil
}

func (w *WorkshopManager) RefreshLocalSdk(ctx context.Context, pid string, wpn string, sdkn string) ([]*state.TaskSet, error) {
	var taskset []*state.TaskSet
	wp, err := w.Workshop(ctx, wpn, pid)
	if err != nil {
		return nil, err
	}
	allowed := []healthstate.Status{healthstate.ReadyStatus}
	if err = healthstate.CheckWorkshopHealth(w.state, wp, allowed); err != nil {
		return nil, fmt.Errorf("cannot refresh %q: %w", wpn, err)
	}

	sto := sdk.StoreService(w.state)
	w.state.Unlock()
	plan, err := resolveRefresh(ctx, sto, wp, wp.File)
	w.state.Lock()
	if err != nil {
		return nil, err
	}

	ts, err := refresh(ctx, w.state, plan, wp, wp.File)
	if err != nil {
		return nil, err
	}
	taskset = append(taskset, ts)
	return taskset, nil
}

func autoconnectSdks(st *state.State, sdks []sdk.Setup) *state.TaskSet {
	autoconnectSet := state.NewTaskSet()
	var prevAuto = (*state.Task)(nil)
	for _, setup := range sdks {
		autoconnect := st.NewTask("auto-connect", fmt.Sprintf("Auto-connect interfaces of %q SDK", setup.Name))
		autoconnect.Set("sdk", setup.Name)
		autoconnectSet.AddTask(autoconnect)
		if prevAuto != nil {
			autoconnect.WaitFor(prevAuto)
		}
		prevAuto = autoconnect
	}
	return autoconnectSet
}

func uninstallSdks(st *state.State, sdks []sdk.Setup) *state.TaskSet {
	var prevRemove *state.Task
	all := state.NewTaskSet()
	addTask := func(ts *state.Task) {
		if prevRemove != nil {
			ts.WaitFor(prevRemove)
		}
		prevRemove = ts

		all.AddTask(ts)
	}

	for _, setup := range sdks {
		// The install task sets must not run concurrently as exec ops are not
		// allowed by LXD to be run concurrently and in general case we cannot
		// guarantee safety of concurrent installations.
		if setup.Channel != "" {
			install := st.NewTask("remove-sdk", fmt.Sprintf("Remove %q SDK", setup.Name))
			install.Set("sdk-retrieve-task", install.ID())
			install.Set("sdk-setup", setup)
			addTask(install)
		}
	}
	return all
}

func unlinkSdks(st *state.State, sdks []sdk.Setup) *state.TaskSet {
	prev := (*state.Task)(nil)
	unlinkSet := state.NewTaskSet()
	for _, s := range sdks {
		unlink := st.NewTask("unlink-sdk", fmt.Sprintf("Unlink %q SDK", s.Name))
		unlink.Set("sdk-retrieve-task", unlink.ID())
		unlink.Set("sdk-setup", s)
		unlinkSet.AddTask(unlink)

		if prev != nil {
			unlink.WaitFor(prev)
		}
		prev = unlink
	}
	return unlinkSet
}

func disconnectSdks(st *state.State, sdks []sdk.Setup) *state.TaskSet {
	prev := (*state.Task)(nil)
	disconnSet := state.NewTaskSet()
	for _, s := range sdks {
		disconn := st.NewTask("auto-disconnect", fmt.Sprintf("Disconnect interfaces of %q SDK", s.Name))
		disconn.Set("sdk", s.Name)
		disconnSet.AddTask(disconn)

		if prev != nil {
			disconn.WaitFor(prev)
		}
		prev = disconn
	}
	return disconnSet
}

func runHooks(st *state.State, pid, w string, installed []sdk.Setup, timeout time.Duration, hooktype hookstate.WorkshopHookType) *state.TaskSet {
	hooks := state.NewTaskSet()
	prev := (*state.Task)(nil)
	for _, sk := range installed {
		hook := hookstate.Hook(st, pid, w, sk.Name, timeout, hooktype)
		hooks.AddTask(hook)
		if prev != nil {
			hook.WaitFor(prev)
		}
		prev = hook
	}
	return hooks
}

func (w *WorkshopManager) StartMany(ctx context.Context, names []string, projectId string) ([]*state.TaskSet, error) {
	// check if all the workshops are stopped
	for _, name := range names {
		wp, err := w.Workshop(ctx, name, projectId)
		if err != nil {
			return nil, fmt.Errorf("cannot start %q: %w", name, err)
		}
		allowed := []healthstate.Status{healthstate.StoppedStatus}
		if err = healthstate.CheckWorkshopHealth(w.state, wp, allowed); err != nil {
			return nil, fmt.Errorf("cannot start %q: %w", name, err)
		}
	}

	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}
	taskset, err := startMany(w.state, names, project)
	if err != nil {
		return nil, err
	}
	return taskset, nil
}

func startMany(st *state.State, names []string, project workshop.Project) ([]*state.TaskSet, error) {
	taskset := []*state.TaskSet{}

	for _, name := range names {
		start := st.NewTask("start-workshop", fmt.Sprintf("Start %q workshop", name))
		start.Set("workshop", name)
		start.Set("project", project)

		taskset = append(taskset, state.NewTaskSet(start))
	}

	return taskset, nil
}

func (w *WorkshopManager) StopMany(ctx context.Context, names []string, projectId string) ([]*state.TaskSet, error) {
	for _, name := range names {
		wp, err := w.Workshop(ctx, name, projectId)
		if err != nil {
			return nil, fmt.Errorf("cannot stop %q: %w", name, err)
		}
		allowed := []healthstate.Status{healthstate.ReadyStatus, healthstate.StoppedStatus}
		if err = healthstate.CheckWorkshopHealth(w.state, wp, allowed); err != nil {
			return nil, fmt.Errorf("cannot stop %q: %w", name, err)
		}
	}

	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}
	taskset, err := stopMany(w.state, names, project)
	if err != nil {
		return nil, err
	}
	return taskset, nil
}

func stopMany(st *state.State, names []string, project workshop.Project) ([]*state.TaskSet, error) {
	taskset := []*state.TaskSet{}

	for _, name := range names {
		stop := st.NewTask("stop-workshop", fmt.Sprintf("Stop %q workshop", name))
		stop.Set("force", false)
		stop.Set("workshop", name)
		stop.Set("project", project)

		taskset = append(taskset, state.NewTaskSet(stop))
	}

	return taskset, nil
}

func (w *WorkshopManager) Exec(ctx context.Context, name, projectId string, args *workshop.ExecArgs, script bool) (*state.TaskSet, error) {
	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	ctx = context.WithValue(ctx, workshop.ContextProjectId, project.ProjectId)
	wp, err := w.backend.Workshop(ctx, name)
	if err != nil {
		return nil, err
	}
	allowed := []healthstate.Status{healthstate.ReadyStatus, healthstate.WaitingStatus}
	if err = healthstate.CheckWorkshopHealth(w.state, wp, allowed); err != nil {
		return nil, err
	}

	wrkspc, err := w.backend.WorkshopFs(ctx, name)
	if err != nil {
		return nil, err
	}
	defer wrkspc.Close()

	info, err := wrkspc.Stat(args.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("working directory %q not found", args.WorkDir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", args.WorkDir)
	}

	var execSet *state.TaskSet
	if script {
		name := args.Command[0]
		cp := w.state.NewTask("install-script", fmt.Sprintf("Install script %q", name))
		exec := w.state.NewTask("exec", fmt.Sprintf("Exec script %q", name))

		// install-script will modify args and pass it to exec.
		w.state.Cache(cmdstate.ExecArgsKey(cp.ID()), args)
		cp.Set("exec-task", exec.ID())

		exec.WaitFor(cp)
		execSet = state.NewTaskSet(cp, exec)
	} else {
		exec := w.state.NewTask("exec", fmt.Sprintf("Exec command %q", args.Command[0]))

		w.state.Cache(cmdstate.ExecArgsKey(exec.ID()), args)

		execSet = state.NewTaskSet(exec)
	}

	for _, task := range execSet.Tasks() {
		task.Set("workshop", name)
		task.Set("project", project)
	}
	return execSet, nil
}

func (w *WorkshopManager) RemoveMany(ctx context.Context, names []string, projectId string) ([]*state.TaskSet, error) {
	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	ctx = context.WithValue(ctx, workshop.ContextProjectId, project.ProjectId)

	var workshops = make([]*workshop.Workshop, 0, len(names))
	for _, name := range names {
		wp, err := w.backend.Workshop(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("cannot remove %q: %w", name, err)
		}
		workshops = append(workshops, wp)

		allowed := []healthstate.Status{healthstate.ReadyStatus, healthstate.ErrorStatus, healthstate.StoppedStatus}
		if err = healthstate.CheckWorkshopHealth(w.state, wp, allowed); err != nil {
			return nil, fmt.Errorf("cannot remove %q: %w", name, err)
		}
	}

	taskset, err := removeMany(w.state, workshops, project)
	if err != nil {
		return nil, err
	}
	return taskset, nil
}

func removeMany(st *state.State, workshops []*workshop.Workshop, project workshop.Project) ([]*state.TaskSet, error) {
	taskset := []*state.TaskSet{}
	for _, name := range workshops {
		remove, err := remove(st, name, project)
		if err != nil {
			return nil, err
		}
		taskset = append(taskset, remove)
	}
	return taskset, nil
}

func remove(st *state.State, w *workshop.Workshop, project workshop.Project) (*state.TaskSet, error) {
	removeSet := state.NewTaskSet()
	var prevRemove *state.TaskSet
	addTaskSet := func(ts *state.TaskSet) {
		if len(ts.Tasks()) == 0 {
			return
		}
		if prevRemove != nil {
			ts.WaitAll(prevRemove)
		}
		prevRemove = ts
		removeSet.AddAll(ts)
	}

	disconnectSet := disconnectSdks(st, slices.Collect(maps.Values(w.Sdks)))
	addTaskSet(disconnectSet)

	discard := st.NewTask("discard-conns", fmt.Sprintf("Discard %q undesired connections", w.Name))
	addTaskSet(state.NewTaskSet(discard))

	unlink := unlinkSdks(st, slices.Collect(maps.Values(w.Sdks)))
	addTaskSet(unlink)

	remove := st.NewTask("remove-workshop", fmt.Sprintf("Remove %q workshop", w.Name))
	addTaskSet(state.NewTaskSet(remove))

	removeAptCache := st.NewTask("remove-apt-cache", fmt.Sprintf("Remove apt cache for %q", w.Name))
	addTaskSet(state.NewTaskSet(removeAptCache))

	// The apt cache cannot be removed until the workshop has stopped, which
	// currently happens as part of remove-workshop. Since there is no way to
	// undo remove-workshop, we run remove-apt-cache in a separate lane. If an
	// error occurs when removing the cache, it will not affect the other tasks.
	cleanupLane := st.NewLane()
	removeAptCache.JoinLane(cleanupLane)

	for _, task := range removeSet.Tasks() {
		task.Set("workshop", w.Name)
		task.Set("project", project)
	}
	return removeSet, nil
}

func (w *WorkshopManager) Remount(ctx context.Context, st *state.State, plug sdk.PlugRef, source string) (*state.TaskSet, error) {
	if !filepath.IsAbs(source) {
		return nil, fmt.Errorf("cannot remount: the `source` path must be absolute")
	}

	source = filepath.Clean(source)

	project, err := w.loadProject(ctx, plug.ProjectId)
	if err != nil {
		return nil, err
	}

	wp, err := w.Workshop(ctx, plug.Workshop, plug.ProjectId)
	if err != nil {
		return nil, fmt.Errorf("cannot load workshop %q: %w", plug.Workshop, err)
	}

	allowed := []healthstate.Status{healthstate.ReadyStatus, healthstate.StoppedStatus}
	if err = healthstate.CheckWorkshopHealth(w.state, wp, allowed); err != nil {
		return nil, fmt.Errorf("cannot remount %q: %w", plug.ShortRef(), err)
	}

	master, _ := ifacestate.MaybeBound(wp, plug)

	remount := st.NewTask("remount", fmt.Sprintf(`Remount %q`, plug.ShortRef()))
	remount.Set("workshop", plug.Workshop)
	remount.Set("project", project)
	remount.Set("plug", master)
	remount.Set("host-source", source)

	return state.NewTaskSet(remount), nil
}
