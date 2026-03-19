// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package sdkstore

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	"go.uber.org/mock/gomock"
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/sdkstore/path"
	"github.com/canonical/workshop/internal/sdkstore/transport"
	"github.com/canonical/workshop/internal/testutil"
)

//go:embed test-sdk-find.raw.json
var testSdkFindRaw []byte

//go:embed test-sdk-find.json
var testSdkFindResponse []byte

//go:embed test-sdk-find-s390x.raw.json
var testSdkFindS390XRaw []byte

//go:embed test-sdk-find-s390x.json
var testSdkFindS390XResponse []byte

type FindSuite struct{}

var _ = check.Suite(&FindSuite{})

func (s *FindSuite) TestFind(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	baseURL := MustParseURL(c, "http://api.foo.bar")

	path := path.MakePath(baseURL)
	query := "test-sdk-find"

	restClient := NewMockRESTClient(ctrl)
	s.expectGet(c, restClient, path, query)

	client := newFindClient(path, restClient)
	responses, err := client.Find(context.Background(), query)
	c.Assert(err, check.IsNil)
	c.Assert(len(responses), check.Equals, 1)
	c.Assert(responses[0].Name, check.Equals, query)
}

func (s *FindSuite) TestFindWithOptions(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	baseURL := MustParseURL(c, "http://api.foo.bar")

	path := path.MakePath(baseURL)
	query := "test-sdk-find"

	expect, err := path.Query("categories", "ide,language")
	c.Assert(err, check.IsNil)
	expect, err = expect.Query("platforms", "ubuntu#24.04#amd64,ubuntu#24.04#arm64")
	c.Assert(err, check.IsNil)

	restClient := NewMockRESTClient(ctrl)
	s.expectGet(c, restClient, expect, query)

	client := newFindClient(path, restClient)
	categories := []string{"ide", "language"}
	platforms := []transport.Platform{
		{Name: "ubuntu", Channel: "24.04", Architecture: "amd64"},
		{Name: "ubuntu", Channel: "24.04", Architecture: "arm64"},
	}
	responses, err := client.Find(context.Background(), query, WithFindCategories(categories), WithFindPlatforms(platforms))
	c.Assert(err, check.IsNil)
	c.Assert(len(responses), check.Equals, 1)
	c.Assert(responses[0].Name, check.Equals, query)
}

func (s *FindSuite) TestFindFailure(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	baseURL := MustParseURL(c, "http://api.foo.bar")

	path := path.MakePath(baseURL)
	query := "test-sdk-find"

	restClient := NewMockRESTClient(ctrl)
	s.expectGetFailure(restClient)

	client := newFindClient(path, restClient)
	_, err := client.Find(context.Background(), query)
	c.Assert(err, check.NotNil)
}

func (s *FindSuite) expectGet(c *check.C, client *MockRESTClient, p path.Path, query string) {
	namedPath, err := p.Query("q", query)
	c.Assert(err, check.IsNil)
	namedPath, err = namedPath.Query("fields", strings.Join(defaultFindFields, ","))
	c.Assert(err, check.IsNil)

	client.EXPECT().Get(gomock.Any(), namedPath, gomock.Any()).Do(func(_ context.Context, _ path.Path, r any) (restResponse, error) {
		results := []transport.FindResponse{{Name: query}}
		data, err := json.Marshal(transport.FindResponses{Results: results})
		if err != nil {
			return restResponse{}, err
		}
		err = json.Unmarshal(data, r)
		return restResponse{}, err
	})
}

func (s *FindSuite) expectGetFailure(client *MockRESTClient) {
	client.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(restResponse{StatusCode: http.StatusInternalServerError}, errors.New("boom"))
}

func (s *FindSuite) TestFindRequestPayload(c *check.C) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Check(r.URL.Path, check.Equals, "/v2/sdks/find")
		c.Check(r.URL.Query()["q"], check.DeepEquals, []string{"test-sdk-find"})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(testSdkFindRaw)
		c.Assert(err, check.IsNil)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	base := basePath(MustParseURL(c, server.URL))
	findPath := base.JoinPath("find")

	apiRequester := newAPIRequester(DefaultHTTPClient())
	restClient := newHTTPRESTClient(apiRequester)

	client := newFindClient(findPath, restClient)
	responses, err := client.Find(context.Background(), "test-sdk-find")
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testSdkFindResponse, &expected)
	c.Assert(err, check.IsNil)
	c.Check(responses, testutil.JsonEquals, expected)
}

var allFindFields = []string{
	"contact",
	"default-release",
	"description",
	"license",
	"links",
	"media",
	"publisher",
	"summary",
}

func (s *FindSuite) TestFindRequestPayloadS390X(c *check.C) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Check(r.URL.Path, check.Equals, "/v2/sdks/find")
		c.Check(r.URL.Query()["q"], check.DeepEquals, []string{"Test SDK s390x summary"})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(testSdkFindS390XRaw)
		c.Assert(err, check.IsNil)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	base := basePath(MustParseURL(c, server.URL))
	findPath := base.JoinPath("find")

	apiRequester := newAPIRequester(DefaultHTTPClient())
	restClient := newHTTPRESTClient(apiRequester)

	client := newFindClient(findPath, restClient)
	responses, err := client.Find(context.Background(), "Test SDK s390x summary")
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testSdkFindS390XResponse, &expected)
	c.Assert(err, check.IsNil)
	c.Check(responses, testutil.JsonEquals, expected)
}

func (s *FindSuite) TestFindErrorPayload(c *check.C) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Check(r.URL.Path, check.Equals, "/v2/sdks/find")
		c.Check(r.URL.Query()["q"], check.DeepEquals, []string{"not-found"})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`
{"error-list": [{"code": "some-error-code", "message": "not found message"}]}
`))
		c.Assert(err, check.IsNil)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	base := basePath(MustParseURL(c, server.URL))
	findPath := base.JoinPath("find")

	apiRequester := newAPIRequester(DefaultHTTPClient())
	restClient := newHTTPRESTClient(apiRequester)

	client := newFindClient(findPath, restClient)
	_, err := client.Find(context.Background(), "not-found")
	c.Assert(err, check.ErrorMatches, "not found message")
}
