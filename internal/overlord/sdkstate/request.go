package sdkstate

import (
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
)

func Retrieve(st *state.State, sdk *workspacebackend.Sdk) *state.Task {
	download := st.NewTask("retrieve-sdk", fmt.Sprintf("Retrieving SDK %q", sdk.Name))
	download.Set("sdk-setup", sdk)
	return download
}

func Install(st *state.State, sdk string, retrieveId string) *state.TaskSet {
	tasks := []*state.Task{}

	install := st.NewTask("install-sdk", fmt.Sprintf("Installing SDK %q", sdk))
	install.Set("sdk-retrieve-task", retrieveId)
	tasks = append(tasks, install)

	link := st.NewTask("link-sdk", fmt.Sprintf("Linking SDK %q", sdk))
	link.Set("sdk-retrieve-task", retrieveId)
	link.WaitFor(install)
	tasks = append(tasks, link)

	return state.NewTaskSet(tasks...)
}
