package daemon

import (
	"cmp"
	"net/http"
	"slices"

	"github.com/canonical/workshop/internal/sdk"
)

type sdkEntry struct {
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Revision string `json:"revision"`
	Summary  string `json:"summary,omitempty"`
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
			Name:     info.Name,
			Version:  info.Version,
			Revision: vol.Revision.String(),
			Summary:  info.Summary,
		})
	}

	slices.SortFunc(entries, func(a, b sdkEntry) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return SyncResponse(entries, http.StatusOK)
}
