package workspace

import (
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
)

func Launch(st *state.State, project *Project, file *workspaceFile) (*state.TaskSet, error) {
	download_tasks, install_tasks := []*state.Task{}, []*state.Task{}
	for i, sdk := range file.Sdks {
		download := st.NewTask("retrieve-sdk", fmt.Sprintf("Retrieve SDK %q", i))
		download.Set("sdk", sdk)
		download_tasks = append(download_tasks, download)

		install := st.NewTask("install-sdk", fmt.Sprintf("Install SDK %q", i))
		install.Set("sdk-retrieve-task", download.ID())
		install_tasks = append(install_tasks, install)
	}
	downloads, installs := state.NewTaskSet(download_tasks...), state.NewTaskSet(install_tasks...)

	create := st.NewTask("create-base", fmt.Sprintf("Create workspace %q base", file.Name))
	create.Set("workspace-base", file.Base)
	create.WaitAll(downloads)

	addProjectDir := st.NewTask("add-device", fmt.Sprintf("Mount project directory %q ", project.ProjectId()))
	addProjectDir.WaitFor(create)

	start := st.NewTask("set-state", fmt.Sprintf("Start workspace %q", project.ProjectId()))
	start.Set("workspace-state", "start")
	start.WaitFor(addProjectDir)

	installs.WaitFor(start)

	set := state.NewTaskSet(create, addProjectDir, start)
	set.AddAll(downloads)
	set.AddAll(installs)

	return set, nil
}
