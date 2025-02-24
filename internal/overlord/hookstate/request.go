package hookstate

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/overlord/state"
)

func Hook(st *state.State, pid, workshop, sdk string, timeout time.Duration, hook WorkshopHookType) *state.Task {
	setup_hook := st.NewTask("run-hook", fmt.Sprintf("Run hook %q for %q SDK", hook.String(), sdk))
	setup := HookSetup{HookType: hook, ProjectId: pid, Workshop: workshop, Sdk: sdk, Environment: map[string]string{}}
	if hook == SaveState || hook == RestoreState {
		setup.Environment["SDK_STATE_DIR"] = filepath.Join(dirs.WorkshopStateDir, "sdk", sdk)
	}
	setup.Timeout = timeout
	setup_hook.Set("hook-setup", &setup)
	return setup_hook
}
