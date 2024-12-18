package daemon

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/canonical/workshop/internal/workshop"
)

func v1GetProjects(c *Command, r *http.Request, _ *userState) Response {
	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()

	projects, err := c.d.overlord.WorkshopBackend().Projects(r.Context())
	if err != nil {
		return statusInternalError("cannot get projects list: %v", err)
	}

	result := make([]workshop.Project, 0)
	for _, val := range projects {
		result = append(result, val...)
	}

	return SyncResponse(result, http.StatusOK)
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

	wBackend := c.d.overlord.WorkshopBackend()

	prj, created, err := wBackend.CreateOrLoadProject(r.Context(), reqData.Path)
	if err != nil && !errors.Is(err, workshop.ErrNotProject) {
		return statusInternalError("cannot create or load project at %q: %v", reqData.Path, err)
	} else if errors.Is(err, workshop.ErrNotProject) {
		return statusBadRequest("%v", err)
	}

	if created {
		return SyncResponse(prj, http.StatusCreated)
	} else {
		return SyncResponse(prj, http.StatusOK)
	}
}
