package daemon

import (
	"encoding/json"
	"net/http"

	"github.com/canonical/workshop/internal/interfaces"
)

func v1PostWorkshopMount(c *Command, r *http.Request, _ *userState) Response {
	projectId := muxVars(r)["id"]
	workshop := muxVars(r)["name"]

	if projectId == "" {
		return statusBadRequest("project-id must be provided")
	}

	if workshop == "" {
		return statusBadRequest("workshop name must be provided")
	}

	var reqData struct {
		Action string             `json:"action"`
		Plug   interfaces.PlugRef `json:"plug"`
		Source string             `json:"source"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		return statusBadRequest("cannot %s: failed to decode data from request body: %v", err)
	}

	return AsyncResponse(nil, "")
}
