package hookstate

import (
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
)

func SetupHook(st *state.State, sdk *workspacebackend.Sdk, retrieveId string, hook workspacebackend.WorkspaceHookType) *state.Task {
	setup_hook := st.NewTask("run-hook", fmt.Sprintf("Run hook %q for SDK %q", hook.String(), sdk.Name))
	setup_hook.Set("hook-setup", &HookSetup{HookType: hook})
	setup_hook.Set("sdk-retrieve-task", retrieveId)
	return setup_hook
}
