package workspacestate

import (
	"context"
	"fmt"

	"github.com/canonical/workspace/internal/overlord/hookstate"
	"github.com/canonical/workspace/internal/overlord/sdkstate"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
	"golang.org/x/exp/slices"
)

const (
	RefreshIncumbentPrefix = "refresh-incumbent-"
	RefreshStateKey        = "refresh-in-progress"
)

type RefreshInProgress map[string]RefreshSetup

type RefreshMode int

const (
	RefreshTransactional RefreshMode = iota
	RefreshHoldOnError
	RefreshContinue
	RefreshAbort
)

func (s RefreshMode) String() string {
	return [...]string{"transactional", "hold-on-error", "continue", "abort"}[s]
}

func ParseRefreshMode(s string) RefreshMode {
	refreshMap := map[string]RefreshMode{
		RefreshTransactional.String(): RefreshTransactional,
		RefreshHoldOnError.String():   RefreshHoldOnError,
		RefreshContinue.String():      RefreshContinue,
		RefreshAbort.String():         RefreshAbort,
	}
	return refreshMap[s]
}

type RefreshSetup struct {
	RefreshChangeId string
}

func LaunchMany(st *state.State, workspaces []string, project *workspacebackend.Project) ([]*state.TaskSet, error) {
	taskset := make([]*state.TaskSet, 0, len(workspaces))
	for _, i := range workspaces {
		file, err := workspacebackend.ReadWorkspace(workspacebackend.WorkspaceFilePath(project.Path, i))
		if err != nil {
			return nil, fmt.Errorf("cannot read workspace \"%s\": %v", i, err)
		}

		tasks, err := Launch(st, file, project)
		if err != nil {
			return nil, fmt.Errorf("cannot launch workspace \"%s\": %v", i, err)
		}
		taskset = append(taskset, tasks)
	}
	return taskset, nil
}

func Launch(st *state.State, file *workspacebackend.WorkspaceFile, project *workspacebackend.Project) (*state.TaskSet, error) {
	retrieve := state.NewTaskSet([]*state.Task{}...)
	install := state.NewTaskSet([]*state.Task{}...)
	setupHook := state.NewTaskSet([]*state.Task{}...)

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
		setupHookTask := hookstate.SetupHook(st, &sdk, workspacebackend.SetupBase)
		if prevSetup != nil {
			setupHookTask.WaitFor(prevSetup)
		}
		prevSetup = setupHookTask

		setupHook.AddTask(setupHookTask)
	}

	create := st.NewTask("create-workspace", fmt.Sprintf("Create %q", file.Name))
	create.Set("base", file.Base)
	create.WaitAll(retrieve)

	mountProject := st.NewTask("mount-project", fmt.Sprintf("Mount project directory %q", project.Path))
	mountProject.WaitFor(create)

	start := st.NewTask("start-workspace", fmt.Sprintf("Start %q", file.Name))
	start.WaitFor(mountProject)

	install.WaitFor(start)
	setupHook.WaitAll(install)

	set := state.NewTaskSet(create, mountProject, start)
	set.AddAll(retrieve)
	set.AddAll(install)
	set.AddAll(setupHook)

	for _, i := range set.Tasks() {
		i.Set("workspace", file.Name)
		i.Set("project-key", project)
	}

	return set, nil
}

func RefreshMany(st *state.State, ctx context.Context, backend workspacebackend.WorkspaceBackend, names []string, project *workspacebackend.Project) ([]*state.TaskSet, error) {
	taskset := make([]*state.TaskSet, 0, len(names))

	// we are only interested in the existing (launched) workspaces
	_, workspaces, err := backend.GetProjectWorkspaces(ctx)
	if err != nil {
		return nil, err
	}

	for _, i := range names {
		idx := slices.IndexFunc(workspaces, func(w *workspacebackend.Workspace) bool { return w.Name == i })
		if idx == -1 {
			return nil, fmt.Errorf("workspace %s not found", i)
		}

		workspace := workspaces[idx]

		tasks, err := Refresh(st, workspace, project)
		if err != nil {
			return nil, fmt.Errorf("cannot refresh workspace \"%s\": %v", i, err)
		}
		taskset = append(taskset, tasks)
	}
	return taskset, nil
}

func Refresh(st *state.State, w *workspacebackend.Workspace, p *workspacebackend.Project) (*state.TaskSet, error) {
	// 1. Save previous state
	// 2. Stop previous workspace
	// 3. Make unavailable
	// 4. Launch the new workspace
	// 5. Run restore state
	// 6. Delete the old workspace
	saveStateHooks := state.NewTaskSet([]*state.Task{}...)
	for _, sdk := range w.File().Sdks {
		saveStateHook := hookstate.SetupHook(st, &sdk, workspacebackend.SaveState)
		saveStateHooks.AddTask(saveStateHook)
	}

	stopOld := st.NewTask("stop-workspace", fmt.Sprintf("Stop %q", w.Name))
	startRefresh := st.NewTask("start-refresh", fmt.Sprintf("Begin refresh for %q", w.Name))

	launch, err := Launch(st, w.File(), p)
	if err != nil {
		return nil, err
	}

	restoreStateHooks := state.NewTaskSet([]*state.Task{}...)
	for _, sdk := range w.File().Sdks {
		restoreStateHook := hookstate.SetupHook(st, &sdk, workspacebackend.RestoreState)
		restoreStateHooks.AddTask(restoreStateHook)
	}

	completeRefresh := st.NewTask("complete-refresh", fmt.Sprintf("Finish refresh for %q", w.Name))

	// save-state -> stop-workspace -> launch -> restore state
	completeRefresh.WaitAll(restoreStateHooks)
	restoreStateHooks.WaitAll(launch)
	launch.WaitFor(startRefresh)
	startRefresh.WaitFor(stopOld)
	stopOld.WaitAll(saveStateHooks)

	refresh := state.NewTaskSet([]*state.Task{}...)
	refresh.AddAll(saveStateHooks)
	refresh.AddAll(launch)
	refresh.AddAll(restoreStateHooks)
	refresh.AddTask(stopOld)
	refresh.AddTask(startRefresh)
	refresh.AddTask(completeRefresh)

	for _, i := range refresh.Tasks() {
		i.Set("workspace", w.Name)
		i.Set("project-key", *p)
	}

	return refresh, nil
}
