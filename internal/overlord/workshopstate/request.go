package workshopstate

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/sdkstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/statecontext"
	"github.com/canonical/workshop/internal/revert"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshopbackend"
	"golang.org/x/exp/slices"
)

const (
	LaneCleanupRefresh                = 1
	EdgeLastBeforeRefreshIrreversible = state.TaskSetEdge("last-before-irreversible")
	EdgeCleanupRefresh                = state.TaskSetEdge("refresh-cleanup")
)

func (w *WorkshopManager) loadProject(ctx context.Context, id string) (*workshopbackend.Project, error) {
	username, ok := ctx.Value(workshopbackend.ContextUser).(string)
	if !ok {
		return nil, fmt.Errorf("context key user not found")
	}

	projects, err := w.backend.Projects(ctx)
	if err != nil {
		return nil, err
	}

	idx := slices.IndexFunc(projects[username], func(p *workshopbackend.Project) bool { return p.ProjectId == id })
	if idx == -1 {
		return nil, fmt.Errorf("no project found with \"id\" %v", id)
	}
	return projects[username][idx], nil
}

func (w *WorkshopManager) LaunchMany(ctx context.Context, names []string, projectId string, opChangeId string) ([]*state.TaskSet, error) {
	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	taskset := make([]*state.TaskSet, 0, len(names))
	for _, i := range names {
		file, err := workshopbackend.ReadWorkshop(workshopbackend.WorkshopFilePath(project.Path, i))
		if err != nil {
			return nil, fmt.Errorf("cannot read %q file: %w", i, err)
		}

		wrkspace, err := w.Workshop(ctx, i, projectId)
		if wrkspace != nil {
			return nil, fmt.Errorf("cannot launch: %q already exists", i)
		}
		if !errors.Is(err, workshopbackend.ErrWorkshopNotFound) {
			return nil, err
		}

		tasks, err := launch(w.state, file, project)
		if err != nil {
			return nil, fmt.Errorf("cannot launch %q: %w", i, err)
		}

		for _, tsk := range tasks.Tasks() {
			if tsk.Kind() == "create-workshop" {
				tsk.Set("start-operation", true)
			}

			if len(file.Sdks) == 0 {
				if tsk.Kind() == "start-workshop" {
					tsk.Set("stop-operation", true)
				}
			} else {
				if tsk.Kind() == "auto-connect" && len(tsk.HaltTasks()) == 0 {
					tsk.Set("stop-operation", true)
				}
			}
		}
		taskset = append(taskset, tasks)
	}

	if err = w.startOperation(names, projectId, statecontext.Operation{ChangeId: opChangeId, Operation: statecontext.OperationLaunch}); err != nil {
		return nil, err
	}
	return taskset, nil
}

func launch(st *state.State, file *workshopbackend.WorkshopFile, project *workshopbackend.Project) (*state.TaskSet, error) {
	retrieve := state.NewTaskSet([]*state.Task{}...)
	install := state.NewTaskSet([]*state.Task{}...)
	setupHook := state.NewTaskSet([]*state.Task{}...)
	autoConnectSet := state.NewTaskSet([]*state.Task{}...)

	create := st.NewTask("create-workshop", fmt.Sprintf("Create new %q workshop", file.Name))
	create.Set("base", file.Base)

	mountProject := st.NewTask("mount-project", fmt.Sprintf("Mount project directory %q", project.Path))
	mountProject.WaitFor(create)

	start := st.NewTask("start-workshop", fmt.Sprintf("Start %q workshop", file.Name))
	start.WaitFor(mountProject)

	prevInstall := (*state.TaskSet)(nil)
	prevSetup := (*state.Task)(nil)
	prevAuto := (*state.Task)(nil)
	for _, sdk := range file.Sdks {
		r := sdkstate.Retrieve(st, &sdk)
		retrieve.AddTask(r)

		// The install task sets must not run concurrently as exec ops are not
		// allowed by LXD to be run concurrently and in general case we cannot
		// guarantee safety of concurrent installations
		installTaskSet := sdkstate.Install(st, sdk.Name, r.ID())
		if prevInstall != nil {
			installTaskSet.WaitAll(prevInstall)
		}
		prevInstall = installTaskSet
		install.AddAll(installTaskSet)

		// Make sure that the hook tasks are not concurrent
		setupHookTask := hookstate.Hook(st, file.Name, sdk.Name, hookstate.SetupBase)
		if prevSetup != nil {
			setupHookTask.WaitFor(prevSetup)
		}
		prevSetup = setupHookTask
		setupHook.AddTask(setupHookTask)

		autoconnect := st.NewTask("auto-connect", fmt.Sprintf("Auto-connect interfaces of %q SDK", sdk.Name))
		autoconnect.Set("sdk", sdk.Name)
		autoConnectSet.AddTask(autoconnect)
		if prevAuto != nil {
			autoconnect.WaitFor(prevAuto)
		}
		prevAuto = autoconnect
	}
	create.WaitAll(retrieve)
	install.WaitFor(start)
	setupHook.WaitAll(install)
	autoConnectSet.WaitAll(setupHook)

	set := state.NewTaskSet(create, mountProject, start)
	set.AddAll(retrieve)
	set.AddAll(install)
	set.AddAll(setupHook)
	set.AddAll(autoConnectSet)

	for _, i := range set.Tasks() {
		i.Set("workshop", file.Name)
		i.Set("project", project)
	}

	return set, nil
}

func (w *WorkshopManager) RefreshMany(ctx context.Context,
	names []string, projectId string, refreshMode statecontext.RefreshMode, opChangeId string) ([]*state.TaskSet, error) {
	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	invalid, status, err := w.CheckStatus(
		ctx,
		names,
		projectId,
		func(status workshopbackend.WorkshopStatus) bool {
			return status == workshopbackend.WorkshopReady
		})
	if err != nil {
		return nil, fmt.Errorf("cannot refresh: %w", err)
	}

	if len(invalid) > 0 {
		return nil, fmt.Errorf("cannot refresh: %q is in %s; must be ready", invalid, strings.ToLower(status.String()))
	}

	_, workshops, err := w.Workshops(ctx, projectId)
	if err != nil {
		return nil, err
	}

	files := make([]*workshopbackend.WorkshopFile, 0)
	content := make([][]sdk.Setup, 0)
	for _, i := range names {
		idx := slices.IndexFunc(workshops, func(w *workshopbackend.Workshop) bool { return w.Name == i })
		if idx == -1 {
			return nil, fmt.Errorf("%q workshop not found", i)
		}
		files = append(files, workshops[idx].File())
		content = append(content, workshops[idx].Content())
	}

	taskset, err := refreshMany(w.state, files, content, project)
	if err != nil {
		return nil, err
	}

	if err = w.startOperation(names, projectId, statecontext.Operation{
		ChangeId:    opChangeId,
		Operation:   statecontext.OperationRefresh,
		WaitOnError: refreshMode == statecontext.RefreshWaitOnError,
	}); err != nil {
		return nil, err
	}

	return taskset, nil
}

func refreshMany(st *state.State, w []*workshopbackend.WorkshopFile, content [][]sdk.Setup,
	project *workshopbackend.Project) ([]*state.TaskSet, error) {
	taskset := make([]*state.TaskSet, 0, len(w))

	for i, w := range w {
		tasks, err := refresh(st, w, content[i], project)
		if err != nil {
			return nil, fmt.Errorf("cannot refresh \"%s\" workshop: %w", w, err)
		}
		taskset = append(taskset, tasks)
	}

	for _, ts := range taskset {
		cleanup, err := ts.Edge(EdgeCleanupRefresh)
		if err != nil {
			return nil, err
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
				last, err := otherts.Edge(EdgeLastBeforeRefreshIrreversible)
				if err != nil {
					return nil, err
				}
				cleanup.WaitFor(last)
				// if the change was aborted during the cleanup stage execution,
				// there is a chance that some of the workshop copies that had
				// been created during the refresh were already deleted. If we
				// start to Undo those workshops' refresh progress we will
				// endup deleting the workshops that finished their refresh.
				// Given that they have no copy already, the undo logic
				// (stash-workshop) will delete the existing workshop
				// and fail to restore from the copy. We don't want that. Hence,
				// all the cleanup tasks are extracted into a separate lane. If
				// any problem happens, the workshops which had finished their
				// refresh will not suffer.
				cleanup.JoinLane(LaneCleanupRefresh)
				for _, t := range cleanup.HaltTasks() {
					t.JoinLane(LaneCleanupRefresh)
				}
			}
		}
	}

	return taskset, nil
}

func refresh(st *state.State, file *workshopbackend.WorkshopFile, content []sdk.Setup, p *workshopbackend.Project) (*state.TaskSet, error) {
	// 1. Save previous state
	// 2. Stop previous workshop
	// 3. Put to stash
	// 4. Launch the new workshop
	// 5. Run restore state
	// 6. Delete the old workshop

	createStateStorage := st.NewTask("create-state-storage", "Create SDK state storage")
	saveStateHooks := saveStateHooks(st, file.Name, content, file.Sdks)
	saveStateHooks.WaitFor(createStateStorage)

	// disconnect and remove SDKs plugs and slots
	disconnect := disconnectSdks(content, st)
	disconnect.WaitAll(saveStateHooks)

	putToStash := st.NewTask("stash-workshop", fmt.Sprintf("Stash previous %q workshop", file.Name))
	putToStash.WaitAll(disconnect)
	putToStash.WaitAll(saveStateHooks)
	putToStash.WaitFor(createStateStorage)

	launch, err := launch(st, file, p)
	if err != nil {
		return nil, err
	}

	restoreStateHooks := restoreStateHooks(st, file.Name, content, file.Sdks)

	removeStateStorage := st.NewTask("remove-state-storage", "Remove SDK state storage")
	removeFromStash := st.NewTask("remove-workshop-stash", fmt.Sprintf("Remove %q workshop from stash", file.Name))

	// save-state -> stop-workshop -> launch -> restore state
	removeFromStash.WaitFor(removeStateStorage)
	if len(restoreStateHooks.Tasks()) > 0 {
		removeStateStorage.WaitAll(restoreStateHooks)
		restoreStateHooks.WaitAll(launch)
	} else {
		lastLaunchTask := launch.Tasks()[len(launch.Tasks())-1]
		removeStateStorage.WaitFor(lastLaunchTask)
		// we are dealing with a workshop that does not have
		// SDKs, i.e. it will not be running any hooks. Thus,
		// the point of no return is the last launch task (i.e.
		// before the moment when we delete the copy of the workshop
		// after the refresh operation).
		launch.MarkEdge(lastLaunchTask, EdgeLastBeforeRefreshIrreversible)
	}

	launch.WaitFor(putToStash)

	refresh := state.NewTaskSet([]*state.Task{}...)
	refresh.AddTask(createStateStorage)
	refresh.AddAll(saveStateHooks)
	refresh.AddAllWithEdges(launch)
	refresh.AddAllWithEdges(restoreStateHooks)
	refresh.AddTask(putToStash)
	refresh.AddAll(disconnect)
	refresh.AddTask(removeStateStorage)
	refresh.AddTask(removeFromStash)

	// mark the first task to start the operation so if cancelled or failed the
	// operation will be removed from the list of the active ones
	createStateStorage.Set("start-operation", true)

	// mark the last task to stop the operation
	// and make the workshop available for other commands
	removeFromStash.Set("stop-operation", true)

	refresh.MarkEdge(removeStateStorage, EdgeCleanupRefresh)

	for _, i := range refresh.Tasks() {
		i.Set("workshop", file.Name)
		i.Set("project", *p)
	}

	return refresh, nil
}

func disconnectSdks(content []sdk.Setup, st *state.State) *state.TaskSet {
	disconnectSet := []*state.Task{}
	prev := (*state.Task)(nil)
	for _, s := range content {
		disc := st.NewTask("disconnect", fmt.Sprintf("Disconnect interfaces of %q SDK", s.Name))
		disc.Set("sdk", s.Name)
		if prev != nil {
			disc.WaitFor(prev)
		}
		disconnectSet = append(disconnectSet, disc)
		prev = disc
	}
	return state.NewTaskSet(disconnectSet...)
}

func saveStateHooks(st *state.State, workshop string, content []sdk.Setup, newContent workshopbackend.SdkList,
) *state.TaskSet {
	return createStateHooks(st, workshop, content, newContent, hookstate.SaveState)
}

func restoreStateHooks(st *state.State, workshop string, content []sdk.Setup, newContent workshopbackend.SdkList) *state.TaskSet {
	stateHooks := createStateHooks(st, workshop, content, newContent, hookstate.RestoreState)

	// if the restore hooks are not present (i.e. workshop has no SDKs after
	// the refresh), we should mark the last launch task as last before the
	// irreversible change happens. This will be done in the refreshMany
	// call.
	if len(stateHooks.Tasks()) > 0 {
		last := stateHooks.Tasks()[len(stateHooks.Tasks())-1]
		// last restore state hook for the refresh call is the last task before the
		// previous refresh copy removal, ie. before making something irreversible
		stateHooks.MarkEdge(last, EdgeLastBeforeRefreshIrreversible)
	}
	return stateHooks
}

func createStateHooks(st *state.State, workshop string, content []sdk.Setup, newContent workshopbackend.SdkList, hooktype hookstate.WorkshopHookType) *state.TaskSet {
	stateHooks := state.NewTaskSet([]*state.Task{}...)
	prevRestore := (*state.Task)(nil)
	for _, newsdk := range newContent {
		// the state hooks will only be set for the SDKs that were installed AND
		// were not removed from the workshop file at the time of refresh
		if slices.IndexFunc(content, func(s sdk.Setup) bool { return s.Name == newsdk.Name }) == -1 {
			continue
		}

		stateHook := hookstate.Hook(st, workshop, newsdk.Name, hooktype)
		stateHooks.AddTask(stateHook)
		if prevRestore != nil {
			stateHook.WaitFor(prevRestore)
		}
		prevRestore = stateHook
	}
	return stateHooks
}

func (w *WorkshopManager) StartMany(ctx context.Context, names []string, projectId string, opChangeId string) (*state.TaskSet, error) {
	// check if all the workshops are stopped
	invalid, status, err := w.CheckStatus(
		ctx,
		names,
		projectId,
		func(status workshopbackend.WorkshopStatus) bool {
			return status == workshopbackend.WorkshopStopped
		})
	if err != nil {
		return nil, fmt.Errorf("cannot start: %w", err)
	}

	if len(invalid) > 0 {
		return nil, fmt.Errorf("cannot start: %q is in %s; must be stopped", invalid, strings.ToLower(status.String()))
	}

	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}
	taskset, err := startMany(w.state, names, project)
	if err != nil {
		return nil, err
	}

	if err = w.startOperation(names, projectId, statecontext.Operation{ChangeId: opChangeId, Operation: statecontext.OperationStart}); err != nil {
		return nil, err
	}
	return taskset, nil
}

func startMany(st *state.State, names []string, project *workshopbackend.Project) (*state.TaskSet, error) {
	taskset := state.NewTaskSet([]*state.Task{}...)

	for _, name := range names {
		start := st.NewTask("start-workshop", fmt.Sprintf("Start %q workshop", name))
		// start is a single task, so it is the beginning and the end of the operation
		start.Set("start-operation", true)
		start.Set("stop-operation", true)
		taskset.AddTask(start)

		start.Set("workshop", name)
		start.Set("project", *project)
	}

	return taskset, nil
}

func (w *WorkshopManager) StopMany(ctx context.Context, names []string, projectId string, opChangeId string) (*state.TaskSet, error) {
	invalid, status, err := w.CheckStatus(
		ctx,
		names,
		projectId,
		func(status workshopbackend.WorkshopStatus) bool {
			return status == workshopbackend.WorkshopStopped || status == workshopbackend.WorkshopReady
		})
	if err != nil {
		return nil, fmt.Errorf("cannot stop: %w", err)
	}

	if len(invalid) > 0 {
		return nil, fmt.Errorf("cannot stop: %q is in %s; must be stopped or ready", invalid, strings.ToLower(status.String()))
	}

	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}
	taskset, err := stopMany(w.state, names, project)
	if err != nil {
		return nil, err
	}

	if err = w.startOperation(names, projectId, statecontext.Operation{ChangeId: opChangeId, Operation: statecontext.OperationStop}); err != nil {
		return nil, err
	}
	return taskset, nil
}

func stopMany(st *state.State, names []string, project *workshopbackend.Project) (*state.TaskSet, error) {
	taskset := state.NewTaskSet([]*state.Task{}...)

	for _, name := range names {
		stop := st.NewTask("stop-workshop", fmt.Sprintf("Stop %q workshop", name))
		// start is a single task, so it is the beginning and the end of the operation
		stop.Set("start-operation", true)
		stop.Set("stop-operation", true)
		stop.Set("force", false)
		taskset.AddTask(stop)

		stop.Set("workshop", name)
		stop.Set("project", *project)
	}

	return taskset, nil
}

type ExecMeta struct {
	Environment map[string]string
	WorkingDir  string
}

func (w *WorkshopManager) Exec(ctx context.Context, name, projectId string, args *workshopbackend.ExecArgs) (*state.Task, error) {
	invalid, status, err := w.CheckStatus(
		ctx,
		[]string{name},
		projectId,
		func(status workshopbackend.WorkshopStatus) bool {
			return status == workshopbackend.WorkshopReady || status == workshopbackend.WorkshopPending
		})
	if err != nil {
		return nil, fmt.Errorf("cannot exec: %w", err)
	}

	if len(invalid) > 0 {
		return nil, fmt.Errorf("cannot exec: %q is in %s; must be ready or pending", invalid, strings.ToLower(status.String()))
	}

	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	ctx = context.WithValue(ctx, workshopbackend.ContextProjectId, project.ProjectId)
	wrkspc, err := w.backend.WorkshopFs(ctx, name)
	if err != nil {
		return nil, err
	}
	defer wrkspc.Close()

	info, err := wrkspc.Stat(args.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("%s does not exist", args.WorkDir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", args.WorkDir)
	}

	exec := w.state.NewTask("exec", fmt.Sprintf("Exec command %q", args.Command[0]))

	exec.Set("exec-setup", args)
	exec.Set("project", project)
	exec.Set("workshop", name)

	return exec, nil
}

func (w *WorkshopManager) RemoveMany(ctx context.Context, names []string, projectId string, opChangeId string) (*state.TaskSet, error) {
	invalid, status, err := w.CheckStatus(
		ctx,
		names,
		projectId,
		func(status workshopbackend.WorkshopStatus) bool {
			return status != workshopbackend.WorkshopPending && status != workshopbackend.WorkshopOff
		})
	if err != nil {
		return nil, fmt.Errorf("cannot remove: %w", err)
	}

	if len(invalid) > 0 {
		return nil, fmt.Errorf("cannot remove: %q is in %s; must be ready, stopped or error", invalid, strings.ToLower(status.String()))
	}

	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	ctx = context.WithValue(ctx, workshopbackend.ContextProjectId, project.ProjectId)

	var workshops = make([]*workshopbackend.Workshop, 0, len(names))
	for _, name := range names {
		if workshop, err := w.backend.Workshop(ctx, name); err != nil {
			return nil, err
		} else {
			workshops = append(workshops, workshop)
		}
	}

	taskset, err := removeMany(w.state, workshops, project)
	if err != nil {
		return nil, err
	}

	if err := w.startOperation(names, projectId, statecontext.Operation{ChangeId: opChangeId, Operation: statecontext.OperationRemove}); err != nil {
		return nil, err
	}

	return taskset, nil
}

func (w *WorkshopManager) startOperation(names []string, projectId string, op statecontext.Operation) error {
	rev := revert.New()
	for _, name := range names {
		err := statecontext.StartOperation(w.state, name, projectId, op)
		if err != nil {
			return err
		}
		name := name // go loop var capturing issue
		rev.Add(func() {
			statecontext.StopOperation(w.state, name, projectId, op.Operation)
		})
	}
	rev.Success()
	return nil
}

func removeMany(st *state.State, workshops []*workshopbackend.Workshop, project *workshopbackend.Project) (*state.TaskSet, error) {
	taskset := state.NewTaskSet([]*state.Task{}...)
	for _, name := range workshops {
		remove, err := remove(st, name, project)
		if err != nil {
			return nil, err
		}
		taskset.AddAll(remove)
	}
	return taskset, nil
}

func remove(st *state.State, workshop *workshopbackend.Workshop, project *workshopbackend.Project) (*state.TaskSet, error) {
	removeSet := state.NewTaskSet()
	disconnectSet := disconnectSdks(workshop.Content(), st)
	remove := st.NewTask("remove-workshop", fmt.Sprintf("Remove %q workshop", workshop.Name))
	if len(disconnectSet.Tasks()) > 0 {
		disconnectSet.Tasks()[0].Set("start-operation", true)
	} else {
		remove.Set("start-operation", true)
	}
	remove.Set("stop-operation", true)
	remove.WaitAll(disconnectSet)
	removeSet.AddAll(disconnectSet)
	removeSet.AddTask(remove)

	for _, i := range removeSet.Tasks() {
		i.Set("workshop", workshop.Name)
		i.Set("project", project)
	}
	return removeSet, nil
}
