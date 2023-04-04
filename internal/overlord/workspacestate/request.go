package workspacestate

import (
	"fmt"

	"github.com/canonical/workspace/internal/overlord/hookstate"
	"github.com/canonical/workspace/internal/overlord/sdkstate"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
)

func Launch(st *state.State, file *workspacebackend.WorkspaceFile) (*state.TaskSet, error) {
	retrieve := state.NewTaskSet([]*state.Task{}...)
	install := state.NewTaskSet([]*state.Task{}...)
	setupHook := state.NewTaskSet([]*state.Task{}...)

	for _, sdk := range file.Sdks {
		r := sdkstate.Retrieve(st, &sdk)
		retrieve.AddTask(r)

		install.AddAll(sdkstate.Install(st, &sdk, r.ID()))

		setupHook.AddTask(hookstate.SetupHook(st, &sdk, r.ID()))
	}

	create := st.NewTask("create-workspace", fmt.Sprintf("Create workspace %q", file.Name))
	create.Set("base", file.Base)
	create.WaitAll(retrieve)

	mountProject := st.NewTask("mount-project", "Mount project directory")
	mountProject.WaitFor(create)

	start := st.NewTask("start-workspace", fmt.Sprintf("Start workspace %q", file.Name))
	start.WaitFor(mountProject)

	install.WaitFor(start)
	setupHook.WaitAll(install)

	set := state.NewTaskSet(create, mountProject, start)
	set.AddAll(retrieve)
	set.AddAll(install)
	set.AddAll(setupHook)

	return set, nil
}
