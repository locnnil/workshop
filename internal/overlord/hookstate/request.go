package hookstate

import (
	"fmt"
	"path/filepath"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
)

func SetupHook(st *state.State, sdk *workspacebackend.Sdk, hook WorkspaceHookType) *state.Task {
	setup_hook := st.NewTask("run-hook", fmt.Sprintf("Run hook %q for %q SDK", hook.String(), sdk.Name))

	setup := HookSetup{HookType: hook, Sdk: *sdk, Environment: map[string]string{}}
	if hook == SaveState || hook == RestoreState {
		setup.Environment["SDK_STATE_DIR"] = filepath.Join(workspacebackend.WorkspaceStateDir, "sdk", sdk.Name)
	}
	setup_hook.Set("hook-setup", &setup)
	return setup_hook
}
