package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/overlord/statecontext"
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
	ws.State = workspacebackend.WorkspaceOff.String()
	for _, i := range file.Sdks {
		ws.Content = append(ws.Content, &SdkInfo{
			Name:    i.Name,
			Channel: i.Channel,
		})
	}
	return &ws
}

func workspacePropsToInfo(props *workspacebackend.Workspace) *WorkspaceInfo {
	var ws WorkspaceInfo
	ws.Name = props.Name
	ws.ProjectId = props.ProjectId()

	// TODO: the order of SDK records is undetermined, we need the latest SDK revision
	// if there are multiple revisions
	for _, val := range props.Content() {
		ws.Content = append(ws.Content, &SdkInfo{val.Name, val.Channel, strconv.FormatInt(val.Revision, 10)})
	}

	for _, err := range props.Errors() {
		ws.Notes = append(ws.Notes, err.String())
	}

	ws.State = props.State().String()

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
	wstate := query.Get("state")
	if wstate == "" {
		wstate = "all"
	}

	wrkmgr := c.d.overlord.WorkspaceManager()
	files, workspaces, err := wrkmgr.Workspaces(r.Context(), projectId)
	if err != nil {
		return statusInternalError("cannot list workspaces: %v", err)
	}

	var infoLst = make([]*WorkspaceInfo, 0)
	for _, w := range workspaces {
		if wstate != "all" && strings.ToLower(w.State().String()) != wstate {
			continue
		}
		info := workspacePropsToInfo(w)
		infoLst = append(infoLst, info)
	}

	// Now, if the client wants only workspace files or just queried everything
	// available, we add workspace files to the response (note these only exist
	// as files, not instances)
	if wstate == "all" || wstate == "off" {
		for _, file := range files {
			info := workspaceFileToInfo(file, projectId)
			infoLst = append(infoLst, info)
		}
	}

	return SyncResponse(infoLst, http.StatusOK)
}

func v1PostProjectWorkspace(c *Command, r *http.Request, _ *userState) Response {
	projectId := muxVars(r)["id"]
	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()
	wsmgr := c.d.overlord.WorkspaceManager()

	type actionOpts struct {
		Mode string `json:"refresh-mode"`
	}

	var reqData struct {
		Names   []string   `json:"names"`
		Action  string     `json:"action"`
		Options actionOpts `json:"options"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		return statusBadRequest("cannot decode data from request body: %v", err)
	}

	if len(reqData.Names) == 0 {
		return statusBadRequest("at least one workspace name must be provided")
	}

	user, ok := r.Context().Value(workspacebackend.ContextUser).(string)
	if !ok {
		return statusBadRequest("user is not known")
	}

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

		taskset, err := wsmgr.LaunchMany(r.Context(), reqData.Names, projectId)
		if err != nil {
			return statusBadRequest(err.Error())
		}

		for _, i := range taskset {
			change.AddAll(i)
		}
	case "refresh":
		refreshMode := statecontext.ParseRefreshMode(reqData.Options.Mode)

		var summary string
		switch len(reqData.Names) {
		case 1:
			summary = fmt.Sprintf("Refresh workspace %q", reqData.Names[0])
		default:
			summary = fmt.Sprintf("Refresh workspaces %s", strutil.Quoted(reqData.Names))
		}

		if len(reqData.Names) > 1 && refreshMode != statecontext.RefreshTransactional {
			return statusBadRequest("wait-on-error is not supported for multiple workspaces")
		}

		if refreshMode == statecontext.RefreshTransactional || refreshMode == statecontext.RefreshWaitOnError {
			taskset, err := wsmgr.RefreshMany(r.Context(), reqData.Names, projectId)
			if err != nil {
				return statusBadRequest(err.Error())
			}

			change = st.NewChange("refresh", summary)
			for _, i := range taskset {
				change.AddAll(i)
			}

			for _, name := range reqData.Names {
				statecontext.StartRefresh(st, name, projectId, change.ID(),
					refreshMode == statecontext.RefreshWaitOnError)
			}
		}

		if refreshMode == statecontext.RefreshContinue || refreshMode == statecontext.RefreshAbort {
			var err error
			change, err = statecontext.ResumeRefresh(st, reqData.Names[0], projectId, refreshMode)
			if err != nil {
				return statusBadRequest(err.Error())
			}
		}

	default:
		return statusBadRequest("unknown action")
	}

	change.Set("user", user)
	change.Set("project-id", projectId)

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

	wrkmgr := c.d.overlord.WorkspaceManager()

	workspace, err := wrkmgr.Workspace(r.Context(), name, projectId)
	if err != nil {
		return statusNotFound("cannot load workspace: %v", err)
	}

	return SyncResponse(workspacePropsToInfo(workspace), http.StatusOK)
}
