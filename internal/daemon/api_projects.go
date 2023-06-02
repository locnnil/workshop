package daemon

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/canonical/workspace/internal/project"

	"github.com/spf13/afero"
)

func v1GetProjects(c *Command, r *http.Request, _ *userState) Response {
	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()

	// In this scenario, we will have go walk all projects in the system
	// and also make sure these are up-to-date, this is what RetrieveWorkspacesGlobal does
	// and returns a list of workspaces for every project found in the system
	projects, err := project.RetrieveAllProjects(r.Context(), c.d.overlord.WorkspaceBackend(), afero.NewOsFs())
	if err != nil {
		return statusInternalError("cannot get projects list: %v", err)
	}

	return SyncResponse(projects, http.StatusOK)
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

	prj, err := project.RetrieveProject(r.Context(), c.d.overlord.WorkspaceBackend(), afero.NewOsFs(), reqData.Path)
	if err != nil && !errors.Is(err, project.ErrProjectNotFound) {
		return statusBadRequest("cannot load project: %v", err)
	} else if err == nil {
		return SyncResponse(prj, http.StatusOK)
	}

	if errors.Is(err, project.ErrProjectNotFound) {
		// create a new project as the end user expects CreateOrLoad behaviour from this endpoint
		prj, err = project.NewProject(afero.NewOsFs(), reqData.Path)
		if err != nil {
			return statusInternalError("cannot create project: %v", err)
		}
	}

	return SyncResponse(prj, http.StatusCreated)
}

func v1GetProjectWorkspace(c *Command, r *http.Request, _ *userState) Response {
	return SyncResponse([]string{}, http.StatusOK)
}
