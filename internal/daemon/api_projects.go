package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/canonical/workspace/internal/overlord/sdkstate"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/overlord/workspacestate"
	"github.com/canonical/workspace/internal/workspacebackend"
	"github.com/spf13/afero"
	"golang.org/x/exp/maps"
)

type SdkInfo struct {
	Name     string `json:"name"`
	Channel  string `json:"channel"`
	Revision string `json:"revision"`
}

type WorkspaceInfo struct {
	Name      string     `json:"name"`
	ProjectId string     `json:"project-id"`
	State     string     `json:"state"`
	Content   []*SdkInfo `json:"content,omitempty"`
	Notes     []string   `json:"notes,omitempty"`
}

var ensureStateSoon = stateEnsureBefore

func workspacePropsToInfo(props *workspacebackend.WorkspaceProps) *WorkspaceInfo {
	var ws WorkspaceInfo
	ws.Name = props.Name
	ws.ProjectId = props.Config[workspacebackend.ProjectIdConfig]
	ws.State = props.State().String()

	var sequence = make(map[string][]*sdkstate.SdkSequenceRecord, 0)
	if sdks, ok := props.Config["user.workspace.sdk"]; ok {
		err := json.Unmarshal([]byte(sdks), &sequence)
		if err != nil {
			ws.State = workspacebackend.Error.String()
			return &ws
		}
		// TODO: the order of SDK records is undetermined, we need the latest one
		for i, val := range sequence {
			ws.Content = append(ws.Content, &SdkInfo{i, val[0].Channel, strconv.FormatInt(val[0].Revision, 10)})
		}
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

func v1GetProject(c *Command, r *http.Request, _ *userState) Response {
	projectId := muxVars(r)["id"]
	state := c.d.overlord.State()
	state.Lock()
	defer state.Unlock()

	return SyncResponse([]string{projectId}, http.StatusOK)
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

	workspaces, err := wBackend.GetAllWorkspaces(ctx)
	if err != nil {
		return statusInternalError("cannot list workspaces: %v", projectId, err)
	}

	var wsInfos = make([]*WorkspaceInfo, 0)
	for _, i := range workspaces {
		if wsState != "all" {
			if strings.ToLower(i.State().String()) != wsState {
				continue
			}
		}
		wsInfos = append(wsInfos, workspacePropsToInfo(i))
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
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		return statusBadRequest("cannot decode data from request body: %v", err)
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

	var change *state.Change
	switch reqData.Action {
	case "launch":
		change = st.NewChange("launch", fmt.Sprintf("Launch workspace(s): %s", strings.Join(reqData.Names, ",")))

		for _, i := range reqData.Names {
			file, err := workspacebackend.ReadWorkspace(afero.NewOsFs(), workspacebackend.WorkspaceFilePath(prj.Path, i))
			if err != nil {
				return statusInternalError("cannot read workspace \"%s\": %v", i, err)
			}

			taskset, err := workspacestate.Launch(st, file, prj)
			if err != nil {
				return statusBadRequest("cannot launch workspace \"%s\": %v", i, err)
			}
			change.AddAll(taskset)
			change.Set("user", user)
			change.Set("project-key", &prj)
		}
	default:
		return statusBadRequest("unknown action")
	}

	ensureStateSoon(st, 0)

	return AsyncResponse(nil, change.ID())

}
