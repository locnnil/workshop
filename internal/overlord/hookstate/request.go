package hookstate

import (
	"fmt"
	"path/filepath"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshopbackend"
)

func SetupHook(st *state.State, sdk *workshopbackend.SdkRecord, hook WorkshopHookType) *state.Task {
	setup_hook := st.NewTask("run-hook", fmt.Sprintf("Run hook %q for %q SDK", hook.String(), sdk.Name))

	setup := HookSetup{HookType: hook, Sdk: *sdk, Environment: map[string]string{}}
	if hook == SaveState || hook == RestoreState {
		setup.Environment["SDK_STATE_DIR"] = filepath.Join(workshopbackend.WorkshopStateDir, "sdk", sdk.Name)
	}
	setup_hook.Set("hook-setup", &setup)
	return setup_hook
}
