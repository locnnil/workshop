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

func v1FindSdks(c *Command, r *http.Request, _ *userState) Response {
	mgr := c.d.overlord.SdkManager()
	query := r.URL.Query()
	q := query.Get("q")

	sdks, err := mgr.FindSdks(r.Context(), q)
	if err != nil {
		if q == "" {
			return statusInternalError("cannot find SDKs: %w", err)
		}
		return statusInternalError("cannot find SDKs matching %q: %w", q, err)
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
