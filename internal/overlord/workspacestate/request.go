package workspacestate

import (
	"fmt"

	util "github.com/canonical/workspace/internal"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
)

func Launch(st *state.State, file *workspacebackend.WorkspaceFile) (*state.TaskSet, error) {
	download_tasks, install_tasks, link_tasks := []*state.Task{}, []*state.Task{}, []*state.Task{}
	setup_hook_tasks := []*state.Task{}
	for _, sdk := range file.Sdks {
		download := st.NewTask("retrieve-sdk", fmt.Sprintf("Retrieve SDK %q", sdk.Name))
		download.Set("sdk", sdk)
		download_tasks = append(download_tasks, download)

		install := st.NewTask("install-sdk", fmt.Sprintf("Install SDK %q", sdk.Name))
		install.Set("sdk-retrieve-task", download.ID())
		install_tasks = append(install_tasks, install)

		link := st.NewTask("link-sdk", fmt.Sprintf("Link SDK %q", sdk.Name))
		link.Set("sdk-retrieve-task", download.ID())
		link_tasks = append(link_tasks, link)

		setup_hook := st.NewTask("run-hook", fmt.Sprintf("setup-base %q", sdk.Name))
		setup_hook.Set("hook-setup", util.SetupBase)
		setup_hook.Set("sdk-retrieve-task", download.ID())
		setup_hook_tasks = append(setup_hook_tasks, setup_hook)
	}
	downloads, installs, links := state.NewTaskSet(download_tasks...),
		state.NewTaskSet(install_tasks...), state.NewTaskSet(link_tasks...)

	setup_hooks := state.NewTaskSet(setup_hook_tasks...)

	create := st.NewTask("create-workspace", fmt.Sprintf("Create workspace %q", file.Name))
	create.Set("base", file.Base)
	create.WaitAll(downloads)

	mountProject := st.NewTask("mount-project", "Mount project directory")
	mountProject.WaitFor(create)

	start := st.NewTask("start-workspace", fmt.Sprintf("Start workspace %q", file.Name))
	start.WaitFor(mountProject)

	installs.WaitFor(start)
	links.WaitAll(installs)
	setup_hooks.WaitAll(links)

	set := state.NewTaskSet(create, mountProject, start)
	set.AddAll(downloads)
	set.AddAll(installs)
	set.AddAll(links)
	set.AddAll(setup_hooks)

	return set, nil
}
