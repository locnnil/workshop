package hookstate

import (
	"fmt"

	util "github.com/canonical/workspace/internal"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
)

func SetupHook(st *state.State, sdk *workspacebackend.Sdk, retrieveId string) *state.Task {
	setup_hook := st.NewTask("run-hook", fmt.Sprintf("setup-base %q", sdk.Name))
	setup_hook.Set("hook-setup", util.SetupBase)
	setup_hook.Set("sdk-retrieve-task", retrieveId)
	return setup_hook
}
