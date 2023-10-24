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
	"github.com/gorilla/mux"

	"github.com/canonical/workshop/internal/overlord/state"
)

var api = []*Command{{
	// See daemon.go:canAccess for details how the access is controlled.
	Path:    "/v1/projects",
	GuestOK: false,
	UserOK:  true,
	GET:     v1GetProjects,
	POST:    v1PostProjects,
}, {
	Path:    "/v1/projects/{id}/workspaces",
	GuestOK: false,
	UserOK:  true,
	GET:     v1GetProjectWorkspaces,
	POST:    v1PostProjectWorkspace,
}, {
	Path:    "/v1/projects/{id}/workspaces/{name}/exec",
	GuestOK: false,
	UserOK:  true,
	POST:    v1PostWorkspaceExec,
}, {
	Path:    "/v1/projects/{id}/workspaces/{name}",
	GuestOK: false,
	UserOK:  true,
	GET:     v1GetProjectWorkspace,
}, {
	Path:   "/v1/changes",
	UserOK: true,
	GET:    v1GetChanges,
}, {
	Path:   "/v1/changes/{id}",
	UserOK: true,
	GET:    v1GetChange,
	POST:   v1PostChange,
}, {
	Path:   "/v1/changes/{id}/wait",
	UserOK: true,
	GET:    v1GetChangeWait,
}, {
	Path:   "/v1/tasks/{task-id}/websocket/{websocket-id}",
	UserOK: true,
	GET:    v1GetTaskWebsocket,
}}

var (
	stateOkayWarnings    = (*state.State).OkayWarnings
	stateAllWarnings     = (*state.State).AllWarnings
	statePendingWarnings = (*state.State).PendingWarnings
	stateEnsureBefore    = (*state.State).EnsureBefore
	muxVars              = mux.Vars
)
