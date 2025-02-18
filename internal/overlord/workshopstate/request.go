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
	EdgeLastTaskBeforeRefreshIrreversible = state.TaskSetEdge("last-before-irreversible")

	// mark the tasks that denote irreversible clean up logic for refresh (e.g.
	// removing state storage and the old workshop copy)
	EdgeRefreshCleanup = state.TaskSetEdge("refresh-cleanup")
)

var checkHealthTimeout = 5 * time.Second

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

		setups := make([]sdk.Setup, 0, len(sdks))
		for _, s := range sdks {
			setups = append(setups, sdk.Setup{Name: s.Name, Channel: s.Channel, Revision: s.Revision})
		}

		tasks := launch(w.state, file, setups, project)
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

func retrieveSdks(st *state.State, sdks []sdk.Setup) *state.TaskSet {
	retrieve := state.NewTaskSet()
	for _, s := range sdks {
		if s.Channel != "" {
			r := sdkstate.Retrieve(st, s)
			retrieve.AddTask(r)
		}
	}
	return retrieve
}

func installSdks(st *state.State, w string, sdks []sdk.Setup, retrieveSet *state.TaskSet) *state.TaskSet {
	var prevInstall *state.TaskSet
	install := state.NewTaskSet()

	var prevSetup *state.Task
	setupHook := state.NewTaskSet()

	for idx, setup := range sdks {
		// The install task sets must not run concurrently as exec ops are not
		// allowed by LXD to be run concurrently and in general case we cannot
		// guarantee safety of concurrent installations.
		var installTs *state.TaskSet
		if setup.Channel != "" {
			installTs = sdkstate.Install(st, setup.Name, retrieveSet.Tasks()[idx].ID())
		} else {
			installTs = sdkstate.InstallLocalSdk(st, setup)
		}

		if prevInstall != nil {
			installTs.WaitAll(prevInstall)
		}
		prevInstall = installTs
		install.AddAll(installTs)

		// Make sure that the hook tasks are not concurrent
		setupHookTask := hookstate.Hook(st, w, setup.Name, hookstate.SetupBase)
		if prevSetup != nil {
			setupHookTask.WaitFor(prevSetup)
		}
		prevSetup = setupHookTask
		setupHook.AddTask(setupHookTask)
	}
	setupHook.WaitAll(install)

	all := state.NewTaskSet(install.Tasks()...)
	all.AddAll(setupHook)

	return all
}

func checkSdksHealth(st *state.State, w string, sdks []sdk.Setup) *state.TaskSet {
	var prev *state.Task
	checkHealth := state.NewTaskSet()
	for _, sk := range sdks {
		check := hookstate.HookWithTimeout(st, w, sk.Name, hookstate.CheckHealth, checkHealthTimeout)
		if prev != nil {
			check.WaitFor(prev)
		}
		prev = check
		checkHealth.AddTask(check)
	}
	return checkHealth
}

func constructWorkshop(st *state.State, file *workshop.File, project workshop.Project) *state.TaskSet {
	base := st.NewTask("download-base", fmt.Sprintf("Download %q base image", file.Base))
	base.Set("workshop-base", file.Base)

	create := st.NewTask("create-workshop", fmt.Sprintf("Create new %q workshop", file.Name))
	create.Set("workshop-file", file)
	create.WaitFor(base)

	mountProject := st.NewTask("mount-project", fmt.Sprintf("Mount project directory %q", project.Path))
	mountProject.WaitFor(create)

	mountAptCache := st.NewTask("mount-apt-cache", fmt.Sprintf("Mount apt cache directory %q", dirs.AptCachePath))
	mountAptCache.WaitFor(mountProject)

	start := st.NewTask("start-workshop", fmt.Sprintf("Start %q workshop", file.Name))
	start.WaitFor(mountAptCache)
	return state.NewTaskSet(base, create, mountProject, mountAptCache, start)
}

func launch(st *state.State, file *workshop.File, sdks []sdk.Setup, project workshop.Project) *state.TaskSet {
	retrieve := retrieveSdks(st, sdks)

	createAptCache := st.NewTask("create-apt-cache", fmt.Sprintf("Create apt cache for %q", file.Name))

	create := constructWorkshop(st, file, project)
	create.WaitAll(retrieve)
	create.WaitFor(createAptCache)

	system := sdkstate.InstallSystemSdk(st)
	system.WaitAll(create)

	install := installSdks(st, file.Name, sdks, retrieve)
	install.WaitAll(system)

	full := slices.Clone(sdks)
	full = slices.Insert(full, 0, sdk.Setup{Name: sdk.System.String(), Revision: sdk.Revision{N: -1}})

	connect := autoconnectSdks(st, full)
	connect.WaitAll(install)
	connect.WaitAll(system)

	checkHealth := checkSdksHealth(st, file.Name, sdks)
	checkHealth.WaitAll(connect)

	all := state.NewTaskSet(retrieve.Tasks()...)
	all.AddAll(retrieve)
	all.AddTask(createAptCache)
	all.AddAll(create)
	all.AddAll(system)
	all.AddAll(install)
	all.AddAll(connect)
	all.AddAll(checkHealth)

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
		tasks, err := refresh(ctx, w.state, sto, req.w, req.file)
		if err != nil {
			return nil, fmt.Errorf("cannot refresh %q: %w", req.w.Name, err)
		}
		if len(tasks.Tasks()) == 0 {
			continue
		}
		taskset = append(taskset, tasks)
	}

	for _, ts := range taskset {
		cleanup := ts.MaybeEdge(EdgeRefreshCleanup)
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
				last, err := otherts.Edge(EdgeLastTaskBeforeRefreshIrreversible)
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

	usr, err := workshop.LookupUsername(username)
	if err != nil {
		return false, err
	}

	rootDir, err := workshop.UserDataRootDir(usr)
	if err != nil {
		return false, err
	}

	sketchdir := workshop.SketchSdkCurrent(rootDir, pid, wp)

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

func maybeRefresh(installed sdk.Setup, candidate *sdk.Info) bool {
	return installed.Revision != candidate.Revision
}

type refreshPlan struct {
	install []sdk.Setup
	refresh []sdk.Setup
	remove  []sdk.Setup

	installOrder  []string
	refreshAnyway bool
}

func (p refreshPlan) InstallOrRefresh() []sdk.Setup {
	ordered := []sdk.Setup{}

	for _, s := range p.installOrder {
		contains := func(sp sdk.Setup) bool { return s == sp.Name }

		idx := slices.IndexFunc(p.install, contains)
		if idx != -1 {
			ordered = append(ordered, p.install[idx])
			continue
		}

		idx = slices.IndexFunc(p.refresh, contains)
		if idx != -1 {
			ordered = append(ordered, p.refresh[idx])
			continue
		}
	}
	return ordered
}

func (p refreshPlan) Refresh() []sdk.Setup {
	return p.refresh
}

func (p refreshPlan) RefreshOrRemove() []sdk.Setup {
	ordered := []sdk.Setup{}

	for _, s := range p.installOrder {
		contains := func(sp sdk.Setup) bool { return s == sp.Name }

		idx := slices.IndexFunc(p.remove, contains)
		if idx != -1 {
			ordered = append(ordered, p.remove[idx])
			continue
		}

		idx = slices.IndexFunc(p.refresh, contains)
		if idx != -1 {
			ordered = append(ordered, p.refresh[idx])
			continue
		}
	}
	return ordered
}

func (p refreshPlan) RecreateWorkshop() bool {
	return p.refreshAnyway || len(p.install) != 0 || len(p.refresh) != 0 || len(p.remove) != 0
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

	// SDKs that only exists in the new workshop definition are to be installed.
	for _, info := range infos {
		if _, exist := w.Sdks[info.Name]; !exist {
			plan.install = append(plan.install, sdk.Setup{Name: info.Name, Channel: info.Channel, Revision: info.Revision})
		}
	}

	// SDKs that only exist in the previous workshop are to be removed.
	for _, rec := range w.Sdks {
		if !slices.ContainsFunc(infos, func(r sdk.SdkResult) bool {
			return r.Info.Name == rec.Name
		}) {
			plan.remove = append(plan.remove, rec)
		}
	}

	// SDKs that exist in both, the previous and new workshop definiotions, are to be refreshed.
	for _, info := range infos {
		// If the SDK is not in the list of the existing SDKs -- it is being
		// installed, not refreshed.
		if installed, exist := w.Sdks[info.Name]; exist {
			plan.refreshAnyway = maybeRefresh(installed, info.Info)
			if plan.refreshAnyway {
				break
			}
		}
	}

	sketchFound, err := maybeSketch(ctx, w.Project.ProjectId, w.Name)
	if err != nil {
		return nil, err
	}

	if w.Base != file.Base {
		plan.refreshAnyway = true
	}

	if !slices.Equal(w.File.Connections, file.Connections) {
		plan.refreshAnyway = true
	}

	cmpRecord := func(s1, s2 workshop.SdkRecord) bool {
		return reflect.DeepEqual(s1, s2)
	}
	if !slices.EqualFunc(w.File.Sdks, file.Sdks, cmpRecord) {
		plan.refreshAnyway = true
	}

	// Establish SDK installation order.
	for _, s := range file.Sdks {
		plan.installOrder = append(plan.installOrder, s.Name)
	}
	if sketchFound {
		plan.installOrder = append(plan.installOrder, sdk.Sketch)
	}

	// TODO: refresh only the required SDK when a workshop will be able to
	// restore from a snapshots. Now, if there is at least one SDK to be
	// refreshed, then refresh the entire set of SDKs.
	if plan.refreshAnyway || len(plan.install) > 0 || len(plan.remove) > 0 || sketchFound {
		for _, info := range infos {
			if _, exist := w.Sdks[info.Name]; exist {
				plan.refresh = append(plan.refresh, sdk.Setup{Name: info.Name, Channel: info.Channel, Revision: info.Revision})
			}
		}

		if sketchFound {
			if installed, exist := w.Sdks[sdk.Sketch]; exist {
				plan.refresh = append(plan.refresh, installed)
			} else {
				plan.install = append(plan.install, sdk.Setup{Name: sdk.Sketch, Revision: sdk.Revision{N: -1}})
			}
		}
	}

	return plan, nil
}

func refresh(ctx context.Context, st *state.State, sto sdk.Store, w *workshop.Workshop, file *workshop.File) (*state.TaskSet, error) {
	st.Unlock()
	plan, err := resolveRefresh(ctx, sto, w, file)
	st.Lock()
	if err != nil {
		return nil, err
	}

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

	retrieve := retrieveSdks(st, plan.InstallOrRefresh())
	addTaskSet(retrieve)

	if len(plan.Refresh()) > 0 {
		saveStateTs := state.NewTaskSet()
		stateStorage := st.NewTask("create-state-storage", "Create SDK state storage")
		stateStorage.WaitAll(retrieve)
		saveStateTs.AddTask(stateStorage)

		// Call save-state hooks for the SDKs that are installed and will not be
		// removed after this refresh.
		saveState := createStateHooks(st, file.Name, plan.Refresh(), hookstate.SaveState)
		saveState.WaitFor(stateStorage)
		saveStateTs.AddAll(saveState)

		addTaskSet(saveStateTs)
	}

	if plan.RecreateWorkshop() {
		disconn := []sdk.Setup{{Name: sdk.System.String(), Revision: sdk.Revision{N: -1}}}
		disconn = append(disconn, slices.Collect(maps.Values(w.Sdks))...)
		disconnect := disconnectSdks(st, disconn)
		addTaskSet(disconnect)
	} else {
		disconnect := disconnectSdks(st, plan.RefreshOrRemove())
		addTaskSet(disconnect)
	}

	if plan.RecreateWorkshop() {
		stash := st.NewTask("stash-workshop", fmt.Sprintf("Stash previous %q workshop", file.Name))
		addTaskSet(state.NewTaskSet(stash))

		launch := constructWorkshop(st, file, w.Project)
		addTaskSet(launch)

		system := sdkstate.InstallSystemSdk(st)
		addTaskSet(system)
	}

	install := installSdks(st, file.Name, plan.InstallOrRefresh(), retrieve)
	addTaskSet(install)

	if plan.RecreateWorkshop() {
		conn := []sdk.Setup{{Name: sdk.System.String(), Revision: sdk.Revision{N: -1}}}
		conn = append(conn, plan.InstallOrRefresh()...)
		connect := autoconnectSdks(st, conn)
		addTaskSet(connect)
	} else {
		connect := autoconnectSdks(st, plan.InstallOrRefresh())
		addTaskSet(connect)
	}

	if len(plan.Refresh()) > 0 {
		restoreState := createStateHooks(st, file.Name, plan.Refresh(), hookstate.RestoreState)
		addTaskSet(restoreState)
	}

	checkHealth := checkSdksHealth(st, file.Name, plan.InstallOrRefresh())
	addTaskSet(checkHealth)

	length := len(refresh.Tasks())
	if length == 0 {
		return refresh, nil
	}

	cleanupLane := st.NewLane()
	lastRefreshTsk := refresh.Tasks()[length-1]
	refresh.MarkEdge(lastRefreshTsk, EdgeLastTaskBeforeRefreshIrreversible)
	if len(plan.Refresh()) > 0 {
		removeStateStorage := st.NewTask("remove-state-storage", "Remove SDK state storage")
		removeStateStorage.WaitFor(lastRefreshTsk)
		removeStateStorage.JoinLane(cleanupLane)
		refresh.MarkEdge(removeStateStorage, EdgeRefreshCleanup)

		refresh.AddTask(removeStateStorage)
	}

	if plan.RecreateWorkshop() {
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
		removeStash.WaitFor(lastRefreshTsk)

		if refresh.MaybeEdge(EdgeRefreshCleanup) == nil {
			refresh.MarkEdge(removeStash, EdgeRefreshCleanup)
		}

		refresh.AddTask(removeStash)
	}

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

	ts, err := w.refreshLocalSdk(wp, sdkn)
	if err != nil {
		return nil, err
	}
	taskset = append(taskset, ts)
	return taskset, nil
}

func (w *WorkshopManager) refreshLocalSdk(wp *workshop.Workshop, sdkn string) (*state.TaskSet, error) {
	cur, installed := wp.Sdks[sdkn]
	var setup sdk.Setup
	if installed {
		setup = sdk.Setup{Name: sdkn, Revision: sdk.Revision{N: cur.Revision.N - 1}}
	} else {
		setup = sdk.Setup{Name: sdkn, Revision: sdk.Revision{N: -1}}
	}

	st := w.state
	ts := state.NewTaskSet()

	if installed {
		createStateStorage := st.NewTask("create-state-storage", "Create SDK state storage")
		ts.AddTask(createStateStorage)

		saveStateHook := hookstate.Hook(st, wp.Name, setup.Name, hookstate.SaveState)
		saveStateHook.WaitFor(createStateStorage)

		disconnect := st.NewTask("auto-disconnect", fmt.Sprintf(`Disconnect interfaces of %q SDK`, setup.Name))
		disconnect.Set("sdk", setup.Name)
		disconnect.WaitFor(saveStateHook)
		ts.AddTask(saveStateHook)
		ts.AddTask(disconnect)
	}

	install := sdkstate.InstallLocalSdk(st, setup)
	install.WaitAll(ts)
	ts.AddAll(install)

	setupHook := hookstate.Hook(st, wp.Name, setup.Name, hookstate.SetupBase)
	setupHook.WaitAll(install)
	ts.AddTask(setupHook)

	autoconnect := st.NewTask("auto-connect", fmt.Sprintf("Auto-connect interfaces of %q SDK", setup.Name))
	autoconnect.Set("sdk", setup.Name)
	autoconnect.WaitFor(setupHook)
	ts.AddTask(autoconnect)

	if installed {
		restoreState := hookstate.Hook(st, wp.Name, setup.Name, hookstate.RestoreState)
		restoreState.WaitFor(autoconnect)
		ts.AddTask(restoreState)
	}

	checkHealth := hookstate.Hook(st, wp.Name, setup.Name, hookstate.CheckHealth)
	checkHealth.WaitAll(ts)
	ts.AddTask(checkHealth)

	if installed {
		removeStateStorage := st.NewTask("remove-state-storage", "Remove SDK state storage")
		removeStateStorage.WaitFor(checkHealth)
		ts.AddTask(removeStateStorage)
	}

	for _, task := range ts.Tasks() {
		task.Set("workshop", wp.Name)
		task.Set("project", wp.Project)
	}

	return ts, nil
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

func createStateHooks(st *state.State, w string, installed []sdk.Setup, hooktype hookstate.WorkshopHookType) *state.TaskSet {
	stateHooks := state.NewTaskSet()
	prev := (*state.Task)(nil)
	for _, sk := range installed {
		if sk.Name == sdk.System.String() {
			continue
		}

		stateHook := hookstate.Hook(st, w, sk.Name, hooktype)
		stateHooks.AddTask(stateHook)
		if prev != nil {
			stateHook.WaitFor(prev)
		}
		prev = stateHook
	}
	return stateHooks
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

	disconn := []sdk.Setup{{Name: sdk.System.String(), Revision: sdk.Revision{N: -1}}}
	disconn = append(disconn, slices.Collect(maps.Values(w.Sdks))...)
	disconnectSet := disconnectSdks(st, disconn)

	discard := st.NewTask("discard-conns", fmt.Sprintf("Discard %q undesired connections", w.Name))
	discard.WaitAll(disconnectSet)

	remove := st.NewTask("remove-workshop", fmt.Sprintf("Remove %q workshop", w.Name))
	remove.WaitAll(disconnectSet)
	remove.WaitFor(discard)

	removeAptCache := st.NewTask("remove-apt-cache", fmt.Sprintf("Remove apt cache for %q", w.Name))
	removeAptCache.WaitFor(remove)

	// The apt cache cannot be removed until the workshop has stopped, which
	// currently happens as part of remove-workshop. Since there is no way to
	// undo remove-workshop, we run remove-apt-cache in a separate lane. If an
	// error occurs when removing the cache, it will not affect the other tasks.
	cleanupLane := st.NewLane()
	removeAptCache.JoinLane(cleanupLane)

	removeSet.AddAll(disconnectSet)
	removeSet.AddTask(discard)
	removeSet.AddTask(remove)
	removeSet.AddTask(removeAptCache)

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
