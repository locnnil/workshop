package hookstate

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/overlord/state"
)

func hookSetup(workshop, sdk string, hook WorkshopHookType) HookSetup {
	setup := HookSetup{HookType: hook, Workshop: workshop, Sdk: sdk, Environment: map[string]string{}}
	if hook == SaveState || hook == RestoreState {
		setup.Environment["SDK_STATE_DIR"] = filepath.Join(dirs.WorkshopStateDir, "sdk", sdk)
	}
	return setup
}

func Hook(st *state.State, workshop, sdk string, hook WorkshopHookType) *state.Task {
	setup_hook := st.NewTask("run-hook", fmt.Sprintf("Run hook %q for %q SDK", hook.String(), sdk))
	setup_hook.Set("hook-setup", hookSetup(workshop, sdk, hook))
	return setup_hook
}

func HookWithTimeout(st *state.State, workshop, sdk string, hook WorkshopHookType, timeout time.Duration) *state.Task {
	setup_hook := st.NewTask("run-hook", fmt.Sprintf("Run hook %q for %q SDK", hook.String(), sdk))
	setup := hookSetup(workshop, sdk, hook)
	setup.Timeout = timeout
	setup_hook.Set("hook-setup", &setup)
	return setup_hook
}
