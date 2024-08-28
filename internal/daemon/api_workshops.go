package daemon

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/canonical/x-go/strutil"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/logger"
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
var workshopMounts = mounts

func workshopFileToInfo(file string, pid string) *WorkshopInfo {
	var ws WorkshopInfo
	ws.Name = file
	ws.ProjectId = pid
	ws.Status = healthstate.OffStatus.String()
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

func mounts(ctx context.Context, w *workshop.Workshop) (map[string][]*Mount, error) {
	var mnts = map[string][]*Mount{}

	content, err := w.ContentInfo(ctx)
	if err != nil {
		return mnts, err
	}

	masters := map[interfaces.PlugRef][]interfaces.PlugRef{}
	for _, sk := range content {
		for s, m := range sk.PlugBinds {
			ref := interfaces.PlugRef{ProjectId: w.Project.ProjectId, Workshop: w.Name, Sdk: m.Sdk, Name: m.Name}
			sref := interfaces.PlugRef{ProjectId: w.Project.ProjectId, Workshop: w.Name, Sdk: sk.Name, Name: s}
			masters[ref] = append(masters[ref], sref)
		}
	}

	for _, sk := range content {
		prof, err := w.Backend.Profile(ctx, w.Name, sk.Name)
		if err != nil && !errors.Is(err, workshop.ErrSdkProfileNotFound) {
			logger.Noticef("Failed to obtain mounts for %s/%s: %v", w.Name, sk.Name, err)
			return mnts, err
		}
		if errors.Is(err, workshop.ErrSdkProfileNotFound) {
			continue
		}
		for n, dev := range prof.Devices {
			if dev.Type == workshop.BindMount {
				pref := interfaces.PlugRef{ProjectId: w.Project.ProjectId, Workshop: w.Name, Sdk: sk.Name, Name: n}
				mnt := &Mount{
					Plug:   pref,
					Source: dev.Properties["source"],
					Target: dev.Properties["path"],
				}
				mnts[sk.Name] = append(mnts[sk.Name], mnt)
				if slaves, ok := masters[pref]; ok {
					for _, slave := range slaves {
						mnt := &Mount{
							Plug:   slave,
							Source: dev.Properties["source"],
							Target: dev.Properties["path"],
						}
						mnts[slave.Sdk] = append(mnts[slave.Sdk], mnt)
					}
				}
			}
		}
	}

	return mnts, nil
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

	ctx := context.WithValue(r.Context(), workshop.ContextProjectId, projectId)
	ms, err := workshopMounts(ctx, w)
	if err != nil {
		return statusBadRequest(err.Error())
	}

	return SyncResponse(workshopToInfo(w, health, ms), http.StatusOK)
}
