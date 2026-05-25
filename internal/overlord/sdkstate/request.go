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

package sdkstate

import (
	"fmt"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
)

func Retrieve(st *state.State, s sdk.Setup) *state.Task {
	summary := fmt.Sprintf("Retrieve %q SDK from channel %q", s.Name, s.Channel)
	if s.Source != sdk.StoreSource {
		summary = fmt.Sprintf("Retrieve %q SDK", s.Name)
	}

	download := st.NewTask("retrieve-sdk", summary)
	download.Set("sdk", s.Name)
	return download
}

func Install(st *state.State, sdk string) *state.Task {
	install := st.NewTask("install-sdk", fmt.Sprintf("Install %q SDK", sdk))
	install.Set("sdk", sdk)
	return install
}

func Uninstall(st *state.State, setup sdk.Setup) *state.Task {
	uninstall := st.NewTask("uninstall-sdk", fmt.Sprintf("Uninstall %q SDK", setup.Name))
	uninstall.Set("sdk", setup.Name)
	uninstall.Set("sdk-setup", setup)
	return uninstall
}

func Snapshot(st *state.State, sdk string) *state.Task {
	snapshot := st.NewTask("snapshot-sdk", fmt.Sprintf("Snapshot %q SDK installation", sdk))
	snapshot.Set("sdk", sdk)
	return snapshot
}
