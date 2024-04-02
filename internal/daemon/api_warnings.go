package daemon

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/canonical/workshop/internal/overlord/state"
)

var (
	stateOkayWarnings    = (*state.State).OkayWarnings
	stateAllWarnings     = (*state.State).AllWarnings
	statePendingWarnings = (*state.State).PendingWarnings
)

func v1GetWarnings(c *Command, r *http.Request, _ *userState) Response {
	query := r.URL.Query()
	var all bool
	sel := query.Get("select")
	switch sel {
	case "all":
		all = true
	case "pending", "":
		all = false
	default:
		return statusBadRequest("invalid select parameter: %q", sel)
	}

	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()

	var ws []*state.Warning
	if all {
		ws = stateAllWarnings(st)
	} else {
		ws, _ = statePendingWarnings(st)
	}
	if len(ws) == 0 {
		// no need to confuse the issue
		return SyncResponse([]state.Warning{}, http.StatusOK)
	}

	return SyncResponse(ws, http.StatusOK)
}

func v1PostWarnings(c *Command, r *http.Request, _ *userState) Response {
	defer r.Body.Close()
	var op struct {
		Action    string    `json:"action"`
		Timestamp time.Time `json:"timestamp"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&op); err != nil {
		return statusBadRequest("cannot decode request body into warnings operation: %v", err)
	}
	if op.Action != "okay" {
		return statusBadRequest("unknown warning action %q", op.Action)
	}
	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()
	n := stateOkayWarnings(st, op.Timestamp)

	return SyncResponse(n, http.StatusOK)
}
