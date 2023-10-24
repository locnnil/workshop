package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/canonical/workshop/internal/workspacebackend"
)

type execPayload struct {
	Command     []string          `json:"command"`
	Environment map[string]string `json:"environment"`
	WorkingDir  string            `json:"working-dir"`
	Timeout     string            `json:"timeout"`
	UserId      *int              `json:"user-id"`
	GroupId     *int              `json:"group-id"`
	Terminal    bool              `json:"terminal"`
	Interactive bool              `json:"interactive"`
	SplitStderr bool              `json:"split-stderr"`
	Width       int               `json:"width"`
	Height      int               `json:"height"`
}

func normaliseUserGroupIds(usrId, grpId *int) (int, int, error) {
	if usrId != nil && grpId == nil {
		return 0, 0, errors.New("must specify group, not just user")
	}

	if usrId == nil && grpId != nil {
		return 0, 0, errors.New("must specify user, not just group")
	}

	userId := 1000
	if usrId != nil {
		userId = *usrId
	}
	groupId := 1000
	if grpId != nil {
		groupId = *grpId
	}
	return userId, groupId, nil
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

	if reqData.Terminal {
		return statusBadRequest("cannot exec: terminal mode is not supported")
	}

	if reqData.SplitStderr {
		return statusBadRequest("cannot exec: splitting stderr is not supported")
	}

	if reqData.WorkingDir == "" {
		reqData.WorkingDir = workspacebackend.WorkspaceProjectPath
	}

	var timeout time.Duration
	if reqData.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(reqData.Timeout)
		if err != nil {
			return statusBadRequest("invalid timeout: %v", err)
		}
	}

	userId, groupId, err := normaliseUserGroupIds(reqData.UserId, reqData.GroupId)
	if err != nil {
		return statusBadRequest("cannot exec: %v", err)
	}

	user, ok := r.Context().Value(workspacebackend.ContextUser).(string)
	if !ok {
		return statusBadRequest("cannot exec: user is not in context")
	}

	var execArgs = &workspacebackend.ExecArgs{
		Command:     reqData.Command,
		Environment: reqData.Environment,
		WorkDir:     reqData.WorkingDir,
		UserId:      userId,
		GroupId:     groupId,
		Timeout:     timeout,
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
	task, err := wsmgr.Exec(r.Context(), wrkspc, projectId, execArgs)
	if err != nil {
		return statusBadRequest(err.Error())
	}

	change := st.NewChange("exec", fmt.Sprintf("Execute command %q", execArgs.Command[0]))
	change.AddTask(task)

	change.Set("user", user)
	change.Set("project-id", projectId)

	ensureStateSoon(st, 0)

	result := map[string]interface{}{
		"task-id": task.ID(),
	}

	return AsyncResponse(result, change.ID())
}
