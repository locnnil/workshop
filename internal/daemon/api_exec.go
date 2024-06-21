package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/canonical/workshop/internal/workshop"
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

var defaultUID, defaultGID = 1000, 1000

func normaliseUserGroupIds(usrId, grpId *int) (int, int, error) {
	if usrId != nil && grpId == nil {
		return 0, 0, errors.New("must specify group, not just user")
	}

	if usrId == nil && grpId != nil {
		return 0, 0, errors.New("must specify user, not just group")
	}

	userId := defaultUID
	if usrId != nil {
		userId = *usrId
	}
	groupId := defaultGID
	if grpId != nil {
		groupId = *grpId
	}
	return userId, groupId, nil
}

func v1PostWorkshopExec(c *Command, r *http.Request, _ *userState) Response {
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
		reqData.WorkingDir = workshop.WorkshopProjectPath
	}

	var environment = make(map[string]string)

	for k, e := range reqData.Environment {
		environment[k] = e
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

	// We do not set PATH here as it's something LXD takes care of. Set HOME and
	// USER if the user is a known default user.
	if userId == defaultUID {
		if environment["HOME"] == "" {
			environment["HOME"] = "/home/workshop"
		}
		if environment["USER"] == "" {
			environment["USER"] = "workshop"
		}
	}

	// Set default value for LANG.
	_, ok := environment["LANG"]
	if !ok {
		environment["LANG"] = "C.UTF-8"
	}

	user, ok := r.Context().Value(workshop.ContextUser).(string)
	if !ok {
		return statusBadRequest("cannot exec: user is not in context")
	}

	var execArgs = &workshop.ExecArgs{
		Command:     reqData.Command,
		Environment: environment,
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

	wsmgr := c.d.overlord.WorkshopManager()
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
