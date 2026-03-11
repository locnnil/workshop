//go:build integration

package sdkstore

import (
	"context"
	_ "embed"
	"encoding/json"

	"gopkg.in/check.v1"
)

type infoIntegration struct{}

var _ = check.Suite(&infoIntegration{})

func (f *infoIntegration) TestSdkInfo(c *check.C) {
	client := NewClient(Config{})
	var response any
	err := client.info(context.Background(), &response, "test-sdk-info")
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testSdkInfoRaw, &expected)
	c.Assert(err, check.IsNil)
	c.Check(response, check.DeepEquals, expected)
}

func (f *infoIntegration) TestSdkInfoMultiBase(c *check.C) {
	client := NewClient(Config{})
	var response any
	err := client.info(context.Background(), &response, "test-sdk-info-multi-base", WithInfoFields(allInfoFields))
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testSdkInfoMultiBaseRaw, &expected)
	c.Assert(err, check.IsNil)
	c.Check(response, check.DeepEquals, expected)
}
