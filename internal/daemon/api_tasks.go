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
		logger.Noticef("Websocket: cannot find task with id %q", taskID)
		return statusNotFound("cannot find task with id %q", taskID)
	}

	if task.Kind() != "exec" {
		logger.Noticef("Websocket %s: %q tasks do not have websockets", task.ID(), task.Kind())
		return statusBadRequest("%q tasks do not have websockets", task.Kind())
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
		logger.Noticef("Websocket %s: cannot find websocket with id %q", wr.task.ID(), wr.websocketID)
		rsp := statusNotFound("cannot find websocket with id %q", wr.websocketID)
		rsp.ServeHTTP(w, r)
		return
	}
	if err != nil {
		logger.Noticef("Websocket %s: cannot connect to websocket %q: %v", wr.task.ID(), wr.websocketID, err)
		rsp := statusInternalError("cannot connect to websocket %q: %v", wr.websocketID, err)
		rsp.ServeHTTP(w, r)
		return
	}
	// In the success case, Connect takes over the connection and upgrades to
	// the websocket protocol.
}
