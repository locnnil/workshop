//go:build integration

package sdkstore

import (
	"context"
	"encoding/json"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/sdkstore/transport"
)

type resolveIntegration struct{}

var _ = check.Suite(&resolveIntegration{})

func (f *resolveIntegration) TestResolveByName(c *check.C) {
	req := transport.ResolveRequest{
		Packages: []transport.ResolvePackage{{
			InstanceKey: "random123",
			Namespace:   "sdk",
			Name:        "test-sdk-info-multi-base",
			Channel:     "latest/stable",
			Platform: transport.Platform{
				Name:         "ubuntu",
				Channel:      "22.04",
				Architecture: "amd64",
			},
		}},
	}

	client := NewClient(Config{})
	var response any
	err := client.resolve(context.Background(), &response, req)
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testResolveNameRaw, &expected)
	c.Assert(err, check.IsNil)
	c.Check(response, check.DeepEquals, expected)
}

func (f *resolveIntegration) TestResolveByID(c *check.C) {
	req := transport.ResolveRequest{
		Packages: []transport.ResolvePackage{{
			InstanceKey: "random456",
			Namespace:   "sdk",
			ID:          "ZeW8fMKBPHZBsaSm6LBPbpDZDpVcIHy1",
			Channel:     "latest/edge",
			Platform: transport.Platform{
				Name:         "ubuntu",
				Channel:      "24.04",
				Architecture: "riscv64",
			},
		}},
	}

	client := NewClient(Config{})
	var response any
	err := client.resolve(context.Background(), &response, req)
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testResolveIDRaw, &expected)
	c.Assert(err, check.IsNil)
	c.Check(response, check.DeepEquals, expected)
}

func (f *resolveIntegration) TestResolveNotFound(c *check.C) {
	req := transport.ResolveRequest{
		Packages: []transport.ResolvePackage{{
			InstanceKey: "random789",
			Namespace:   "sdk",
			Name:        "not-found",
			Channel:     "latest/stable",
			Platform: transport.Platform{
				Name:         "ubuntu",
				Channel:      "24.04",
				Architecture: "s390x",
			},
		}},
	}

	client := NewClient(Config{})
	var response any
	err := client.resolve(context.Background(), &response, req)
	c.Assert(err, check.IsNil)

	var expected any
	err = json.Unmarshal(testResolveNotFoundRaw, &expected)
	c.Assert(err, check.IsNil)
	c.Check(response, check.DeepEquals, expected)
}
