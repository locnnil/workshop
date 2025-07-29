package sdkstate

import (
	"fmt"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
)

func Retrieve(st *state.State, s sdk.Setup) *state.Task {
	summary := fmt.Sprintf("Retrieve %q SDK from channel %q", s.Name, s.Channel)
	if s.Source != sdk.StoreSource {
		summary = fmt.Sprintf("Retrieve %q SDK", s.Name)
	}

	download := st.NewTask("retrieve-sdk", summary)
	download.Set("sdk-setup", s)
	return download
}

func InstallLocalSdk(st *state.State, setup sdk.Setup) *state.Task {
	install := st.NewTask("install-sdk", fmt.Sprintf("Install %q SDK", setup.Name))
	install.Set("sdk-setup", setup)
	install.Set("sdk-retrieve-task", install.ID())
	return install
}

func Install(st *state.State, sdk, retrieveId string) *state.Task {
	install := st.NewTask("install-sdk", fmt.Sprintf("Install %q SDK", sdk))
	install.Set("sdk-retrieve-task", retrieveId)
	return install
}

func Register(st *state.State, sdk, retrieveId string) *state.Task {
	register := st.NewTask("register-sdk", fmt.Sprintf("Register %q SDK plugs and slots", sdk))
	register.Set("sdk-retrieve-task", retrieveId)
	return register
}

func Unregister(st *state.State, setup sdk.Setup) *state.Task {
	unregister := st.NewTask("unregister-sdk", fmt.Sprintf("Unregister %q SDK plugs and slots", setup.Name))
	unregister.Set("sdk-retrieve-task", unregister.ID())
	unregister.Set("sdk-setup", setup)
	return unregister
}
