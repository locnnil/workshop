package daemon

import (
	"net/http"

	"github.com/canonical/workspace/internal/logger"
)

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

	var location string
	err := task.Get(websocketId, &location)
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
