package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/overlord/statecontext"
	"github.com/canonical/workspace/internal/workspacebackend"
	"github.com/canonical/x-go/strutil"
	"golang.org/x/exp/maps"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type SdkInfo struct {
	Name        string    `json:"name"`
	Channel     string    `json:"channel"`
	Revision    string    `json:"revision"`
	InstallTime time.Time `json:"install-time"`
}

type WorkspaceInfo struct {
	Name      string     `json:"name"`
	Base      string     `json:"base"`
	ProjectId string     `json:"project-id"`
	State     string     `json:"state"`
	Content   []*SdkInfo `json:"content,omitempty"`
	Notes     []string   `json:"notes,omitempty"`
}

var ensureStateSoon = stateEnsureBefore

func workspaceFileToInfo(file *workspacebackend.WorkspaceFile, pid string) *WorkspaceInfo {
	var ws WorkspaceInfo
	ws.Name = file.Name
	ws.Base = file.Base
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
	ws.Base = props.Base()

	for _, val := range props.Content() {
		ws.Content = append(ws.Content, &SdkInfo{
			Name:        val.Name,
			Channel:     val.Channel,
			Revision:    strconv.FormatInt(val.Revision, 10),
			InstallTime: val.InstallTime})
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
		return statusBadRequest("cannot %s: failed to decode data from request body: %v", err)
	}

	if len(reqData.Names) == 0 {
		return statusBadRequest("cannot %s: at least one workspace name must be provided", reqData.Action)
	}

	user, ok := r.Context().Value(workspacebackend.ContextUser).(string)
	if !ok {
		return statusBadRequest("cannot %s: user is not known", reqData.Action)
	}

	var summary string
	switch len(reqData.Names) {
	case 1:
		summary = fmt.Sprintf("%s %q workspace", cases.Title(language.BritishEnglish).String(reqData.Action), reqData.Names[0])
	default:
		summary = fmt.Sprintf("%s %s workspaces", cases.Title(language.BritishEnglish).String(reqData.Action), strutil.Quoted(reqData.Names))
	}

	var change *state.Change
	switch reqData.Action {
	case "launch":
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

		if len(reqData.Names) > 1 && refreshMode != statecontext.RefreshTransactional {
			return statusBadRequest("wait-on-error is not supported for multiple workspaces")
		}

		if refreshMode == statecontext.RefreshTransactional || refreshMode == statecontext.RefreshWaitOnError {
			// check if all the workspace are available for the operation
			avail, ops, err := wsmgr.CheckStatus(r.Context(), reqData.Names, projectId,
				func(status workspacebackend.WorkspaceState) bool { return status == workspacebackend.WorkspaceReady })
			if err != nil {
				return statusBadRequest("cannot %s: %v", reqData.Action, err)
			}

			if !avail {
				return statusConflict("cannot %s: operation is already in progress for %s", reqData.Action, strutil.Quoted(ops))
			}

			taskset, err := wsmgr.RefreshMany(r.Context(), reqData.Names, projectId)
			if err != nil {
				return statusBadRequest(err.Error())
			}

			change = st.NewChange("refresh", summary)
			for _, i := range taskset {
				change.AddAll(i)
			}

			for _, name := range reqData.Names {
				statecontext.StartOperation(st, name, projectId,
					statecontext.Operation{ChangeId: change.ID(), Operation: statecontext.OperationRefresh, WaitOnError: refreshMode == statecontext.RefreshWaitOnError})
			}
		}

		if refreshMode == statecontext.RefreshContinue || refreshMode == statecontext.RefreshAbort {
			var err error
			change, err = statecontext.ResumeRefresh(st, reqData.Names[0], projectId, refreshMode)
			if err != nil {
				return statusBadRequest(err.Error())
			}
		}
	case "start":
		// check if all the workspaces are stopped
		avail, ops, err := wsmgr.CheckStatus(r.Context(), reqData.Names, projectId,
			func(status workspacebackend.WorkspaceState) bool { return status == workspacebackend.WorkspaceStopped })
		if err != nil {
			return statusBadRequest("cannot %s: %v", reqData.Action, err)
		}

		if !avail {
			return statusConflict("cannot %s: %s must be stopped", reqData.Action, strutil.Quoted(ops))
		}

		taskset, err := wsmgr.StartMany(r.Context(), reqData.Names, projectId)
		if err != nil {
			return statusBadRequest(err.Error())
		}

		change = st.NewChange("start", summary)
		change.AddAll(taskset)

		for _, name := range reqData.Names {
			statecontext.StartOperation(st, name, projectId,
				statecontext.Operation{ChangeId: change.ID(), Operation: statecontext.OperationStart})
		}
	case "stop":
		// check if all the workspaces are started or stopped
		avail, ops, err := wsmgr.CheckStatus(r.Context(), reqData.Names, projectId,
			func(status workspacebackend.WorkspaceState) bool {
				return status == workspacebackend.WorkspaceStopped || status == workspacebackend.WorkspaceReady
			})
		if err != nil {
			return statusBadRequest("cannot %s: %v", reqData.Action, err)
		}

		if !avail {
			return statusConflict("cannot %s: %s must be ready", reqData.Action, strutil.Quoted(ops))
		}

		taskset, err := wsmgr.StopMany(r.Context(), reqData.Names, projectId)
		if err != nil {
			return statusBadRequest(err.Error())
		}

		change = st.NewChange("stop", summary)
		change.AddAll(taskset)

		for _, name := range reqData.Names {
			statecontext.StartOperation(st, name, projectId,
				statecontext.Operation{ChangeId: change.ID(), Operation: statecontext.OperationStop})
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
