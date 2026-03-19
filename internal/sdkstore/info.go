// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package sdkstore

import (
	"context"
	"net/http"
	"strings"

	"github.com/canonical/workshop/internal/sdkstore/path"
	"github.com/canonical/workshop/internal/sdkstore/transport"
)

// InfoOption to be passed to Info to customize the resulting request.
type InfoOption func(*infoOptions)

type infoOptions struct {
	fields []string
}

var defaultInfoFields = []string{
	"channel-map",
	"created-at",
	"description",
	"download",
	"license",
	"publisher",
	"revision",
	"sdk-yaml",
	"summary",
	"title",
	"version",
}

// WithInfoFields sets the fields to query (nil for default).
func WithInfoFields(fields []string) InfoOption {
	return func(infoOptions *infoOptions) {
		infoOptions.fields = fields
	}
}

func newInfoOptions() *infoOptions {
	return &infoOptions{}
}

type infoClient struct {
	path   path.Path
	client RESTClient
}

func newInfoClient(path path.Path, client RESTClient) *infoClient {
	return &infoClient{
		path:   path,
		client: client,
	}
}

// Info queries the SDK Store for metadata belonging to the given SDK.
func (c *infoClient) Info(ctx context.Context, name string, options ...InfoOption) (transport.InfoResponse, error) {
	var resp struct {
		transport.InfoResponse
		transport.ErrorResponse
	}
	if err := c.info(ctx, &resp, name, options...); err != nil {
		return resp.InfoResponse, err
	}
	if err := handleBasicAPIErrors(resp.ErrorList); err != nil {
		return resp.InfoResponse, err
	}

	return resp.InfoResponse, nil
}

func (c *infoClient) info(ctx context.Context, resp any, name string, options ...InfoOption) error {
	opts := newInfoOptions()
	for _, option := range options {
		option(opts)
	}

	if opts.fields == nil {
		opts.fields = defaultInfoFields
	}
	path, err := c.path.JoinPath(name).Query("fields", strings.Join(opts.fields, ","))
	if err != nil {
		return err
	}

	restResp, err := c.client.Get(ctx, path, resp)
	if err != nil {
		return err
	}
	if restResp.StatusCode == http.StatusNotFound {
		return &SdkNotFoundError{Name: name}
	}
	return nil
}
