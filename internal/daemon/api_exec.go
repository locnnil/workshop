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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshop"
)

type execPayload struct {
	// Command is the user-requested command. It is used for user-facing
	// summaries and diagnostics.
	Command []string `json:"command"`

	// CommandPrefix is a Workshop-managed wrapper prepended to [Command] at the
	// execution boundary. Older clients may omit this field.
	CommandPrefix []string          `json:"command-prefix"`
	Action        bool              `json:"action"`
	Environment   map[string]string `json:"environment"`
	WorkingDir    string            `json:"working-dir"`
	Timeout       string            `json:"timeout"`
	UserId        *int              `json:"user-id"`
	GroupId       *int              `json:"group-id"`
	Terminal      bool              `json:"terminal"`
	Interactive   bool              `json:"interactive"`
	SplitStderr   bool              `json:"split-stderr"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
}

func normaliseUserGroupIds(usrId, grpId *int) (int, int, error) {
	if usrId != nil && grpId == nil {
		return 0, 0, errors.New("must specify group, not just user")
	}

	if usrId == nil && grpId != nil {
		return 0, 0, errors.New("must specify user, not just group")
	}

	userId := workshop.Uid
	if usrId != nil {
		userId = *usrId
	}
	groupId := workshop.Gid
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
		return statusBadRequest("cannot exec: failed to decode data from request body: %w", err)
	}

	action := "exec"
	subject := "command"
	if reqData.Action {
		action = "run"
		subject = "action"
	}

	if len(reqData.Command) < 1 {
		return statusBadRequest("cannot %s %s in %q: must specify %s", action, subject, wrkspc, subject)
	}

	if reqData.Terminal {
		return statusBadRequest("cannot %s %s in %q: terminal mode is not supported", action, subject, wrkspc)
	}

	if reqData.SplitStderr {
		return statusBadRequest("cannot %s %s in %q: splitting stderr is not supported", action, subject, wrkspc)
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
			return statusBadRequest("invalid timeout: %w", err)
		}
	}

	userId, groupId, err := normaliseUserGroupIds(reqData.UserId, reqData.GroupId)
	if err != nil {
		return statusBadRequest("cannot %s %s in %q: %w", action, subject, wrkspc, err)
	}

	// We do not set PATH here as it's something LXD takes care of. Set HOME and
	// USER if the user is a known default user.
	if userId == workshop.Uid {
		if environment["HOME"] == "" {
			environment["HOME"] = workshop.User.HomeDir
		}
		if environment["USER"] == "" {
			environment["USER"] = workshop.User.Username
		}
	}

	// Set default value for LANG.
	_, ok := environment["LANG"]
	if !ok {
		environment["LANG"] = "C.UTF-8"
	}

	user, ok := r.Context().Value(workshop.ContextUser).(string)
	if !ok {
		return statusBadRequest("cannot %s %s in %q: user is not in context", action, subject, wrkspc)
	}

	var execArgs = &workshop.ExecArgs{
		Command:       reqData.Command,
		CommandPrefix: reqData.CommandPrefix,
		Environment:   environment,
		WorkDir:       reqData.WorkingDir,
		UserId:        userId,
		GroupId:       groupId,
		Timeout:       timeout,
		Terminal:      reqData.Terminal,
		Interactive:   reqData.Interactive,
		SplitStderr:   reqData.SplitStderr,
		Width:         reqData.Width,
		Height:        reqData.Height,
	}

	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()

	wsmgr := c.d.overlord.WorkshopManager()
	taskset, err := wsmgr.Exec(r.Context(), wrkspc, projectId, execArgs, reqData.Action)
	if err != nil {
		return statusBadRequest("cannot %s %s in %q: %w", action, subject, wrkspc, err)
	}

	change := st.NewChange("exec", fmt.Sprintf("Execute %s %q", subject, execArgs.Command[0]))
	change.AddAll(taskset)

	change.Set("user", user)
	change.Set("project-id", projectId)

	ensureStateSoon(st, 0)

	tasks := taskset.Tasks()
	idx := slices.IndexFunc(tasks, func(t *state.Task) bool { return t.Kind() == "exec" })
	if idx < 0 {
		return statusInternalError(`cannot find "exec" task`)
	}
	result := map[string]any{
		"task-id": tasks[idx].ID(),
	}

	return AsyncResponse(result, change.ID())
}
