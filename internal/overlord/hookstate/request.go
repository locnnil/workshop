package hookstate

import (
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
)

func SetupHook(st *state.State, sdk *workspacebackend.Sdk, hook workspacebackend.WorkspaceHookType) *state.Task {
	setup_hook := st.NewTask("run-hook", fmt.Sprintf("Run hook %q for SDK %q if present", hook.String(), sdk.Name))
	setup_hook.Set("hook-setup", &HookSetup{HookType: hook, Sdk: *sdk})
	return setup_hook
}
