package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/overlord/workspacestate"
	"github.com/canonical/workspace/internal/workspacebackend"
	"github.com/canonical/x-go/strutil"
	"golang.org/x/exp/maps"
)

type SdkInfo struct {
	Name     string `json:"name"`
	Channel  string `json:"channel"`
	Revision string `json:"revision"`
}

type WorkspaceInfo struct {
	Name         string     `json:"name"`
	ProjectId    string     `json:"project-id"`
	State        string     `json:"state"`
	Content      []*SdkInfo `json:"content,omitempty"`
	Notes        []string   `json:"notes,omitempty"`
	RefreshChgId string     `json:"refresh-change-id,omitempty"`
}

var ensureStateSoon = stateEnsureBefore

func workspaceFileToInfo(file *workspacebackend.WorkspaceFile, pid string) *WorkspaceInfo {
	var ws WorkspaceInfo
	ws.Name = file.Name
	ws.ProjectId = pid
	ws.State = workspacebackend.Off.String()
	for _, i := range file.Sdks {
		ws.Content = append(ws.Content, &SdkInfo{
			Name:    i.Name,
			Channel: i.Channel,
		})
	}
	return &ws
}

func workspacePropsToInfo(props *workspacebackend.Workspace, pid string) *WorkspaceInfo {
	var ws WorkspaceInfo
	ws.Name = props.Name
	ws.ProjectId = pid // props.Config[workspacebackend.ProjectIdConfig]
	ws.State = props.State().String()

	// TODO: the order of SDK records is undetermined, we need the latest SDK revision
	// if there are multiple revisions
	for _, val := range props.Content() {
		ws.Content = append(ws.Content, &SdkInfo{val.Name, val.Channel, strconv.FormatInt(val.Revision, 10)})
	}

	if props.Reason() != workspacebackend.None {
		ws.Notes = append(ws.Notes, props.Reason().String())
	}

	return &ws
}

func v1GetProjects(c *Command, r *http.Request, _ *userState) Response {
	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()

	// In this scenario, we will have go walk all projects in the system
	// and also make sure these are up-to-date, this is what RetrieveWorkspacesGlobal does
	// and returns a list of workspaces for every project found in the system
	projects, err := c.d.overlord.WorkspaceBackend().Projects(r.Context())
	if err != nil {
		return statusInternalError("cannot get projects list: %v", err)
	}

	return SyncResponse(maps.Values(projects), http.StatusOK)
}

func v1PostProjects(c *Command, r *http.Request, _ *userState) Response {
	state := c.d.overlord.State()
	state.Lock()
	defer state.Unlock()

	var reqData struct {
		Path string `json:"path"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		return statusBadRequest("cannot decode data from request body: %v", err)
	}

	wBackend := c.d.overlord.WorkspaceBackend()

	prj, created, err := wBackend.CreateOrLoadProject(r.Context(), reqData.Path)
	if err != nil && !errors.Is(err, workspacebackend.ErrNotAProject) {
		return statusInternalError("cannot create or load project: %v", err)
	} else if errors.Is(err, workspacebackend.ErrNotAProject) {
		return statusBadRequest("%v", err)
	}

	if created {
		return SyncResponse(prj, http.StatusCreated)
	} else {
		return SyncResponse(prj, http.StatusOK)
	}
}

func v1GetProjectWorkspaces(c *Command, r *http.Request, _ *userState) Response {
	projectId := muxVars(r)["id"]
	state := c.d.overlord.State()
	state.Lock()
	defer state.Unlock()

	query := r.URL.Query()
	wsState := query.Get("state")
	if wsState == "" {
		wsState = "all"
	}

	wBackend := c.d.overlord.WorkspaceBackend()

	// project-id must be in the context for this query
	ctx := context.WithValue(r.Context(), workspacebackend.ContextProjectId, projectId)

	files, workspaces, err := wBackend.GetProjectWorkspaces(ctx)
	if err != nil {
		return statusInternalError("cannot list workspaces: %v", err)
	}

	var wsInfos = make([]*WorkspaceInfo, 0)
	for _, i := range workspaces {
		if wsState != "all" {
			if strings.ToLower(i.State().String()) != wsState {
				continue
			}
		}
		wsInfos = append(wsInfos, workspacePropsToInfo(i, projectId))
	}

	// Now, if the client wants only workspace files or just queried everything
	// available, we add workspace files to the response (note these only exist
	// as files, not instances)
	if wsState == "all" || wsState == "off" {
		for _, j := range files {
			wsInfos = append(wsInfos, workspaceFileToInfo(j, projectId))
		}
	}

	return SyncResponse(wsInfos, http.StatusOK)
}

func v1PostProjectWorkspace(c *Command, r *http.Request, _ *userState) Response {
	projectId := muxVars(r)["id"]
	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()
	wBackend := c.d.overlord.WorkspaceBackend()

	var reqData struct {
		Names  []string `json:"names"`
		Action string   `json:"action"`
		Mode   string   `json:"refresh-mode"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		return statusBadRequest("cannot decode data from request body: %v", err)
	}
	refreshMode := workspacestate.ParseRefreshMode(reqData.Mode)

	if len(reqData.Names) == 0 {
		return statusBadRequest("at least one workspace name must be provided")
	}

	if len(reqData.Names) > 1 && refreshMode != workspacestate.RefreshTransactional {
		return statusBadRequest("wait-on-error is not supported for multiple workspaces")
	}

	projects, err := wBackend.Projects(r.Context())
	if err != nil {
		return statusBadRequest("no project found with \"id\" %v", projectId)
	}

	prj, ok := projects[projectId]
	if !ok {
		return statusBadRequest("no project found with \"id\" %v", projectId)
	}

	user, ok := r.Context().Value(workspacebackend.ContextUser).(string)
	if !ok {
		return statusBadRequest("user is not known")
	}

	// project-id must be in the context for this query
	ctx := context.WithValue(r.Context(), workspacebackend.ContextProjectId, projectId)

	var change *state.Change
	switch reqData.Action {
	case "launch":
		var summary string
		switch len(reqData.Names) {
		case 1:
			summary = fmt.Sprintf("Launch workspace %q", reqData.Names[0])
		default:
			summary = fmt.Sprintf("Launch workspaces %s", strutil.Quoted(reqData.Names))
		}

		change = st.NewChange("launch", summary)
		change.Set("user", user)
		change.Set("project-key", prj)

		taskset, err := workspacestate.LaunchMany(st, reqData.Names, prj)
		if err != nil {
			return statusBadRequest(err.Error())
		}

		for _, i := range taskset {
			change.AddAll(i)
		}
	case "refresh":
		wm := c.d.overlord.WorkspaceManager()
		if refreshMode == workspacestate.RefreshTransactional || refreshMode == workspacestate.RefreshWaitOnError {
			var inProgress = []string{}
			for _, r := range reqData.Names {
				if wm.RefreshInProgress(st, r, prj.ProjectId) != "" {
					inProgress = append(inProgress, r)
				}
			}

			if len(inProgress) > 0 {
				return statusBadRequest("refresh operation is already in progress for: %s", strings.Join(inProgress, ","))
			}

			taskset, err := workspacestate.RefreshMany(st, ctx, wBackend, reqData.Names, prj)
			if err != nil {
				return statusBadRequest(err.Error())
			}

			var summary string
			switch len(reqData.Names) {
			case 1:
				summary = fmt.Sprintf("Refresh workspace %q", reqData.Names[0])
			default:
				summary = fmt.Sprintf("Refresh workspaces %s", strutil.Quoted(reqData.Names))
			}

			change = st.NewChange("refresh", summary)
			change.Set("user", user)
			change.Set("project-key", prj)

			if refreshMode == workspacestate.RefreshWaitOnError {
				change.Set("wait-on-error", true)
			} else {
				change.Set("wait-on-error", false)
			}

			for _, i := range taskset {
				change.AddAll(i)
			}
		}

		if refreshMode == workspacestate.RefreshContinue || refreshMode == workspacestate.RefreshAbort {
			currentRefreshId := wm.RefreshInProgress(st, reqData.Names[0], prj.ProjectId)

			if currentRefreshId == "" {
				return statusBadRequest("cannot %s, no refresh in progress", reqData.Mode)
			}

			change = st.Change(currentRefreshId)
			if change == nil {
				return statusInternalError("cannot %s, no refresh in progress", reqData.Mode)
			}

			for _, tsk := range change.Tasks() {
				if tsk.Status() == state.WaitStatus {
					waited := tsk.WaitedStatus()
					tsk.SetStatus(waited)
					tsk.ClearLog()
				}
			}

			if refreshMode == workspacestate.RefreshAbort {
				change.Abort()
			}
		}

	default:
		return statusBadRequest("unknown action")
	}

	ensureStateSoon(st, 0)

	return AsyncResponse(nil, change.ID())

}

func v1GetProjectWorkspace(c *Command, r *http.Request, _ *userState) Response {
	projectId := muxVars(r)["id"]
	name := muxVars(r)["name"]

	if projectId == "" {
		return statusBadRequest("project-id must be provided")
	}

	if name == "" {
		return statusBadRequest("workspace name must be provided")
	}

	state := c.d.overlord.State()
	state.Lock()
	defer state.Unlock()

	wBackend := c.d.overlord.WorkspaceBackend()

	// project-id must be in the context for this query
	ctx := context.WithValue(r.Context(), workspacebackend.ContextProjectId, projectId)
	workspace, err := wBackend.GetWorkspace(ctx, name)
	if err != nil {
		return statusNotFound("cannot get workspace: %v", err)
	}

	return SyncResponse(workspacePropsToInfo(workspace, projectId), http.StatusOK)
}
