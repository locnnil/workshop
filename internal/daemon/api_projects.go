package daemon

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/canonical/workshop/internal/overlord/healthstate"
	"github.com/canonical/workshop/internal/workshopbackend"
)

type HealthCheckInfo struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
	Code      string    `json:"code,omitempty"`
}

type SdkInfo struct {
	Name        string           `json:"name"`
	Channel     string           `json:"channel"`
	Revision    string           `json:"revision"`
	InstallTime time.Time        `json:"install-time"`
	Health      *HealthCheckInfo `json:"health-check,omitempty"`
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

func workshopFileToInfo(file *workshopbackend.WorkshopFile, pid string) *WorkshopInfo {
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

func workshopPropsToInfo(props *workshopbackend.Workshop, health healthstate.HealthState) *WorkshopInfo {
	var info WorkshopInfo
	info.Name = props.Name
	info.ProjectId = props.Project().ProjectId
	info.Base = props.Base()

	for _, val := range props.Content() {
		var healthInfo *HealthCheckInfo
		if sdkHealth, ok := health.SdkHealth[val.Name]; ok {
			healthInfo = &HealthCheckInfo{
				Timestamp: sdkHealth.Timestamp,
				Message:   sdkHealth.Message,
				Code:      sdkHealth.Code,
			}
		}

		info.Content = append(info.Content, &SdkInfo{
			Name:        val.Name,
			Channel:     val.Channel,
			Revision:    strconv.FormatInt(val.Revision, 10),
			InstallTime: val.InstallTime,
			Health:      healthInfo,
		})
	}

	if len(health.Code) > 0 {
		info.Notes = append(info.Notes, health.Code)
	}
	info.Status = health.Status.String()
	return &info
}

func v1GetProjects(c *Command, r *http.Request, _ *userState) Response {
	st := c.d.overlord.State()
	st.Lock()
	defer st.Unlock()

	projects, err := c.d.overlord.WorkshopBackend().Projects(r.Context())
	if err != nil {
		return statusInternalError("cannot get projects list: %v", err)
	}

	result := make([]*workshopbackend.Project, 0)
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
	if err != nil && !errors.Is(err, workshopbackend.ErrNotAProject) {
		return statusInternalError("cannot create or load project at %q: %v", reqData.Path, err)
	} else if errors.Is(err, workshopbackend.ErrNotAProject) {
		return statusBadRequest("%v", err)
	}

	if created {
		return SyncResponse(prj, http.StatusCreated)
	} else {
		return SyncResponse(prj, http.StatusOK)
	}
}
