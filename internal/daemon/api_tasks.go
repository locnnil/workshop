package daemon

import (
	"net/http"
	"time"

	"github.com/canonical/workspace/internal/logger"
)

const execReadyTimeout = 5 * time.Second

func v1GetTaskWebsocket(c *Command, req *http.Request, _ *userState) Response {
	vars := muxVars(req)
	taskID := vars["task-id"]
	websocketId := vars["websocket-id"]

	err := c.d.overlord.WorkspaceManager().WaitExecReady(req.Context(), taskID, execReadyTimeout)
	if err != nil {
		logger.Debugf("Websocket: exec operation is not ready: %v", err)
		return statusBadRequest("cannot exec: %v", err)
	}

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

	var location string
	err = task.Get(websocketId, &location)
	if err != nil {
		return statusNotFound("cannot find %q for the command", websocketId)
	}

	return websocketRedirectResponse{location: location}
}

type websocketRedirectResponse struct {
	location string
}

func (wr websocketRedirectResponse) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, wr.location, http.StatusTemporaryRedirect)
}
