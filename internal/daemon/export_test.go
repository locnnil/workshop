// Copyright (c) 2014-2020 Canonical Ltd
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

package daemon

import (
	"net/http"
	"time"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/overlord/healthstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/workshopstate"
	"github.com/canonical/workshop/internal/workshopbackend"
)

func FakeMuxVars(f func(*http.Request) map[string]string) (restore func()) {
	old := muxVars
	muxVars = f
	return func() {
		muxVars = old
	}
}

func FakeStateEnsureBefore(f func(st *state.State, d time.Duration)) (restore func()) {
	old := stateEnsureBefore
	stateEnsureBefore = f
	return func() {
		stateEnsureBefore = old
	}
}

func FakeWorkshopHealth(f func(mgr *workshopstate.WorkshopManager, w *workshopbackend.Workshop) healthstate.HealthState) (restore func()) {
	old := workshopHealth
	workshopHealth = f
	return func() {
		workshopHealth = old
	}
}

func FakeSdkMounts(f func(state *state.State, repo *interfaces.Repository, projectId, workshop, sdk string) []*Mount) (restore func()) {
	old := sdkMounts
	sdkMounts = f
	return func() {
		sdkMounts = old
	}
}

func MockWarningsAccessors(okay func(*state.State, time.Time) int, all func(*state.State) []*state.Warning, pending func(*state.State) ([]*state.Warning, time.Time)) (restore func()) {
	oldOK := stateOkayWarnings
	oldAll := stateAllWarnings
	oldPending := statePendingWarnings
	stateOkayWarnings = okay
	stateAllWarnings = all
	statePendingWarnings = pending
	return func() {
		stateOkayWarnings = oldOK
		stateAllWarnings = oldAll
		statePendingWarnings = oldPending
	}
}
