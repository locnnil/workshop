package daemon

import (
	"encoding/json"
	"net/http"
)

// workshopCtlOptions holds the various options with which workshopctl is invoked.
type workshopCtlOptions struct {
	// ContextID is a string used to determine the context of this call (e.g.
	// which context and handler should be used, etc.)
	ContextID string `json:"context-id"`

	// Args contains a list of parameters to use for this invocation.
	Args []string `json:"args"`
}

// workshopCtlPostData is the data posted to the daemon /v2/workshopctl endpoint
// TODO: this can be removed once we no longer need to pass stdin data
// but instead use a real stdin stream
type workshopCtlPostData struct {
	workshopCtlOptions

	Stdin []byte `json:"stdin,omitempty"`
}

type snapctlOutput struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

func v1PostWorkshopCtl(c *Command, r *http.Request, _ *userState) Response {
	var reqData workshopCtlPostData

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		return statusBadRequest("cannot decode data from request body: %v", err)
	}

	// Ignore missing context error to allow 'workshopctl -h' without a context;
	// Actual context is validated later by get/set.
	context, err := c.d.overlord.HookManager().Context(reqData.ContextID)
	if err != nil {
		return statusNotFound(err.Error())
	}

	if reqData.Stdin != nil {
		context.Lock()
		context.Set("stdin", reqData.Stdin)
		context.Unlock()
	}

	var stdout, stderr []byte

	result := snapctlOutput{
		Stdout: string(stdout),
		Stderr: string(stderr),
	}

	return SyncResponse(result, http.StatusOK)
}
