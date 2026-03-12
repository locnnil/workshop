package daemon

import (
	"errors"
	"net/http"

	"github.com/canonical/workshop/internal/sdkstore"
)

func v1GetSdks(c *Command, r *http.Request, _ *userState) Response {
	mgr := c.d.overlord.SdkManager()

	sdks, err := mgr.SdkVolumes(r.Context())
	if err != nil {
		return statusInternalError("cannot list SDK volumes: %w", err)
	}

	return SyncResponse(sdks, http.StatusOK)
}

func v1GetSdkInfo(c *Command, r *http.Request, _ *userState) Response {
	name := muxVars(r)["name"]
	if name == "" {
		return statusBadRequest("sdk name required")
	}
	mgr := c.d.overlord.SdkManager()

	sk, err := mgr.Sdk(r.Context(), name)
	var sdkErr *sdkstore.SdkNotFoundError
	if errors.As(err, &sdkErr) {
		return statusNotFound("%w", sdkErr)
	}
	if err != nil {
		return statusInternalError("%w", err)
	}

	return SyncResponse(sk, http.StatusOK)
}
