// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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
