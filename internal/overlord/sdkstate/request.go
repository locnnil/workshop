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

func InstallLocalSdk(st *state.State, setup sdk.Setup) *state.TaskSet {
	install := st.NewTask("install-local-sdk", fmt.Sprintf("Install %q SDK", setup.Name))
	install.Set("sdk-setup", setup)
	install.Set("sdk-retrieve-task", install.ID())

	link := st.NewTask("link-sdk", fmt.Sprintf("Link %q SDK", setup.Name))
	link.Set("sdk-retrieve-task", install.ID())
	link.WaitFor(install)

	return state.NewTaskSet(install, link)
}

func InstallSystemSdk(st *state.State) *state.TaskSet {
	return InstallLocalSdk(st, sdk.Setup{Name: sdk.System.String(), Revision: sdk.Revision{N: -1}})
}

func Install(st *state.State, sdk string, retrieveId string) *state.TaskSet {
	install := st.NewTask("install-sdk", fmt.Sprintf("Install %q SDK", sdk))
	install.Set("sdk-retrieve-task", retrieveId)

	link := st.NewTask("link-sdk", fmt.Sprintf("Link %q SDK", sdk))
	link.Set("sdk-retrieve-task", retrieveId)
	link.WaitFor(install)

	return state.NewTaskSet(install, link)
}
