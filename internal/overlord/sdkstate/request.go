package sdkstate

import (
	"fmt"

	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
)

func Retrieve(st *state.State, s sdk.Setup) *state.Task {
	summary := fmt.Sprintf("Retrieve %q SDK from channel %q", s.Name, s.Channel)
	if sdk.IsSystem(s.Name) {
		summary = fmt.Sprintf("Retrieve %q SDK", s.Name)
	}

	download := st.NewTask("retrieve-sdk", summary)
	download.Set("sdk-setup", s)
	return download
}

func InstallLocalSdk(st *state.State, pid, w string, setup sdk.Setup) *state.TaskSet {
	install := st.NewTask("install-local-sdk", fmt.Sprintf("Install %q SDK", setup.Name))
	install.Set("sdk-setup", setup)
	install.Set("sdk-retrieve-task", install.ID())

	link := st.NewTask("link-sdk", fmt.Sprintf("Link %q SDK", setup.Name))
	link.Set("sdk-retrieve-task", install.ID())
	link.WaitFor(install)

	hook := hookstate.Hook(st, pid, w, setup.Name, 0, hookstate.SetupBase)
	hook.WaitFor(link)

	return state.NewTaskSet(install, link, hook)
}

func Install(st *state.State, pid, w, sdk, retrieveId string) *state.TaskSet {
	install := st.NewTask("install-sdk", fmt.Sprintf("Install %q SDK", sdk))
	install.Set("sdk-retrieve-task", retrieveId)

	link := st.NewTask("link-sdk", fmt.Sprintf("Link %q SDK", sdk))
	link.Set("sdk-retrieve-task", retrieveId)
	link.WaitFor(install)

	hook := hookstate.Hook(st, pid, w, sdk, 0, hookstate.SetupBase)
	hook.WaitFor(link)

	return state.NewTaskSet(install, link, hook)
}
