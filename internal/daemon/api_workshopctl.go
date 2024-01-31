package daemon

import (
	"encoding/json"
	"net/http"
)

func v1PostWorkshopCtl(c *Command, r *http.Request, _ *userState) Response {
	state := c.d.overlord.State()
	state.Lock()
	defer state.Unlock()

	var reqData struct {
		Context string   `json:"context"`
		Args    []string `json:"args"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		return statusBadRequest("cannot decode data from request body: %v", err)
	}

	var stdout, stderr []byte

	result := map[string]string{
		"stdout": string(stdout),
		"stderr": string(stderr),
	}

	return SyncResponse(result, http.StatusOK)
}
