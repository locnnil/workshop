package daemon

import (
	"cmp"
	"net/http"
	"slices"

	"github.com/canonical/workshop/internal/overlord/sdkstate"
)

func v1GetSdks(c *Command, r *http.Request, _ *userState) Response {
	mgr := c.d.overlord.SdkManager()

	sdks, err := mgr.SdkVolumes(r.Context())
	if err != nil {
		return statusInternalError("cannot list SDK volumes: %w", err)
	}

	slices.SortFunc(sdks, func(a, b sdkstate.SdkVolume) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return SyncResponse(sdks, http.StatusOK)
}

func v1GetSdkInfo(c *Command, r *http.Request, _ *userState) Response {
	name := muxVars(r)["name"]
	if name == "" {
		return statusBadRequest("sdk name required")
	}
	mgr := c.d.overlord.SdkManager()

	sk, err := mgr.Sdk(r.Context(), name)
	if err != nil {
		return statusInternalError("%w", err)
	}

	slices.SortFunc(sk.Installed, func(a, b sdkstate.SdkInstalled) int {
		if a.Workshop != b.Workshop {
			return cmp.Compare(a.Workshop, b.Workshop)
		}
		return cmp.Compare(a.ProjectPath, b.ProjectPath)
	})

	return SyncResponse(sk, http.StatusOK)
}
