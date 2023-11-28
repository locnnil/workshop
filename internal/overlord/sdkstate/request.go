package sdkstate

import (
	"fmt"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshopbackend"
)

func Retrieve(st *state.State, sdk *workshopbackend.SdkRecord) *state.Task {
	download := st.NewTask("retrieve-sdk", fmt.Sprintf("Retrieve %q SDK from channel %q", sdk.Name, sdk.Channel))
	download.Set("sdk-record", sdk)
	return download
}

func Install(st *state.State, sdk string, retrieveId string) *state.TaskSet {
	tasks := []*state.Task{}

	install := st.NewTask("install-sdk", fmt.Sprintf("Install %q SDK", sdk))
	install.Set("sdk-retrieve-task", retrieveId)
	tasks = append(tasks, install)

	link := st.NewTask("link-sdk", fmt.Sprintf("Link %q SDK", sdk))
	link.Set("sdk-retrieve-task", retrieveId)
	link.WaitFor(install)
	tasks = append(tasks, link)

	autoconnect := st.NewTask("auto-connect", fmt.Sprintf("Auto-connect %q SDK interfaces", sdk))
	autoconnect.Set("sdk", sdk)
	autoconnect.WaitFor(link)
	tasks = append(tasks, autoconnect)

	return state.NewTaskSet(tasks...)
}
