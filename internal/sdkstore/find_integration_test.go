//go:build integration

package sdkstore

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/sdkstore/transport"
)

type findIntegration struct{}

var _ = check.Suite(&findIntegration{})

func (f *findIntegration) TestSdkFind(c *check.C) {
	client := NewClient(Config{})
	var response any
	err := client.find(context.Background(), &response, "test-sdk-find-2")
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testSdkFindRaw, &expected)
	c.Assert(err, check.IsNil)
	c.Check(response, check.DeepEquals, expected)
}

func (f *findIntegration) TestSdkFindWithPlatform(c *check.C) {
	platforms := []transport.Platform{{
		Name:         "all",
		Channel:      "all",
		Architecture: "s390x",
	}}

	client := NewClient(Config{})
	var response any
	err := client.find(context.Background(), &response, "test-sdk-find-2", WithFindFields(allFindFields), WithFindPlatforms(platforms))
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testSdkFindS390XRaw, &expected)
	c.Assert(err, check.IsNil)
	c.Check(response, check.DeepEquals, expected)
}

func (f *findIntegration) TestSdkFindByPublisher(c *check.C) {
	// Use platform to narrow down the search results.
	platforms := []transport.Platform{{
		Name:         "all",
		Channel:      "all",
		Architecture: "s390x",
	}}

	client := NewClient(Config{})
	responses, err := client.Find(context.Background(), "dlyfar", WithFindFields(allFindFields), WithFindPlatforms(platforms))
	c.Assert(err, check.IsNil)

	var expected transport.FindResponses
	err = json.Unmarshal(testSdkFindS390XRaw, &expected)
	c.Assert(err, check.IsNil)
	c.Assert(expected.Results, check.HasLen, 1)

	found := slices.ContainsFunc(responses, func(r transport.FindResponse) bool {
		return reflect.DeepEqual(r, expected.Results[0])
	})
	c.Check(found, check.Equals, true)
}

func (f *findIntegration) TestSdkFindByTitle(c *check.C) {
	client := NewClient(Config{})
	var response any
	err := client.find(context.Background(), &response, "Test SDK find 2 s390x title", WithFindFields(allFindFields))
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testSdkFindS390XRaw, &expected)
	c.Assert(err, check.IsNil)
	c.Check(response, check.DeepEquals, expected)
}

func (f *findIntegration) TestSdkFindBySummary(c *check.C) {
	client := NewClient(Config{})
	var response any
	err := client.find(context.Background(), &response, "Test SDK find 2 s390x summary", WithFindFields(allFindFields))
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testSdkFindS390XRaw, &expected)
	c.Assert(err, check.IsNil)
	c.Check(response, check.DeepEquals, expected)
}

func (f *findIntegration) TestSdkFindByDescription(c *check.C) {
	client := NewClient(Config{})
	var response any
	err := client.find(context.Background(), &response, "Test SDK find 2 s390x description", WithFindFields(allFindFields))
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testSdkFindS390XRaw, &expected)
	c.Assert(err, check.IsNil)
	c.Check(response, check.DeepEquals, expected)
}
