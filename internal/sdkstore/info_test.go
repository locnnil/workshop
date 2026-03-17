// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package sdkstore

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/juju/errors"
	"go.uber.org/mock/gomock"
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/sdkstore/path"
	"github.com/canonical/workshop/internal/sdkstore/transport"
	"github.com/canonical/workshop/internal/testutil"
)

//go:embed test-sdk-info.raw.json
var testSdkInfoRaw []byte

//go:embed test-sdk-info.json
var testSdkInfoResponse []byte

//go:embed test-sdk-info-multi-base.raw.json
var testSdkInfoMultiBaseRaw []byte

//go:embed test-sdk-info-multi-base.json
var testSdkInfoMultiBaseResponse []byte

type InfoSuite struct{}

var _ = check.Suite(&InfoSuite{})

func (s *InfoSuite) TestInfo(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	baseURL := MustParseURL(c, "http://api.foo.bar")

	path := path.MakePath(baseURL)
	name := "test-sdk-info"

	restClient := NewMockRESTClient(ctrl)
	s.expectGet(c, restClient, path, name)

	client := newInfoClient(path, restClient)
	response, err := client.Info(context.Background(), name)
	c.Assert(err, check.IsNil)
	c.Assert(response.Name, check.Equals, name)
}

func (s *InfoSuite) TestInfoFailure(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	baseURL := MustParseURL(c, "http://api.foo.bar")

	path := path.MakePath(baseURL)
	name := "test-sdk-info"

	restClient := NewMockRESTClient(ctrl)
	s.expectGetFailure(restClient)

	client := newInfoClient(path, restClient)
	_, err := client.Info(context.Background(), name)
	c.Assert(err, check.ErrorMatches, "boom")
}

func (s *InfoSuite) TestInfoError(c *check.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	baseURL := MustParseURL(c, "http://api.foo.bar")

	path := path.MakePath(baseURL)
	name := "test-sdk-info"

	restClient := NewMockRESTClient(ctrl)
	s.expectGetError(c, restClient, path, name)

	client := newInfoClient(path, restClient)
	_, err := client.Info(context.Background(), name)
	c.Assert(err, check.ErrorMatches, `no matching SDKs for "test-sdk-info"`)
}

func (s *InfoSuite) expectGet(c *check.C, client *MockRESTClient, p path.Path, name string) {
	namedPath := p.JoinPath(name)
	namedPath, err := namedPath.Query("fields", strings.Join(defaultInfoFields, ","))
	c.Assert(err, check.IsNil)

	client.EXPECT().Get(gomock.Any(), namedPath, gomock.Any()).Do(func(_ context.Context, _ path.Path, r any) (restResponse, error) {
		data, err := json.Marshal(transport.InfoResponse{Name: name})
		if err != nil {
			return restResponse{}, err
		}
		err = json.Unmarshal(data, r)
		return restResponse{}, err
	})
}

func (s *InfoSuite) expectGetFailure(client *MockRESTClient) {
	client.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(restResponse{StatusCode: http.StatusInternalServerError}, errors.Errorf("boom"))
}

func (s *InfoSuite) expectGetError(c *check.C, client *MockRESTClient, p path.Path, name string) {
	namedPath := p.JoinPath(name)
	namedPath, err := namedPath.Query("fields", strings.Join(defaultInfoFields, ","))
	c.Assert(err, check.IsNil)

	client.EXPECT().Get(gomock.Any(), namedPath, gomock.Any()).DoAndReturn(func(_ context.Context, _ path.Path, r any) (restResponse, error) {
		message := transport.ErrorResponse{ErrorList: []transport.APIError{{Message: "not found"}}}
		data, err := json.Marshal(message)
		if err != nil {
			return restResponse{}, err
		}
		if err := json.Unmarshal(data, r); err != nil {
			return restResponse{}, err
		}
		return restResponse{StatusCode: http.StatusNotFound}, nil
	})
}

func (s *InfoSuite) TestInfoPayload(c *check.C) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Check(r.URL.Path, check.Equals, "/v2/sdks/info/test-sdk-info")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(testSdkInfoRaw)
		c.Check(err, check.IsNil)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	base := basePath(MustParseURL(c, server.URL))
	infoPath := base.JoinPath("info")

	apiRequester := newAPIRequester(DefaultHTTPClient())
	restClient := newHTTPRESTClient(apiRequester)

	client := newInfoClient(infoPath, restClient)
	response, err := client.Info(context.Background(), "test-sdk-info")
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testSdkInfoResponse, &expected)
	c.Assert(err, check.IsNil)
	c.Check(response, testutil.JsonEquals, expected)
}

var allInfoFields = []string{
	"categories",
	"channel-map",
	"contact",
	"created-at",
	"description",
	"download",
	"license",
	"links",
	"media",
	"private",
	"publisher",
	"revision",
	"sdk-yaml",
	"summary",
	"title",
	"version",
	"website",
}

func (s *InfoSuite) TestInfoPayloadMultiBase(c *check.C) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Check(r.URL.Path, check.Equals, "/v2/sdks/info/test-sdk-info-multi-base")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(testSdkInfoMultiBaseRaw)
		c.Check(err, check.IsNil)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	base := basePath(MustParseURL(c, server.URL))
	infoPath := base.JoinPath("info")

	apiRequester := newAPIRequester(DefaultHTTPClient())
	restClient := newHTTPRESTClient(apiRequester)

	client := newInfoClient(infoPath, restClient)
	response, err := client.Info(context.Background(), "test-sdk-info-multi-base", WithInfoFields(allInfoFields))
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testSdkInfoMultiBaseResponse, &expected)
	c.Assert(err, check.IsNil)
	c.Check(response, testutil.JsonEquals, expected)
}
