package daemon

import (
	"cmp"
	"net/http"
	"slices"
	"time"

	"github.com/canonical/workshop/internal/sdk"
)

type sdkEntry struct {
	Name        string     `json:"name"`
	Version     string     `json:"version,omitempty"`
	Revision    string     `json:"revision"`
	Summary     string     `json:"summary,omitempty"`
	Description string     `json:"description,omitempty"`
	BuildTime   *time.Time `json:"build-time,omitempty"`
	Size        uint64     `json:"size,omitempty"`
}

func v1GetSdks(c *Command, r *http.Request, _ *userState) Response {
	state := c.d.overlord.State()
	state.Lock()
	wb := c.d.overlord.WorkshopBackend()
	state.Unlock()

	volumes, err := wb.Volumes(r.Context(), "sdk")
	if err != nil {
		return statusInternalError("cannot list SDK volumes: %w", err)
	}

	entries := make([]sdkEntry, 0, len(volumes))
	for _, vol := range volumes {
		if sdk.IsSystem(vol.Sdk) {
			continue
		}

		info, err := sdk.ReadSdkInfo([]byte(vol.Metadata), "", "")
		if err != nil {
			return statusInternalError("cannot parse SDK metadata for %q: %w", vol.Name, err)
		}

		entries = append(entries, sdkEntry{
			Name:        vol.Sdk,
			Version:     info.Version,
			Revision:    vol.Revision.String(),
			Summary:     info.Summary,
			Description: info.Description,
			BuildTime:   info.BuildTime,
			Size:        vol.Size,
		})
	}

	slices.SortFunc(entries, func(a, b sdkEntry) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return SyncResponse(entries, http.StatusOK)
}
