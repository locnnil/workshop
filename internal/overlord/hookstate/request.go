package hookstate

import (
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
)

func SetupHook(st *state.State, sdk *workspacebackend.Sdk, retrieveId string) *state.Task {
	setup_hook := st.NewTask("run-hook", fmt.Sprintf("Run hook \"setup-base\" for SDK %q", sdk.Name))
	setup_hook.Set("hook-setup", workspacebackend.SetupBase)
	setup_hook.Set("sdk-retrieve-task", retrieveId)
	return setup_hook
}
