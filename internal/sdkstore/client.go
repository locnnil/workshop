// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package sdkstore

import (
	"net/url"
	"time"

	"github.com/canonical/workshop/internal/https"
	"github.com/canonical/workshop/internal/sdkstore/path"
)

const (
	// DefaultServerURL is the default location of the global SDK Store API.
	// An alternate location can be configured by changing the URL
	// field in the Config struct.
	DefaultServerURL = "https://api.staging.pkg.store"

	// RefreshTimeout is the timout callers should use for Refresh calls.
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

// basePath returns the base configuration path for speaking to the server API.
func basePath(base *url.URL) path.Path {
	return path.MakePath(base).JoinPath(serverVersion, serverEntity)
}

// Client represents the client side of an SDK store.
type Client struct{}

// NewClient creates a new SDK Store client from the supplied configuration.
func NewClient(config Config) *Client {
	return &Client{}
}
