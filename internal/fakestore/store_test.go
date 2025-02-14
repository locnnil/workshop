package store_test

import (
	"context"
	"testing"

	"gopkg.in/check.v1"

	store "github.com/canonical/workshop/internal/fakestore"
	"github.com/canonical/workshop/internal/sdk"
)

type storeSuite struct {
}

var _ = check.Suite(&storeSuite{})

func Test(t *testing.T) {
	check.TestingT(t)
}

var testSdk = `name: test-sdk
base: ubuntu@20.04
license: LGPL-2.1
summary: The Go programming language
description: |
  Go is an open source programming language that enables
  the production of simple, efficient and reliable software at scale.
plugs:
  data:
    interface: mount
    workshop-target: /test
  gpu:
    interface: gpu
`

func (s *storeSuite) TestSdkActionInstallOK(c *check.C) {
	r := store.FakeSdkStoreInfo(func(ctx context.Context, name, channel string) (store.StoreSdk, error) {
		var s = store.StoreSdk{
			Name:     "test-sdk",
			Channel:  channel,
			Revision: sdk.Revision{N: 123},
			SdkYAML:  testSdk,
		}
		return s, nil
	})
	defer r()
	defer sdk.MockSanitizePlugsSlots(func(sdkInfo *sdk.Info) {})()

	store := store.New()
	acts := []sdk.SdkAction{{
		ProjectId: "24242424",
		Workshop:  "test-workshop",
		Action:    sdk.Install,
		Name:      "test-sdk",
		Base:      "ubuntu@20.04",
		Channel:   "latest/stable",
	},
	}
	res, err := store.SdkAction(context.Background(), acts)
	c.Assert(res, check.HasLen, 1)
	c.Assert(err, check.IsNil)
}

func (s *storeSuite) TestSdkActionInstallCannotParseSdkInfo(c *check.C) {
	r := store.FakeSdkStoreInfo(func(ctx context.Context, name, channel string) (store.StoreSdk, error) {
		var s = map[string]store.StoreSdk{
			"test-sdk-broken": {
				Name:     "test-sdk-broken",
				Channel:  channel,
				Revision: sdk.Revision{N: 123},
				SdkYAML:  `incorrect yaml: -`,
			},
			"test-sdk-valid": {
				Name:     "test-sdk",
				Channel:  channel,
				Revision: sdk.Revision{N: 123},
				SdkYAML:  testSdk,
			},
		}
		return s[name], nil
	})
	defer r()
	defer sdk.MockSanitizePlugsSlots(func(sdkInfo *sdk.Info) {})()

	store := store.New()
	acts := []sdk.SdkAction{{
		ProjectId: "24242424",
		Workshop:  "test-workshop",
		Action:    sdk.Install,
		Name:      "test-sdk-broken",
		Base:      "ubuntu@20.04",
		Channel:   "latest/stable",
	}, {
		ProjectId: "24242424",
		Workshop:  "test-workshop",
		Action:    sdk.Install,
		Name:      "test-sdk-valid",
		Base:      "ubuntu@20.04",
		Channel:   "latest/stable",
	}}
	res, err := store.SdkAction(context.Background(), acts)
	c.Assert(res, check.HasLen, 1)
	c.Assert(err, check.ErrorMatches, "(?s)*.test-sdk-broken: yaml: block sequence entries are not allowed in this context")
}

func (s *storeSuite) TestSdkActionInstallIncompatibleBase(c *check.C) {
	r := store.FakeSdkStoreInfo(func(ctx context.Context, name, channel string) (store.StoreSdk, error) {
		var s = store.StoreSdk{
			Name:     "test-sdk",
			Channel:  channel,
			Revision: sdk.Revision{N: 123},
			SdkYAML:  testSdk,
		}
		return s, nil
	})
	defer r()
	defer sdk.MockSanitizePlugsSlots(func(sdkInfo *sdk.Info) {})()

	store := store.New()
	acts := []sdk.SdkAction{{
		ProjectId: "24242424",
		Workshop:  "test-workshop",
		Action:    sdk.Install,
		Name:      "test-sdk",
		Base:      "ubuntu@22.04",
		Channel:   "latest/stable",
	},
	}
	_, err := store.SdkAction(context.Background(), acts)
	c.Assert(err, check.ErrorMatches, `"test-sdk" SDK from "latest/stable" has "ubuntu@20.04" base; required: "ubuntu@22.04"`)
}
