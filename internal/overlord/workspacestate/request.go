package workspacestate

import (
	"fmt"

	"github.com/canonical/workspace/internal/overlord/hookstate"
	"github.com/canonical/workspace/internal/overlord/sdkstate"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
)

func Launch(st *state.State, file *workspacebackend.WorkspaceFile, project *workspacebackend.Project) (*state.TaskSet, error) {
	retrieve := state.NewTaskSet([]*state.Task{}...)
	install := state.NewTaskSet([]*state.Task{}...)
	setupHook := state.NewTaskSet([]*state.Task{}...)

	prevInstall := (*state.TaskSet)(nil)
	prevSetup := (*state.Task)(nil)
	for _, sdk := range file.Sdks {
		r := sdkstate.Retrieve(st, &sdk)
		retrieve.AddTask(r)

		/* install task sets must not run concurrently as exec ops are not allowed
		by LXD to be run concurrently and in general case we cannot guarantee safety of
		concurrent installations */
		installTaskSet := sdkstate.Install(st, sdk.Name, r.ID())
		if prevInstall != nil {
			installTaskSet.WaitAll(prevInstall)
		} else {
			prevInstall = installTaskSet
		}
		install.AddAll(installTaskSet)

		/* Make sure that the hook tasks are not concurrent */
		setupHookTask := hookstate.SetupHook(st, &sdk, r.ID())
		if prevSetup != nil {
			setupHookTask.WaitFor(prevSetup)
		} else {
			prevSetup = setupHookTask
		}
		setupHook.AddTask(setupHookTask)
	}

	create := st.NewTask("create-workspace", fmt.Sprintf("Create workspace %q", file.Name))
	create.Set("base", file.Base)
	create.WaitAll(retrieve)

	mountProject := st.NewTask("mount-project", fmt.Sprintf("Mount project directory %q", project.Path))
	mountProject.WaitFor(create)

	start := st.NewTask("start-workspace", fmt.Sprintf("Start workspace %q", file.Name))
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
