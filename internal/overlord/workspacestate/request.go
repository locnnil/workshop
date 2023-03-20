package workspace

import (
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
)

func Launch(st *state.State, file *WorkspaceFile) (*state.TaskSet, error) {
	download_tasks, install_tasks := []*state.Task{}, []*state.Task{}
	for _, sdk := range file.Sdks {
		download := st.NewTask("retrieve-sdk", fmt.Sprintf("Retrieve SDK %q", sdk.Name))
		download.Set("sdk", sdk)
		download_tasks = append(download_tasks, download)

		install := st.NewTask("install-sdk", fmt.Sprintf("Install SDK %q", sdk.Name))
		install.Set("sdk-retrieve-task", download.ID())
		install_tasks = append(install_tasks, install)
	}
	downloads, installs := state.NewTaskSet(download_tasks...), state.NewTaskSet(install_tasks...)

	create := st.NewTask("create-workspace", fmt.Sprintf("Create workspace %q", file.Name))
	create.Set("base", file.Base)
	create.WaitAll(downloads)

	addProjectDir := st.NewTask("add-workspace-device", "Mount project directory")
	addProjectDir.WaitFor(create)

	start := st.NewTask("set-workspace-state", fmt.Sprintf("Start workspace %q", file.Name))
	start.Set("workspace-state", "start")
	start.WaitFor(addProjectDir)

	installs.WaitFor(start)

	set := state.NewTaskSet(create, addProjectDir, start)
	set.AddAll(downloads)
	set.AddAll(installs)

	return set, nil
}
