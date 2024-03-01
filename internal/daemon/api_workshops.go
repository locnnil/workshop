package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/canonical/workshop/internal/overlord/conflict"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/workshopstate"
	"github.com/canonical/workshop/internal/workshopbackend"
	"github.com/canonical/x-go/strutil"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func v1GetProjectWorkshops(c *Command, r *http.Request, _ *userState) Response {
	projectId := muxVars(r)["id"]
	state := c.d.overlord.State()
	state.Lock()
	defer state.Unlock()

	query := r.URL.Query()
	wstate := query.Get("state")
	if wstate == "" {
		wstate = "all"
	}

	wrkmgr := c.d.overlord.WorkshopManager()
	files, workshops, err := wrkmgr.Workshops(r.Context(), projectId)
	if err != nil {
		return statusInternalError("cannot list workshops: %v", err)
	}

	var infoLst = make([]*WorkshopInfo, 0)
	for _, w := range workshops {
		health := wrkmgr.WorkshopHealth(w)
		if wstate != "all" && strings.ToLower(health.Status.String()) != wstate {
			continue
		}
		info := workshopPropsToInfo(w, health)
		infoLst = append(infoLst, info)
	}

	// Now, if the client wants only workshop files or just queried everything
	// available, we add workshop files to the response (note these only exist
	// as files, not instances)
	if wstate == "all" || wstate == "off" {
		for _, file := range files {
			info := workshopFileToInfo(file, projectId)
			infoLst = append(infoLst, info)
		}
	}

	return SyncResponse(infoLst, http.StatusOK)
}

func v1PostProjectWorkshop(c *Command, r *http.Request, _ *userState) Response {
	projectId := muxVars(r)["id"]
	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()
	wsmgr := c.d.overlord.WorkshopManager()

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
		return statusBadRequest("cannot %s: at least one workshop name must be provided", reqData.Action)
	}

	reqData.Names = strutil.Deduplicate(reqData.Names)

	user, ok := r.Context().Value(workshopbackend.ContextUser).(string)
	if !ok {
		return statusBadRequest("cannot %s: user is not known", reqData.Action)
	}

	var summary string
	switch len(reqData.Names) {
	case 1:
		summary = fmt.Sprintf("%s %q workshop", cases.Title(language.BritishEnglish).String(reqData.Action), reqData.Names[0])
	default:
		summary = fmt.Sprintf("%s %s workshops", cases.Title(language.BritishEnglish).String(reqData.Action), strutil.Quoted(reqData.Names))
	}

	var change *state.Change
	newChange := func(kind, summary string) *state.Change {
		change := st.NewChange(kind, summary)
		change.Set("user", user)
		change.Set("project-id", projectId)
		return change
	}

	defer func() {
		if change != nil && len(change.Tasks()) == 0 {
			change.SetStatus(state.DoneStatus)
		}
	}()

	switch reqData.Action {
	case "launch":
		change = newChange("launch", summary)
		taskset, err := wsmgr.LaunchMany(r.Context(), reqData.Names, projectId, change.ID())
		if err != nil {
			return statusBadRequest(err.Error())
		}

		for _, tset := range taskset {
			change.AddAll(tset)
		}
	case "refresh":
		refreshMode, err := conflict.ParseRefreshMode(reqData.Options.Mode)
		if err != nil {
			return statusBadRequest("cannot refresh: %v", err)
		}

		if len(reqData.Names) > 1 && refreshMode != conflict.RefreshTransactional {
			return statusBadRequest("wait-on-error is not supported for multiple workshops")
		}

		if refreshMode == conflict.RefreshTransactional || refreshMode == conflict.RefreshWaitOnError {
			change = newChange("refresh", summary)
			taskset, err := wsmgr.RefreshMany(r.Context(), reqData.Names, projectId, refreshMode, change.ID())
			if err != nil {
				return statusBadRequest(err.Error())
			}
			for _, tset := range taskset {
				change.AddAll(tset)
			}
		}

		if refreshMode == conflict.RefreshContinue || refreshMode == conflict.RefreshAbort {
			var err error
			change, err = conflict.ResumeRefresh(st, reqData.Names[0], projectId, refreshMode)
			if err != nil {
				return statusBadRequest(err.Error())
			}
		}
	case "start":
		change = newChange("start", summary)
		taskset, err := wsmgr.StartMany(r.Context(), reqData.Names, projectId, change.ID())
		if err != nil {
			return statusBadRequest(err.Error())
		}

		change.AddAll(taskset)
	case "stop":
		change = newChange("stop", summary)
		taskset, err := wsmgr.StopMany(r.Context(), reqData.Names, projectId, change.ID())
		if err != nil {
			return statusBadRequest(err.Error())
		}

		change.AddAll(taskset)
	case "remove":
		change = newChange("remove", summary)
		taskset, err := wsmgr.RemoveMany(r.Context(), reqData.Names, projectId, change.ID())
		if err != nil {
			return statusBadRequest(err.Error())
		}
		change.AddAll(taskset)
	default:
		return statusBadRequest("unknown action")
	}

	ensureStateSoon(st, 0)

	return AsyncResponse(nil, change.ID())
}

var workshopHealth = (*workshopstate.WorkshopManager).WorkshopHealth

func v1GetProjectWorkshop(c *Command, r *http.Request, _ *userState) Response {
	projectId := muxVars(r)["id"]
	name := muxVars(r)["name"]

	if projectId == "" {
		return statusBadRequest("project-id must be provided")
	}

	if name == "" {
		return statusBadRequest("workshop name must be provided")
	}

	state := c.d.overlord.State()
	state.Lock()
	defer state.Unlock()

	wrkmgr := c.d.overlord.WorkshopManager()
	workshop, err := wrkmgr.Workshop(r.Context(), name, projectId)
	if err != nil {
		return statusNotFound("%v", err)
	}
	health := workshopHealth(wrkmgr, workshop)
	return SyncResponse(workshopPropsToInfo(workshop, health), http.StatusOK)
}
