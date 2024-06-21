package daemon

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/canonical/x-go/strutil"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/overlord/conflict"
	"github.com/canonical/workshop/internal/overlord/healthstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/workshopstate"
	"github.com/canonical/workshop/internal/workshop"
)

type actionOpts struct {
	Mode string `json:"refresh-mode"`
}

type workshopReq struct {
	Names   []string   `json:"names"`
	Action  string     `json:"action"`
	Options actionOpts `json:"options"`
}

type HealthCheckInfo struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
	Code      string    `json:"code,omitempty"`
}

type SdkInfo struct {
	Name        string           `json:"name"`
	Channel     string           `json:"channel"`
	Revision    string           `json:"revision"`
	InstallTime *time.Time       `json:"install-time,omitempty"`
	Health      *HealthCheckInfo `json:"health-check,omitempty"`
	Mounts      []*Mount         `json:"mounts,omitempty"`
}

type Mount struct {
	Plug   interfaces.PlugRef `json:"plug"`
	Source string             `json:"source"`
	Target string             `json:"target"`
}

type WorkshopInfo struct {
	Name      string     `json:"name"`
	Base      string     `json:"base"`
	ProjectId string     `json:"project-id"`
	Status    string     `json:"status"`
	Content   []*SdkInfo `json:"content,omitempty"`
	Notes     []string   `json:"notes,omitempty"`
}

var ensureStateSoon = stateEnsureBefore
var sdkMounts = sdkConnsToMounts

func workshopFileToInfo(file *workshop.File, pid string) *WorkshopInfo {
	var ws WorkshopInfo
	ws.Name = file.Name
	ws.Base = file.Base
	ws.ProjectId = pid
	ws.Status = healthstate.OffStatus.String()
	for _, i := range file.Sdks {
		ws.Content = append(ws.Content, &SdkInfo{
			Name:    i.Name,
			Channel: i.Channel,
		})
	}
	return &ws
}

func workshopToInfo(w *workshop.Workshop, health healthstate.HealthState, mounts map[string][]*Mount) *WorkshopInfo {
	var info WorkshopInfo
	info.Name = w.Name
	info.ProjectId = w.Project.ProjectId
	info.Base = w.Base

	for _, sdk := range w.Content {
		var healthInfo *HealthCheckInfo
		if sdkHealth, ok := health.SdkHealth[sdk.Name]; ok {
			healthInfo = &HealthCheckInfo{
				Timestamp: sdkHealth.Timestamp,
				Message:   sdkHealth.Message,
				Code:      sdkHealth.Code,
			}
		}

		var sdkMounts []*Mount
		if mounts != nil {
			sdkMounts = mounts[sdk.Name]
		}

		info.Content = append(info.Content, &SdkInfo{
			Name:        sdk.Name,
			Channel:     sdk.Channel,
			Revision:    strconv.FormatInt(sdk.Revision, 10),
			InstallTime: sdk.InstallTime,
			Health:      healthInfo,
			Mounts:      sdkMounts,
		})
	}

	if len(health.Code) > 0 {
		info.Notes = append(info.Notes, health.Code)
	}
	info.Status = health.Status.String()

	slices.SortFunc(info.Content, func(a, b *SdkInfo) int { return cmp.Compare(a.Name, b.Name) })
	return &info
}

func sdkConnsToMounts(st *state.State, repo *interfaces.Repository, projectId, w, sdk string) []*Mount {
	connections, err := repo.Connections(projectId, w, sdk)
	if err != nil {
		return nil
	}
	var mounts []*Mount
	for _, conn := range connections {
		connection, err := repo.Connection(conn)
		if err != nil {
			continue
		}

		if connection.Interface() == "content" {
			var source, target string
			err = connection.Plug.Attr("source", &source)
			if err != nil {
				continue
			}
			// check if the source exists as otherwise the mount is broken
			if _, err = os.Stat(source); osutil.IsDirNotExist(err) {
				st.Warnf("%s/%s:%s mount is broken: %s does not exist", w, sdk, connection.Plug.Name(), source)
			}

			err = connection.Plug.Attr("target", &target)
			if err != nil {
				continue
			}
			mounts = append(mounts, &Mount{Source: source, Target: target, Plug: conn.PlugRef})
		}
	}
	return mounts
}

func newWorkshopChange(st *state.State, kind string, user, projectId string, reqData *workshopReq) *state.Change {
	var summary string
	switch len(reqData.Names) {
	case 1:
		summary = fmt.Sprintf("%s %q workshop", cases.Title(language.BritishEnglish).String(reqData.Action), reqData.Names[0])
	default:
		summary = fmt.Sprintf("%s %s workshops", cases.Title(language.BritishEnglish).String(reqData.Action), strutil.Quoted(reqData.Names))
	}

	change := st.NewChange(kind, summary)
	change.Set("user", user)
	change.Set("project-id", projectId)
	return change
}

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
		info := workshopToInfo(w, health, nil)
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

	var reqData workshopReq
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqData); err != nil {
		return statusBadRequest("cannot decode data from request body: %v", err)
	}

	if len(reqData.Names) == 0 {
		return statusBadRequest("cannot %s: at least one workshop name must be provided", reqData.Action)
	}

	reqData.Names = strutil.Deduplicate(reqData.Names)

	user, ok := r.Context().Value(workshop.ContextUser).(string)
	if !ok {
		return statusBadRequest("cannot %s: user is not known", reqData.Action)
	}

	var change *state.Change
	var taskset = []*state.TaskSet{}
	var err error

	switch reqData.Action {
	case "launch":
		change = newWorkshopChange(st, "launch", user, projectId, &reqData)
		taskset, err = wsmgr.LaunchMany(r.Context(), reqData.Names, projectId, change.ID())
	case "refresh":
		var refreshMode conflict.RefreshMode
		refreshMode, err = conflict.ParseRefreshMode(reqData.Options.Mode)
		if err != nil {
			return statusBadRequest("cannot refresh: %v", err)
		}

		if len(reqData.Names) > 1 && refreshMode != conflict.RefreshTransactional {
			return statusBadRequest("wait-on-error is not supported for multiple workshops")
		}

		if refreshMode == conflict.RefreshTransactional || refreshMode == conflict.RefreshWaitOnError {
			change = newWorkshopChange(st, "refresh", user, projectId, &reqData)
			taskset, err = wsmgr.RefreshMany(r.Context(), reqData.Names, projectId, refreshMode, change.ID())
		}

		if refreshMode == conflict.RefreshContinue || refreshMode == conflict.RefreshAbort {
			change, err = conflict.ResumeRefresh(st, reqData.Names[0], projectId, refreshMode)
			if err != nil {
				return statusBadRequest(err.Error())
			}
		}
	case "start":
		change = newWorkshopChange(st, "start", user, projectId, &reqData)
		taskset, err = wsmgr.StartMany(r.Context(), reqData.Names, projectId, change.ID())
	case "stop":
		change = newWorkshopChange(st, "stop", user, projectId, &reqData)
		taskset, err = wsmgr.StopMany(r.Context(), reqData.Names, projectId, change.ID())
	case "remove":
		change = newWorkshopChange(st, "remove", user, projectId, &reqData)
		taskset, err = wsmgr.RemoveMany(r.Context(), reqData.Names, projectId, change.ID())
	default:
		return statusBadRequest("unknown action")
	}

	for _, tset := range taskset {
		change.AddAll(tset)
	}
	if len(change.Tasks()) == 0 {
		change.SetStatus(state.DoneStatus)
	}

	if err != nil {
		return statusBadRequest(err.Error())
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
	w, err := wrkmgr.Workshop(r.Context(), name, projectId)
	if err != nil {
		return statusNotFound("%v", err)
	}
	health := workshopHealth(wrkmgr, w)

	repo := c.d.overlord.InterfaceManager().Repository()
	mounts := map[string][]*Mount{}
	for _, sdk := range w.Content {
		mounts[sdk.Name] = sdkMounts(state, repo, projectId, name, sdk.Name)
	}

	return SyncResponse(workshopToInfo(w, health, mounts), http.StatusOK)
}
