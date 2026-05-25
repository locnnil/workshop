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

package daemon

import (
	"errors"
	"net/http"
	"os"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/overlord/state"
)

type websocketConnectFunc func(r *http.Request, w http.ResponseWriter, task *state.Task, websocketID string) error

type websocketResponse struct {
	task        *state.Task
	websocketID string
	connect     websocketConnectFunc
}

func v1GetTaskWebsocket(c *Command, req *http.Request, _ *userState) Response {
	vars := muxVars(req)
	taskID := vars["task-id"]
	websocketId := vars["websocket-id"]

	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()

	task := st.Task(taskID)
	if task == nil {
		logger.Noticef("Websocket: cannot find task %q", taskID)
		return statusNotFound("cannot find task %q", taskID)
	}

	if task.Kind() != "exec" {
		logger.Noticef("Websocket %s: %s tasks do not have websockets", task.ID(), task.Kind())
		return statusBadRequest("%s task %q has no websockets", task.Kind(), task.ID())
	}

	cmdmgr := c.d.overlord.CommandManager()

	return websocketResponse{
		task:        task,
		websocketID: websocketId,
		connect:     cmdmgr.Connect,
	}
}

func (wr websocketResponse) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := wr.connect(r, w, wr.task, wr.websocketID)
	if errors.Is(err, os.ErrNotExist) {
		logger.Noticef("Websocket %s: cannot find %s websocket", wr.task.ID(), wr.websocketID)
		rsp := statusNotFound("cannot find %s websocket for task %q", wr.websocketID, wr.task.ID())
		rsp.ServeHTTP(w, r)
	} else if err != nil {
		logger.Noticef("Websocket %s: cannot connect to %s websocket: %v", wr.task.ID(), wr.websocketID, err)
		rsp := statusInternalError("%w", err)
		rsp.ServeHTTP(w, r)
	}
	// In the success case, Connect takes over the connection and upgrades to
	// the websocket protocol.
}
