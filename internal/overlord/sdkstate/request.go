package sdkstate

import (
	"fmt"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
)

func Retrieve(st *state.State, s sdk.Setup) *state.Task {
	download := st.NewTask("retrieve-sdk", fmt.Sprintf("Retrieve %q SDK from channel %q", s.Name, s.Channel))
	download.Set("sdk-setup", s)
	return download
}

func InstallHostSdk(st *state.State) *state.TaskSet {
	name := sdk.Host.String()
	install := st.NewTask("install-host-sdk", fmt.Sprintf("Install %q SDK", name))
	install.Set("sdk-setup", sdk.Setup{
		Name: name,
	})
	install.Set("sdk-retrieve-task", install.ID())

	link := st.NewTask("link-sdk", fmt.Sprintf("Link %q SDK", name))
	link.Set("sdk-retrieve-task", install.ID())
	link.WaitFor(install)

	return state.NewTaskSet(install, link)
}

func Install(st *state.State, sdk string, retrieveId string) *state.TaskSet {
	install := st.NewTask("install-sdk", fmt.Sprintf("Install %q SDK", sdk))
	install.Set("sdk-retrieve-task", retrieveId)

	link := st.NewTask("link-sdk", fmt.Sprintf("Link %q SDK", sdk))
	link.Set("sdk-retrieve-task", retrieveId)
	link.WaitFor(install)

	return state.NewTaskSet(install, link)
}
