package workspacestate

import (
	"context"
	"fmt"

	"github.com/canonical/workspace/internal/overlord/hookstate"
	"github.com/canonical/workspace/internal/overlord/sdkstate"
	"github.com/canonical/workspace/internal/overlord/state"
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
	names []string, projectId string) ([]*state.TaskSet, error) {
	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
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
	return refreshMany(w.state, files, content, project)
}

func refreshMany(st *state.State, w []*workspacebackend.WorkspaceFile, content [][]*sdk.SdkInfo,
	project *workspacebackend.Project) ([]*state.TaskSet, error) {
	taskset := make([]*state.TaskSet, 0, len(w))

	for i, w := range w {
		tasks, err := refresh(st, w, content[i], project)
		if err != nil {
			return nil, fmt.Errorf("cannot refresh \"%s\" workspace: %v", w, err)
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
	// 3. Make unavailable
	// 4. Launch the new workspace
	// 5. Run restore state
	// 6. Delete the old workspace

	createStateStorage := st.NewTask("create-state-storage", "Create SDK state storage")
	saveStateHooks := saveStateHooks(st, content, file.Sdks)

	makeCopy := st.NewTask("stash-workspace", fmt.Sprintf("Stash previous %q workspace", file.Name))

	launch, err := launch(st, file, p)
	if err != nil {
		return nil, err
	}

	restoreStateHooks := restoreStateHooks(st, content, file.Sdks)

	removeStateStorage := st.NewTask("remove-state-storage", "Remove SDK state storage")
	deleteCopy := st.NewTask("remove-workspace-stash", fmt.Sprintf("Remove %q workspace from stash", file.Name))

	// save-state -> stop-workspace -> launch -> restore state
	deleteCopy.WaitFor(removeStateStorage)
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
	launch.WaitFor(makeCopy)
	if len(saveStateHooks.Tasks()) > 0 {
		makeCopy.WaitAll(saveStateHooks)
		saveStateHooks.WaitFor(createStateStorage)
	} else {
		makeCopy.WaitFor(createStateStorage)
	}

	refresh := state.NewTaskSet([]*state.Task{}...)
	refresh.AddTask(createStateStorage)
	refresh.AddAll(saveStateHooks)
	refresh.AddAllWithEdges(launch)
	refresh.AddAllWithEdges(restoreStateHooks)
	refresh.AddTask(makeCopy)
	refresh.AddTask(removeStateStorage)
	refresh.AddTask(deleteCopy)

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

func (w *WorkspaceManager) StartMany(ctx context.Context, names []string, projectId string) (*state.TaskSet, error) {
	project, err := w.loadProject(ctx, projectId)
	if err != nil {
		return nil, err
	}
	return startMany(w.state, names, project)
}

func startMany(st *state.State, names []string, project *workspacebackend.Project) (*state.TaskSet, error) {
	taskset := state.NewTaskSet([]*state.Task{}...)

	for _, name := range names {
		start := st.NewTask("start-workspace", fmt.Sprintf("Start %q workspace", name))
		start.Set("stop-operation", true)
		taskset.AddTask(start)

		start.Set("workspace", name)
		start.Set("project", *project)
	}

	return taskset, nil
}
