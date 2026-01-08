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
	download.Set("sdk", s.Name)
	return download
}

func Install(st *state.State, sdk string) *state.Task {
	install := st.NewTask("install-sdk", fmt.Sprintf("Install %q SDK", sdk))
	install.Set("sdk", sdk)
	return install
}

func Register(st *state.State, sdk string) *state.Task {
	register := st.NewTask("register-sdk", fmt.Sprintf("Register %q SDK plugs and slots", sdk))
	register.Set("sdk", sdk)
	return register
}

func Unregister(st *state.State, setup sdk.Setup) *state.Task {
	unregister := st.NewTask("unregister-sdk", fmt.Sprintf("Unregister %q SDK plugs and slots", setup.Name))
	unregister.Set("sdk", setup.Name)
	unregister.Set("sdk-setup", setup)
	return unregister
}

func Snapshot(st *state.State, sdk string) *state.Task {
	snapshot := st.NewTask("snapshot-sdk", fmt.Sprintf("Snapshot %q SDK installation", sdk))
	snapshot.Set("sdk", sdk)
	return snapshot
}
