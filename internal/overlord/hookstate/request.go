package hookstate

import (
	"fmt"
	"path/filepath"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshopbackend"
)

func Hook(st *state.State, workshop, sdk string, hook WorkshopHookType) *state.Task {
	setup_hook := st.NewTask("run-hook", fmt.Sprintf("Run hook %q for %q SDK", hook.String(), sdk))

	setup := HookSetup{HookType: hook, Workshop: workshop, Sdk: sdk, Environment: map[string]string{}}
	if hook == SaveState || hook == RestoreState {
		setup.Environment["SDK_STATE_DIR"] = filepath.Join(workshopbackend.WorkshopStateDir, "sdk", sdk)
	}
	setup_hook.Set("hook-setup", &setup)
	return setup_hook
}
