// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package sdkstore

import (
	"context"
	"strings"

	"github.com/canonical/workshop/internal/sdkstore/path"
	"github.com/canonical/workshop/internal/sdkstore/transport"
)

// FindOption to be passed to Find to customize the resulting request.
type FindOption func(*findOptions)

type findOptions struct {
	fields     []string
	categories []string
	platforms  []transport.Platform
}

var defaultFindFields = []string{
	"default-release",
	"description",
	"license",
	"publisher",
	"summary",
}

// WithFindFields sets the fields to query (nil for default).
func WithFindFields(fields []string) FindOption {
	return func(findOptions *findOptions) {
		findOptions.fields = fields
	}
}

// WithFindCategories configures categories to filter by.
func WithFindCategories(categories []string) FindOption {
	return func(findOptions *findOptions) {
		findOptions.categories = categories
	}
}

// WithFindPlatforms configures platforms to filter by.
func WithFindPlatforms(platforms []transport.Platform) FindOption {
	return func(findOptions *findOptions) {
		findOptions.platforms = platforms
	}
}

func newFindOptions() *findOptions {
	return &findOptions{}
}

type findClient struct {
	path   path.Path
	client RESTClient
}

func newFindClient(path path.Path, client RESTClient) *findClient {
	return &findClient{
		path:   path,
		client: client,
	}
}

// Find searches the Store for SDKs matching the given query. The query matches
// most text fields: name, title, summary, description and publisher.
func (c *findClient) Find(ctx context.Context, query string, options ...FindOption) (result []transport.FindResponse, err error) {
	var resp struct {
		transport.FindResponses
		transport.ErrorResponse
	}
	if err := c.find(ctx, &resp, query, options...); err != nil {
		return resp.Results, err
	}
	if err := handleBasicAPIErrors(resp.ErrorList); err != nil {
		return resp.Results, err
	}

	return resp.Results, nil
}

func (c *findClient) find(ctx context.Context, resp any, query string, options ...FindOption) error {
	opts := newFindOptions()
	for _, option := range options {
		option(opts)
	}

	if opts.fields == nil {
		opts.fields = defaultFindFields
	}
	path, err := c.path.Query("fields", strings.Join(opts.fields, ","))
	if err != nil {
		return err
	}
	if query != "" {
		path, err = path.Query("q", query)
		if err != nil {
			return err
		}
	}
	if opts.categories != nil {
		path, err = path.Query("categories", strings.Join(opts.categories, ","))
		if err != nil {
			return err
		}
	}
	if opts.platforms != nil {
		platforms := make([]string, 0, len(opts.platforms))
		for _, p := range opts.platforms {
			platforms = append(platforms, p.String())
		}
		path, err = path.Query("platforms", strings.Join(platforms, ","))
		if err != nil {
			return err
		}
	}

	_, err = c.client.Get(ctx, path, resp)
	return err
}
