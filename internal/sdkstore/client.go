// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package sdkstore

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/canonical/workshop/internal/https"
	"github.com/canonical/workshop/internal/sdkstore/path"
)

const (
	// DefaultServerURL is the default location of the global SDK Store API.
	// An alternate location can be configured by changing the URL
	// field in the Config struct.
	DefaultServerURL = "https://api.pkg.store"

	// RefreshTimeout is the timeout callers should use for Refresh calls.
	RefreshTimeout = 10 * time.Second
)

const (
	serverVersion = "v2"
	serverEntity  = "sdks"
)

// Config holds configuration for creating a new SDK Store client.
// The zero value is a valid default configuration.
type Config struct {
	// URL holds the base endpoint URL of the SDK Store API.
	// If nil use the default endpoint.
	URL *url.URL

	// HTTPClient represents the HTTP client to use for all API
	// requests. If nil, use the default HTTP client.
	HTTPClient https.HTTPClient
}

func EnvConfig() (Config, error) {
	var storeURL *url.URL
	if custom := os.Getenv("SDK_STORE_URL"); custom != "" {
		var err error
		storeURL, err = url.Parse(custom)
		if err != nil {
			return Config{}, fmt.Errorf("invalid SDK_STORE_URL: %w", err)
		}
	}
	return Config{URL: storeURL}, nil
}

// basePath returns the base configuration path for speaking to the server API.
func basePath(base *url.URL) path.Path {
	return path.MakePath(base).JoinPath(serverVersion, serverEntity)
}

// downloadPath returns the base configuration path for downloading SDKs.
func downloadPath(base *url.URL) path.Path {
	return path.MakePath(base).JoinPath("api", "v1", serverEntity, "download")
}

// resolvePath returns the configuration path for speaking to the resolve API.
func resolvePath(base *url.URL) path.Path {
	return path.MakePath(base).JoinPath(serverVersion, "revisions", "resolve")
}

// Client represents the client side of an SDK store.
type Client struct {
	*downloadClient
	*findClient
	*infoClient
	*resolveClient
}

// NewClient creates a new SDK Store client from the supplied configuration.
func NewClient(config Config) *Client {
	baseURL := config.URL
	if baseURL == nil {
		baseURL, _ = url.Parse(DefaultServerURL)
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = DefaultHTTPClient()
	}

	base := basePath(baseURL)
	downloadPath := downloadPath(baseURL)
	findPath := base.JoinPath("find")
	infoPath := base.JoinPath("info")
	resolvePath := resolvePath(baseURL)

	apiRequester := newAPIRequester(httpClient)
	restClient := newHTTPRESTClient(apiRequester)

	return &Client{
		downloadClient: newDownloadClient(downloadPath, httpClient),
		findClient:     newFindClient(findPath, restClient),
		infoClient:     newInfoClient(infoPath, restClient),
		resolveClient:  newResolveClient(resolvePath, restClient),
	}
}
