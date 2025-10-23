package hookstate

import (
	"fmt"
	"time"

	"github.com/canonical/workshop/internal/overlord/state"
)

func Hook(st *state.State, sdk string, timeout time.Duration, hook WorkshopHookType) *state.Task {
	setup_hook := st.NewTask("run-hook", fmt.Sprintf("Run hook %q for %q SDK", hook.String(), sdk))
	setup := HookSetup{HookType: hook, Sdk: sdk, Timeout: timeout}
	setup_hook.Set("hook-setup", &setup)
	return setup_hook
}
