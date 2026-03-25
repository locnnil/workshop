// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package sdkstore

import (
	"context"

	"github.com/canonical/workshop/internal/sdkstore/path"
	"github.com/canonical/workshop/internal/sdkstore/transport"
)

type resolveClient struct {
	path   path.Path
	client RESTClient
}

func newResolveClient(path path.Path, client RESTClient) *resolveClient {
	return &resolveClient{
		path:   path,
		client: client,
	}
}

// Resolve is used to find the latest revisions of a given set of SDKs.
func (c *resolveClient) Resolve(ctx context.Context, req transport.ResolveRequest) (transport.ResolveResponse, error) {
	var resp struct {
		transport.ResolveResponse
		transport.ErrorResponse
	}
	if err := c.resolve(ctx, &resp, req); err != nil {
		return resp.ResolveResponse, err
	}
	if err := handleBasicAPIErrors(resp.ErrorList); err != nil {
		return resp.ResolveResponse, err
	}

	return resp.ResolveResponse, nil
}

func (c *resolveClient) resolve(ctx context.Context, resp any, req transport.ResolveRequest) error {
	_, err := c.client.Post(ctx, c.path, nil, req, resp)
	return err
}
