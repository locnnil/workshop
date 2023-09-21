package workspacestate

import (
	"context"
	"fmt"
	"strings"

	"github.com/canonical/workspace/internal/overlord/hookstate"
	"github.com/canonical/workspace/internal/overlord/sdkstate"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/overlord/statecontext"
	"github.com/canonical/workspace/internal/sdk"
	"github.com/canonical/workspace/internal/workspacebackend"
	"golang.org/x/exp/slices"
)

const (
	LaneCleanupRefresh                = 1
	EdgeLastBeforeRefreshIrreversible = state.TaskSetEdge("last-before-irreversible")
	EdgeCleanupRefresh                = state.TaskSetEdge("refresh-cleanup")
)

func (w *WorkspaceManager) loadProject(ctx context.Context, id string) (*workspacebackend.Project, error) {
	projects, err := w.backend.Projects(ctx)
	if err != nil {
		return nil, fmt.Errorf("no project found with \"id\" %v", id)
	}

	prj, ok := projects[id]
	if !ok {
		return nil, fmt.Errorf("no project found with \"id\" %v", id)
	}
	return prj, nil
}

func (w *WorkspaceManager) LaunchMany(ctx context.Context, names []string, projectId string, opChangeId string) ([]*state.TaskSet, error) {
	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	taskset := make([]*state.TaskSet, 0, len(names))
	for _, i := range names {
		file, err := workspacebackend.ReadWorkspace(workspacebackend.WorkspaceFilePath(project.Path, i))
		if err != nil {
			return nil, fmt.Errorf("cannot read %q file: %w", i, err)
		}

		wrkspace, err := w.Workspace(ctx, i, projectId)
		if wrkspace != nil {
			return nil, fmt.Errorf("cannot launch: %q already exists", i)
		}
		if !errors.Is(err, workspacebackend.ErrWorkspaceNotFound) {
			return nil, err
		}

		tasks, err := launch(w.state, file, project)
		if err != nil {
			return nil, fmt.Errorf("cannot launch %q: %w", i, err)
		}

		for _, tsk := range tasks.Tasks() {
			if tsk.Kind() == "create-workspace" {
				tsk.Set("start-operation", true)
			}

			if len(file.Sdks) == 0 {
				if tsk.Kind() == "start-workspace" {
					tsk.Set("stop-operation", true)
				}
			} else {
				if tsk.Kind() == "run-hook" && len(tsk.HaltTasks()) == 0 {
					tsk.Set("stop-operation", true)
				}
			}
		}
		taskset = append(taskset, tasks)
	}

	for _, name := range names {
		err = statecontext.StartOperation(w.state, name, projectId,
			statecontext.Operation{
				ChangeId:  opChangeId,
				Operation: statecontext.OperationLaunch,
			})
		if err != nil {
			return nil, err
		}
	}
	return taskset, nil
}

func launch(st *state.State, file *workspacebackend.WorkspaceFile, project *workspacebackend.Project) (*state.TaskSet, error) {
	retrieve := state.NewTaskSet([]*state.Task{}...)
	install := state.NewTaskSet([]*state.Task{}...)
	setupHook := state.NewTaskSet([]*state.Task{}...)

	create := st.NewTask("create-workspace", fmt.Sprintf("Create new %q workspace", file.Name))
	create.Set("base", file.Base)
	create.WaitAll(retrieve)

	mountProject := st.NewTask("mount-project", fmt.Sprintf("Mount project directory %q", project.Path))
	mountProject.WaitFor(create)

	start := st.NewTask("start-workspace", fmt.Sprintf("Start %q workspace", file.Name))
	start.WaitFor(mountProject)

	prevInstall := (*state.TaskSet)(nil)
	prevSetup := (*state.Task)(nil)
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
		setupHookTask := hookstate.SetupHook(st, &sdk, hookstate.SetupBase)
		if prevSetup != nil {
			setupHookTask.WaitFor(prevSetup)
		}
		prevSetup = setupHookTask

		setupHook.AddTask(setupHookTask)
	}

	install.WaitFor(start)
	setupHook.WaitAll(install)

	set := state.NewTaskSet(create, mountProject, start)
	set.AddAll(retrieve)
	set.AddAll(install)
	set.AddAll(setupHook)

	for _, i := range set.Tasks() {
		i.Set("workspace", file.Name)
		i.Set("project", project)
	}

	return set, nil
}

func (w *WorkspaceManager) RefreshMany(ctx context.Context,
	names []string, projectId string, refreshMode statecontext.RefreshMode, opChangeId string) ([]*state.TaskSet, error) {
	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	invalid, status, err := w.CheckStatus(
		ctx,
		names,
		projectId,
		func(status workspacebackend.WorkspaceState) bool {
			return status == workspacebackend.WorkspaceReady
		})
	if err != nil {
		return nil, fmt.Errorf("cannot refresh: %w", err)
	}

	if len(invalid) > 0 {
		return nil, fmt.Errorf("cannot refresh: %q is in %s; must be ready", invalid, strings.ToLower(status.String()))
	}

	_, workspaces, err := w.Workspaces(ctx, projectId)
	if err != nil {
		return nil, err
	}

	files := make([]*workspacebackend.WorkspaceFile, 0)
	content := make([][]*sdk.SdkInfo, 0)
	for _, i := range names {
		idx := slices.IndexFunc(workspaces, func(w *workspacebackend.Workspace) bool { return w.Name == i })
		if idx == -1 {
			return nil, fmt.Errorf("%q workspace not found", i)
		}
		files = append(files, workspaces[idx].File())
		content = append(content, workspaces[idx].Content())
	}

	taskset, err := refreshMany(w.state, files, content, project)
	if err != nil {
		return nil, err
	}

	for _, name := range names {
		err = statecontext.StartOperation(w.state, name, projectId,
			statecontext.Operation{
				ChangeId:    opChangeId,
				Operation:   statecontext.OperationRefresh,
				WaitOnError: refreshMode == statecontext.RefreshWaitOnError,
			})
		if err != nil {
			return nil, err
		}
	}
	return taskset, nil
}

func refreshMany(st *state.State, w []*workspacebackend.WorkspaceFile, content [][]*sdk.SdkInfo,
	project *workspacebackend.Project) ([]*state.TaskSet, error) {
	taskset := make([]*state.TaskSet, 0, len(w))

	for i, w := range w {
		tasks, err := refresh(st, w, content[i], project)
		if err != nil {
			return nil, fmt.Errorf("cannot refresh \"%s\" workspace: %w", w, err)
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
		// finished. This will ensure that we start to remove the workspaces'
		// previous copies once all the refresh operations were successful (at
		// this stage, we only need to remove a copy, the newly refreshed
		// workspace is already up and running). Thus, every CleanupEdge will
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
				// there is a chance that some of the workspace copies that had
				// been created during the refresh were already deleted. If we
				// start to Undo those workspaces' refresh progress we will
				// endup deleting the workspaces that finished their refresh.
				// Given that they have no copy already, the undo logic
				// (undoMakeWorkspaceCopy) will delete the existing workspace
				// and fail to restore from the copy. We don't want that. Hence,
				// all the cleanup tasks are extracted into a separate lane. If
				// any problem happens, the workspaces which had finished their
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

func refresh(st *state.State, file *workspacebackend.WorkspaceFile, content []*sdk.SdkInfo, p *workspacebackend.Project) (*state.TaskSet, error) {
	// 1. Save previous state
	// 2. Stop previous workspace
	// 3. Put to stash
	// 4. Launch the new workspace
	// 5. Run restore state
	// 6. Delete the old workspace

	createStateStorage := st.NewTask("create-state-storage", "Create SDK state storage")
	saveStateHooks := saveStateHooks(st, content, file.Sdks)

	putToStash := st.NewTask("stash-workspace", fmt.Sprintf("Stash previous %q workspace", file.Name))

	launch, err := launch(st, file, p)
	if err != nil {
		return nil, err
	}

	restoreStateHooks := restoreStateHooks(st, content, file.Sdks)

	removeStateStorage := st.NewTask("remove-state-storage", "Remove SDK state storage")
	removeFromStash := st.NewTask("remove-workspace-stash", fmt.Sprintf("Remove %q workspace from stash", file.Name))

	// save-state -> stop-workspace -> launch -> restore state
	removeFromStash.WaitFor(removeStateStorage)
	if len(restoreStateHooks.Tasks()) > 0 {
		removeStateStorage.WaitAll(restoreStateHooks)
		restoreStateHooks.WaitAll(launch)
	} else {
		lastLaunchTask := launch.Tasks()[len(launch.Tasks())-1]
		removeStateStorage.WaitFor(lastLaunchTask)
		// we are dealing with a workspace that does not have
		// SDKs, i.e. it will not be running any hooks. Thus,
		// the point of no return is the last launch task (i.e.
		// before the moment when we delete the copy of the workspace
		// after the refresh operation)
		launch.MarkEdge(lastLaunchTask, EdgeLastBeforeRefreshIrreversible)
	}
	launch.WaitFor(putToStash)
	if len(saveStateHooks.Tasks()) > 0 {
		putToStash.WaitAll(saveStateHooks)
		saveStateHooks.WaitFor(createStateStorage)
	} else {
		putToStash.WaitFor(createStateStorage)
	}

	refresh := state.NewTaskSet([]*state.Task{}...)
	refresh.AddTask(createStateStorage)
	refresh.AddAll(saveStateHooks)
	refresh.AddAllWithEdges(launch)
	refresh.AddAllWithEdges(restoreStateHooks)
	refresh.AddTask(putToStash)
	refresh.AddTask(removeStateStorage)
	refresh.AddTask(removeFromStash)

	// mark the first task to start the operation so if cancelled or failed the
	// operation will be removed from the list of the active ones
	createStateStorage.Set("start-operation", true)

	// mark the last task to stop the operation
	// and make the workspace available for other commands
	removeFromStash.Set("stop-operation", true)

	refresh.MarkEdge(removeStateStorage, EdgeCleanupRefresh)

	for _, i := range refresh.Tasks() {
		i.Set("workspace", file.Name)
		i.Set("project", *p)
	}

	return refresh, nil
}

func saveStateHooks(st *state.State, content []*sdk.SdkInfo, newContent workspacebackend.SdkList,
) *state.TaskSet {
	return createStateHooks(st, content, newContent, hookstate.SaveState)
}

func restoreStateHooks(st *state.State, content []*sdk.SdkInfo, newContent workspacebackend.SdkList) *state.TaskSet {
	stateHooks := createStateHooks(st, content, newContent, hookstate.RestoreState)

	// if the restore hooks are not present (i.e. workspace has no SDKs after
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

func createStateHooks(st *state.State, content []*sdk.SdkInfo, newContent workspacebackend.SdkList, hooktype hookstate.WorkspaceHookType) *state.TaskSet {
	stateHooks := state.NewTaskSet([]*state.Task{}...)
	prevRestore := (*state.Task)(nil)
	for _, newsdk := range newContent {
		// the state hooks will only be set for the SDKs that were installed AND
		// were not removed from the workspace file at the time of refresh
		if slices.IndexFunc(content, func(s *sdk.SdkInfo) bool { return s.Name == newsdk.Name }) == -1 {
			continue
		}

		stateHook := hookstate.SetupHook(st, &newsdk, hooktype)
		stateHooks.AddTask(stateHook)
		if prevRestore != nil {
			stateHook.WaitFor(prevRestore)
		}
		prevRestore = stateHook
	}
	return stateHooks
}

func (w *WorkspaceManager) StartMany(ctx context.Context, names []string, projectId string, opChangeId string) (*state.TaskSet, error) {
	// check if all the workspaces are stopped
	invalid, status, err := w.CheckStatus(
		ctx,
		names,
		projectId,
		func(status workspacebackend.WorkspaceState) bool {
			return status == workspacebackend.WorkspaceStopped
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

	for _, name := range names {
		err = statecontext.StartOperation(w.state, name, projectId,
			statecontext.Operation{
				ChangeId:  opChangeId,
				Operation: statecontext.OperationStart,
			})
		if err != nil {
			return nil, err
		}
	}
	return taskset, nil
}

func startMany(st *state.State, names []string, project *workspacebackend.Project) (*state.TaskSet, error) {
	taskset := state.NewTaskSet([]*state.Task{}...)

	for _, name := range names {
		start := st.NewTask("start-workspace", fmt.Sprintf("Start %q workspace", name))
		// start is a single task, so it is the beginning and the end of the operation
		start.Set("start-operation", true)
		start.Set("stop-operation", true)
		taskset.AddTask(start)

		start.Set("workspace", name)
		start.Set("project", *project)
	}

	return taskset, nil
}

func (w *WorkspaceManager) StopMany(ctx context.Context, names []string, projectId string, opChangeId string) (*state.TaskSet, error) {
	invalid, status, err := w.CheckStatus(
		ctx,
		names,
		projectId,
		func(status workspacebackend.WorkspaceState) bool {
			return status == workspacebackend.WorkspaceStopped || status == workspacebackend.WorkspaceReady
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

	for _, name := range names {
		err = statecontext.StartOperation(w.state, name, projectId,
			statecontext.Operation{
				ChangeId:  opChangeId,
				Operation: statecontext.OperationStop,
			})
		if err != nil {
			return nil, err
		}
	}
	return taskset, nil
}

func stopMany(st *state.State, names []string, project *workspacebackend.Project) (*state.TaskSet, error) {
	taskset := state.NewTaskSet([]*state.Task{}...)

	for _, name := range names {
		stop := st.NewTask("stop-workspace", fmt.Sprintf("Stop %q workspace", name))
		// start is a single task, so it is the beginning and the end of the operation
		stop.Set("start-operation", true)
		stop.Set("stop-operation", true)
		stop.Set("force", false)
		taskset.AddTask(stop)

		stop.Set("workspace", name)
		stop.Set("project", *project)
	}

	return taskset, nil
}

type ExecMeta struct {
	Environment map[string]string
	WorkingDir  string
}

func (w *WorkspaceManager) Exec(ctx context.Context, name, projectId string, args *workspacebackend.ExecArgs) (*state.Task, ExecMeta, error) {
	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, ExecMeta{}, err
	}
	execArgs := *args

	// check if all the workspaces are stopped
	avail, _, err := w.CheckStatus(ctx, []string{name}, projectId,
		func(status workspacebackend.WorkspaceState) bool {
			return status == workspacebackend.WorkspaceReady || status == workspacebackend.WorkspacePending
		})
	if err != nil {
		return nil, ExecMeta{}, err
	}
	if !avail {
		return nil, ExecMeta{}, fmt.Errorf("%q is not in a ready or pending status", name)
	}

	ctx = context.WithValue(ctx, workspacebackend.ContextProjectId, project.ProjectId)
	wrkspc, err := w.backend.GetWorkspaceFs(ctx, name)
	if err != nil {
		return nil, ExecMeta{}, err
	}

	defer wrkspc.Close()

	info, err := wrkspc.Stat(args.WorkDir)
	if err != nil {
		return nil, ExecMeta{}, fmt.Errorf("%s does not exist", args.WorkDir)
	}
	if !info.IsDir() {
		return nil, ExecMeta{}, fmt.Errorf("%s is not a directory", args.WorkDir)
	}

	exec := w.state.NewTask("exec", fmt.Sprintf("exec command %q", args.Command[0]))

	exec.Set("exec-setup", &execArgs)
	exec.Set("project", *project)
	exec.Set("workspace", name)

	w.execChannelsLock.Lock()
	defer w.execChannelsLock.Unlock()
	w.execChannels[exec.ID()] = make(chan bool)

	return exec, ExecMeta{
		WorkingDir:  execArgs.WorkDir,
		Environment: execArgs.Environment,
	}, nil
}

func (w *WorkspaceManager) RemoveMany(ctx context.Context, names []string, projectId string, opChangeId string) (*state.TaskSet, error) {
	invalid, status, err := w.CheckStatus(
		ctx,
		names,
		projectId,
		func(status workspacebackend.WorkspaceState) bool {
			return status != workspacebackend.WorkspacePending && status != workspacebackend.WorkspaceOff
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

	taskset, err := removeMany(w.state, names, project)
	if err != nil {
		return nil, err
	}

	for _, name := range names {
		err = statecontext.StartOperation(w.state, name, projectId,
			statecontext.Operation{
				ChangeId:  opChangeId,
				Operation: statecontext.OperationRemove,
			})
		if err != nil {
			// TODO: stop operation for the workspaces that
			// had them started successfully
			return nil, err
		}
	}
	return taskset, nil
}

func removeMany(st *state.State, names []string, project *workspacebackend.Project) (*state.TaskSet, error) {
	taskset := state.NewTaskSet([]*state.Task{}...)

	for _, name := range names {
		remove := st.NewTask("remove-workspace", fmt.Sprintf("Remove %q workspace", name))
		// remove is a single task, so it is the beginning and the end of the operation
		remove.Set("start-operation", true)
		remove.Set("stop-operation", true)
		taskset.AddTask(remove)

		remove.Set("workspace", name)
		remove.Set("project", *project)
	}

	return taskset, nil
}
