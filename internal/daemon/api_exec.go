package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/canonical/workspace/internal/workspacebackend"
)

type execPayload struct {
	Command     []string          `json:"command"`
	Environment map[string]string `json:"environment"`
	WorkingDir  string            `json:"working-dir"`
	UserId      int               `json:"user-id"`
	GroupId     int               `json:"group-id"`
	Terminal    bool              `json:"terminal"`
	Interactive bool              `json:"interactive"`
	SplitStderr bool              `json:"split-stderr"`
	Width       int               `json:"width"`
	Height      int               `json:"height"`
}

func v1PostWorkspaceExec(c *Command, r *http.Request, _ *userState) Response {
	wrkspc := muxVars(r)["name"]
	projectId := muxVars(r)["id"]

	var reqData execPayload

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		return statusBadRequest("cannot exec: failed to decode data from request body: %v", err)
	}

	if len(reqData.Command) < 1 {
		return statusBadRequest("cannot exec: must specify command")
	}

	user, ok := r.Context().Value(workspacebackend.ContextUser).(string)
	if !ok {
		return statusBadRequest("cannot exec: user is not in context")
	}

	var execArgs = &workspacebackend.ExecArgs{
		Command:     reqData.Command,
		Environment: reqData.Environment,
		WorkDir:     reqData.WorkingDir,
		UserId:      reqData.UserId,
		GroupId:     reqData.GroupId,
		Terminal:    reqData.Terminal,
		Interactive: reqData.Interactive,
		SplitStderr: reqData.SplitStderr,
		Width:       reqData.Width,
		Height:      reqData.Height,
	}

	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()

	wsmgr := c.d.overlord.WorkspaceManager()
	task, metadata, err := wsmgr.Exec(r.Context(), wrkspc, projectId, execArgs)
	if err != nil {
		return statusBadRequest("cannot exec: %v", err)
	}

	change := st.NewChange("exec", fmt.Sprintf("Execute command %q", execArgs.Command[0]))
	change.AddTask(task)

	change.Set("user", user)
	change.Set("project-id", projectId)

	ensureStateSoon(st, 0)

	result := map[string]interface{}{
		"environment": metadata.Environment,
		"task-id":     task.ID(),
		"working-dir": metadata.WorkingDir,
	}

	return AsyncResponse(result, change.ID())
}
