package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshopbackend"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type mountRequest struct {
	Action string             `json:"action"`
	Plug   interfaces.PlugRef `json:"plug"`
	Source string             `json:"source"`
}

func newMountChange(st *state.State, user string, reqData *mountRequest) *state.Change {
	summary := fmt.Sprintf("%s %q", cases.Title(language.BritishEnglish).String(reqData.Action), fmt.Sprintf("%s/%s:%s", reqData.Plug.Workshop, reqData.Plug.Sdk, reqData.Plug.Name))

	change := st.NewChange(reqData.Action, summary)
	change.Set("user", user)
	change.Set("project-id", reqData.Plug.ProjectId)
	return change
}

func v1PostWorkshopMount(c *Command, r *http.Request, _ *userState) Response {
	projectId := muxVars(r)["id"]
	workshop := muxVars(r)["name"]

	if projectId == "" {
		return statusBadRequest("project-id must be provided")
	}

	if workshop == "" {
		return statusBadRequest("workshop name must be provided")
	}

	user, ok := r.Context().Value(workshopbackend.ContextUser).(string)
	if !ok {
		return statusBadRequest("internal error: no user associated with the request")
	}

	st := c.d.state
	st.Lock()
	defer st.Unlock()

	o := c.d.overlord

	var reqData mountRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		return statusBadRequest("cannot decode data from request body: %v", err)
	}
	if reqData.Action != "remount" {
		return statusBadRequest("unknown action %q", reqData.Action)
	}
	reqData.Plug.Workshop = workshop
	reqData.Plug.ProjectId = projectId

	change := newMountChange(st, user, &reqData)
	defer func() {
		if len(change.Tasks()) == 0 {
			change.SetStatus(state.DoneStatus)
		}
	}()

	conn, err := o.InterfaceManager().Repository().Connected(reqData.Plug.ProjectId, reqData.Plug.Workshop, reqData.Plug.Sdk, reqData.Plug.Name)
	if err != nil {
		return statusBadRequest(err.Error())
	}

	if len(conn) == 0 {
		return statusBadRequest(`"%s/%s:%s" must be connected for remount`, reqData.Plug.Workshop, reqData.Plug.Sdk, reqData.Plug.Name)
	}

	taskset, err := o.WorkshopManager().Remount(r.Context(), st, reqData.Plug, reqData.Source, projectId)
	if err != nil {
		return statusBadRequest(err.Error())
	}

	change.AddAll(taskset)

	return AsyncResponse(nil, change.ID())
}
