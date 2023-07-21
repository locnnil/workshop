package workspacestate

import (
	"context"
	"fmt"
	"strings"

	"github.com/canonical/workspace/internal/overlord/hookstate"
	"github.com/canonical/workspace/internal/overlord/sdkstate"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/overlord/statecontext"
	"github.com/canonical/workspace/internal/workspacebackend"
	"golang.org/x/exp/slices"
)

const (
	RefreshIncumbentPrefix            = "refresh-incumbent-"
	RefereshDeleteCopyLane            = 1
	LastBeforeRefreshIrreversibleEdge = state.TaskSetEdge("last-before-irreversible")
	CleanupRefreshEdge                = state.TaskSetEdge("cleanup-refresh")
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

func (w *WorkspaceManager) LaunchMany(ctx context.Context, workspaces []string, projectId string) ([]*state.TaskSet, error) {
	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	taskset := make([]*state.TaskSet, 0, len(workspaces))
	for _, i := range workspaces {
		file, err := workspacebackend.ReadWorkspace(workspacebackend.WorkspaceFilePath(project.Path, i))
		if err != nil {
			return nil, fmt.Errorf("cannot read workspace \"%s\": %v", i, err)
		}

		tasks, err := launch(w.state, file, project)
		if err != nil {
			return nil, fmt.Errorf("cannot launch workspace \"%s\": %v", i, err)
		}
		taskset = append(taskset, tasks)
	}
	return taskset, nil
}

func launch(st *state.State, file *workspacebackend.WorkspaceFile, project *workspacebackend.Project) (*state.TaskSet, error) {
	retrieve := state.NewTaskSet([]*state.Task{}...)
	install := state.NewTaskSet([]*state.Task{}...)
	setupHook := state.NewTaskSet([]*state.Task{}...)

	create := st.NewTask("create-workspace", fmt.Sprintf("Create %q workspace", file.Name))
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
		setupHookTask := hookstate.SetupHook(st, file.Name, project.ProjectId, &sdk, hookstate.SetupBase)
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
	names []string, projectId string) ([]*state.TaskSet, error) {
	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	var inProgress = []string{}
	for _, r := range names {
		if _, prg := statecontext.RefreshInProgress(w.state, r, project.ProjectId); prg {
			inProgress = append(inProgress, r)
		}
	}
	if len(inProgress) > 0 {
		return nil, fmt.Errorf("refresh operation is already in progress for: %s", strings.Join(inProgress, ","))
	}

	// we are only interested in the existing (launched) workspaces
	_, workspaces, err := w.Workspaces(ctx, projectId)
	if err != nil {
		return nil, err
	}

	wss := make([]*workspacebackend.WorkspaceFile, 0)
	for _, i := range names {
		idx := slices.IndexFunc(workspaces, func(w *workspacebackend.Workspace) bool { return w.Name == i })
		if idx == -1 {
			return nil, fmt.Errorf("workspace %s not found", i)
		}
		wss = append(wss, workspaces[idx].File())
	}
	return refreshMany(w.state, wss, project)
}

func refreshMany(st *state.State, w []*workspacebackend.WorkspaceFile,
	project *workspacebackend.Project) ([]*state.TaskSet, error) {
	taskset := make([]*state.TaskSet, 0, len(w))

	for _, w := range w {
		tasks, err := refresh(st, w, project)
		if err != nil {
			return nil, fmt.Errorf("cannot refresh workspace \"%s\": %v", w, err)
		}
		taskset = append(taskset, tasks)
	}

	for _, ts := range taskset {
		cleanup, err := ts.Edge(CleanupRefreshEdge)
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
				last, err := otherts.Edge(LastBeforeRefreshIrreversibleEdge)
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
				cleanup.JoinLane(RefereshDeleteCopyLane)
			}
		}
	}

	return taskset, nil
}

func refresh(st *state.State, w *workspacebackend.WorkspaceFile, p *workspacebackend.Project) (*state.TaskSet, error) {
	// 1. Save previous state
	// 2. Stop previous workspace
	// 3. Make unavailable
	// 4. Launch the new workspace
	// 5. Run restore state
	// 6. Delete the old workspace

	createStateStorage := st.NewTask("create-state-storage", "Mount SDK state storage")
	saveStateHooks := state.NewTaskSet([]*state.Task{}...)
	prevSave := (*state.Task)(nil)
	for _, sdk := range w.Sdks {
		saveStateHook := hookstate.SetupHook(st, w.Name, p.ProjectId, &sdk, hookstate.SaveState)
		saveStateHooks.AddTask(saveStateHook)
		if prevSave != nil {
			saveStateHook.WaitFor(prevSave)
		}
		prevSave = saveStateHook
	}

	makeCopy := st.NewTask("make-workspace-copy", fmt.Sprintf("Copy %q workspace", w.Name))

	launch, err := launch(st, w, p)
	if err != nil {
		return nil, err
	}

	restoreStateHooks := state.NewTaskSet([]*state.Task{}...)
	prevRestore := (*state.Task)(nil)
	for _, sdk := range w.Sdks {
		restoreStateHook := hookstate.SetupHook(st, w.Name, p.ProjectId, &sdk, hookstate.RestoreState)
		restoreStateHooks.AddTask(restoreStateHook)
		if prevRestore != nil {
			restoreStateHook.WaitFor(prevRestore)
		}
		prevRestore = restoreStateHook
	}

	removeStateStorage := st.NewTask("remove-state-storage", "Unmount SDK state storage")
	deleteCopy := st.NewTask("delete-workspace-copy", fmt.Sprintf("Remove %q workspace copy", w.Name))

	// save-state -> stop-workspace -> launch -> restore state
	deleteCopy.WaitFor(removeStateStorage)
	removeStateStorage.WaitAll(restoreStateHooks)
	restoreStateHooks.WaitAll(launch)
	launch.WaitFor(makeCopy)
	makeCopy.WaitAll(saveStateHooks)
	saveStateHooks.WaitFor(createStateStorage)

	refresh := state.NewTaskSet([]*state.Task{}...)
	refresh.AddAll(saveStateHooks)
	refresh.AddAll(launch)
	refresh.AddAll(restoreStateHooks)
	refresh.AddTask(makeCopy)
	refresh.AddTask(deleteCopy)
	refresh.AddTask(createStateStorage)
	refresh.AddTask(removeStateStorage)

	refresh.MarkEdge(removeStateStorage, LastBeforeRefreshIrreversibleEdge)
	refresh.MarkEdge(deleteCopy, CleanupRefreshEdge)

	for _, i := range refresh.Tasks() {
		i.Set("workspace", w.Name)
		i.Set("project", *p)
	}

	return refresh, nil
}
