package workspace

import (
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
)

func Launch(st *state.State, file *WorkspaceFile) (*state.TaskSet, error) {
	download_tasks, install_tasks, link_tasks := []*state.Task{}, []*state.Task{}, []*state.Task{}
	for _, sdk := range file.Sdks {
		download := st.NewTask("retrieve-sdk", fmt.Sprintf("Retrieve SDK %q", sdk.Name))
		download.Set("sdk", sdk)
		download_tasks = append(download_tasks, download)

		install := st.NewTask("install-sdk", fmt.Sprintf("Install SDK %q", sdk.Name))
		install.Set("sdk-retrieve-task", download.ID())
		install_tasks = append(install_tasks, install)

		link := st.NewTask("link-sdk", fmt.Sprintf("Link SDK %q", sdk.Name))
		link.Set("sdk-retrieve-task", download.ID())
		link.WaitFor(install)
		link_tasks = append(link_tasks, link)
	}
	downloads, installs, links := state.NewTaskSet(download_tasks...),
		state.NewTaskSet(install_tasks...), state.NewTaskSet(link_tasks...)

	create := st.NewTask("create-workspace", fmt.Sprintf("Create workspace %q", file.Name))
	create.Set("base", file.Base)
	create.WaitAll(downloads)

	mountProject := st.NewTask("mount-project", "Mount project directory")
	mountProject.WaitFor(create)

	start := st.NewTask("start-workspace", fmt.Sprintf("Start workspace %q", file.Name))
	start.WaitFor(mountProject)

	installs.WaitFor(start)

	set := state.NewTaskSet(create, mountProject, start)
	set.AddAll(downloads)
	set.AddAll(installs)
	set.AddAll(links)

	return set, nil
}
