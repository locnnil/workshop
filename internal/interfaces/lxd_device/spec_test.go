// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package lxd_device

import (
	"context"
	"os/user"
	"strconv"
	"testing"

	"github.com/canonical/lxd/shared/api"
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/workshop"
)

func Test(t *testing.T) { check.TestingT(t) }

type lxdSpecSuite struct {
	restoreUserLookup func()
	restoreUserEnv    func()
	restoreLxdInfo    func()
}

var _ = check.Suite(&lxdSpecSuite{})

func (s *lxdSpecSuite) SetUpTest(c *check.C) {
	s.restoreUserLookup = osutil.FakeUserLookup(func(name string) (*user.User, error) {
		return &user.User{Username: "testuser", Uid: "1000", Gid: "1000", HomeDir: "/home/testuser"}, nil
	})
	s.restoreUserEnv = osutil.FakeUserEnvironment(func(u *user.User) (map[string]string, error) {
		return nil, nil
	})
}

func (s *lxdSpecSuite) TearDownTest(c *check.C) {
	if s.restoreLxdInfo != nil {
		s.restoreLxdInfo()
	}
	s.restoreUserEnv()
	s.restoreUserLookup()
}

func (s *lxdSpecSuite) TestAddCustomDeviceSubsystem(c *check.C) {
	s.restoreLxdInfo = MockLxdServerInfo(func(ctx context.Context) (*api.Resources, error) {
		return &api.Resources{}, nil
	})

	spec, err := NewSpecification("testuser", "test-sdk")
	c.Assert(err, check.IsNil)

	err = spec.AddCustomDevice(workshop.CustomDevice{Name: "mydevice", Subsystem: "tty"})
	c.Assert(err, check.IsNil)

	// Without a files allowlist, a single unix-hotplug device matches the
	// whole subsystem.
	c.Assert(spec.devices, check.HasLen, 1)
	dev, ok := spec.devices["test-sdk_mydevice"]
	c.Assert(ok, check.Equals, true)
	c.Check(dev["type"], check.Equals, "unix-hotplug")
	c.Check(dev["subsystem"], check.Equals, "tty")
	c.Check(dev["required"], check.Equals, "false")
	c.Check(dev["ownership.inherit"], check.Equals, "true")
	c.Check(spec.config["user.workshop.test-sdk_mydevice.type"], check.Equals, "custom-device")
}

func (s *lxdSpecSuite) TestAddCustomDeviceFiles(c *check.C) {
	s.restoreLxdInfo = MockLxdServerInfo(func(ctx context.Context) (*api.Resources, error) {
		return &api.Resources{}, nil
	})

	spec, err := NewSpecification("testuser", "test-sdk")
	c.Assert(err, check.IsNil)

	device := workshop.CustomDevice{
		Name:      "mydevice",
		Subsystem: "tty",
		Files:     []string{"/dev/tnt0", "/dev/tnt1"},
	}
	err = spec.AddCustomDevice(device)
	c.Assert(err, check.IsNil)

	// With a files allowlist: one type=none JSON blob plus one unix-char per
	// allowed path.
	c.Assert(spec.devices, check.HasLen, 3)

	blob, ok := spec.devices["test-sdk_mydevice"]
	c.Assert(ok, check.Equals, true)
	c.Check(blob["type"], check.Equals, "none")
	c.Check(spec.config["user.workshop.test-sdk_mydevice.type"], check.Equals, "custom-device")
	c.Check(spec.config["user.workshop.test-sdk_mydevice"], check.Equals,
		`{"name":"mydevice","subsystem":"tty","files":["/dev/tnt0","/dev/tnt1"]}`)

	for i, path := range device.Files {
		name := "test-sdk_mydevice_" + strconv.Itoa(i)
		dev, ok := spec.devices[name]
		c.Assert(ok, check.Equals, true, check.Commentf("missing device %q", name))
		c.Check(dev["type"], check.Equals, "unix-char")
		c.Check(dev["source"], check.Equals, path)
		c.Check(dev["required"], check.Equals, "false")
		c.Check(dev["uid"], check.Equals, workshop.User.Uid)
		c.Check(dev["gid"], check.Equals, workshop.User.Gid)
		c.Check(spec.config["user.workshop."+name+".type"], check.Equals, "custom-device")
	}
}

func (s *lxdSpecSuite) TestSetGpuCDINoGpuDetected(c *check.C) {
	s.restoreLxdInfo = MockLxdServerInfo(func(ctx context.Context) (*api.Resources, error) {
		return &api.Resources{GPU: api.ResourcesGPU{Total: 0}}, nil
	})

	spec, err := NewSpecification("testuser", "test-sdk")
	c.Assert(err, check.IsNil)

	err = spec.SetGpu(workshop.Gpu{Name: "gpu"})
	c.Assert(err, check.IsNil)

	// CDI with no GPUs: no devices created.
	c.Assert(spec.devices, check.HasLen, 0)
}

func (s *lxdSpecSuite) TestSetGpuCDINvidia(c *check.C) {
	s.restoreLxdInfo = MockLxdServerInfo(func(ctx context.Context) (*api.Resources, error) {
		return &api.Resources{GPU: api.ResourcesGPU{
			Total: 1,
			Cards: []api.ResourcesGPUCard{
				{VendorID: "10de", DRM: &api.ResourcesGPUCardDRM{ID: 0}},
			},
		}}, nil
	})

	spec, err := NewSpecification("testuser", "test-sdk")
	c.Assert(err, check.IsNil)

	err = spec.SetGpu(workshop.Gpu{Name: "gpu"})
	c.Assert(err, check.IsNil)

	c.Assert(spec.devices, check.HasLen, 1)
	dev, ok := spec.devices["test-sdk_gpu_nvidia"]
	c.Assert(ok, check.Equals, true)
	c.Check(dev["type"], check.Equals, "gpu")
	c.Check(dev["gputype"], check.Equals, "physical")
	c.Check(dev["id"], check.Equals, "nvidia.com/gpu=all")
}

func (s *lxdSpecSuite) TestSetGpuCDIAmd(c *check.C) {
	s.restoreLxdInfo = MockLxdServerInfo(func(ctx context.Context) (*api.Resources, error) {
		return &api.Resources{GPU: api.ResourcesGPU{
			Total: 1,
			Cards: []api.ResourcesGPUCard{
				{VendorID: "1002", DRM: &api.ResourcesGPUCardDRM{ID: 1}},
			},
		}}, nil
	})

	spec, err := NewSpecification("testuser", "test-sdk")
	c.Assert(err, check.IsNil)

	err = spec.SetGpu(workshop.Gpu{Name: "gpu"})
	c.Assert(err, check.IsNil)

	c.Assert(spec.devices, check.HasLen, 1)
	dev, ok := spec.devices["test-sdk_gpu_amd"]
	c.Assert(ok, check.Equals, true)
	c.Check(dev["id"], check.Equals, "amd.com/gpu=all")
}

func (s *lxdSpecSuite) TestSetGpuCDIIntel(c *check.C) {
	s.restoreLxdInfo = MockLxdServerInfo(func(ctx context.Context) (*api.Resources, error) {
		return &api.Resources{GPU: api.ResourcesGPU{
			Total: 1,
			Cards: []api.ResourcesGPUCard{
				{VendorID: "8086", DRM: &api.ResourcesGPUCardDRM{ID: 3}},
			},
		}}, nil
	})

	spec, err := NewSpecification("testuser", "test-sdk")
	c.Assert(err, check.IsNil)

	err = spec.SetGpu(workshop.Gpu{Name: "gpu"})
	c.Assert(err, check.IsNil)

	// Intel falls back to physical GPU with specific ID.
	c.Assert(spec.devices, check.HasLen, 1)
	dev, ok := spec.devices["test-sdk_gpu_intel_3"]
	c.Assert(ok, check.Equals, true)
	c.Check(dev["type"], check.Equals, "gpu")
	c.Check(dev["gputype"], check.Equals, "physical")
	c.Check(dev["id"], check.Equals, "3")
	c.Check(dev["uid"], check.Equals, workshop.User.Uid)
	c.Check(dev["gid"], check.Equals, workshop.User.Gid)
}

func (s *lxdSpecSuite) TestSetGpuCDIMultipleVendors(c *check.C) {
	s.restoreLxdInfo = MockLxdServerInfo(func(ctx context.Context) (*api.Resources, error) {
		return &api.Resources{GPU: api.ResourcesGPU{
			Total: 3,
			Cards: []api.ResourcesGPUCard{
				{VendorID: "10de", DRM: &api.ResourcesGPUCardDRM{ID: 0}},
				{VendorID: "10de", DRM: &api.ResourcesGPUCardDRM{ID: 1}},
				{VendorID: "8086", DRM: &api.ResourcesGPUCardDRM{ID: 2}},
			},
		}}, nil
	})

	spec, err := NewSpecification("testuser", "test-sdk")
	c.Assert(err, check.IsNil)

	err = spec.SetGpu(workshop.Gpu{Name: "gpu"})
	c.Assert(err, check.IsNil)

	// nvidia deduplicates to one "all" entry, intel gets its own per-card entry.
	c.Assert(spec.devices, check.HasLen, 2)
	_, hasNvidia := spec.devices["test-sdk_gpu_nvidia"]
	c.Check(hasNvidia, check.Equals, true)
	_, hasIntel := spec.devices["test-sdk_gpu_intel_2"]
	c.Check(hasIntel, check.Equals, true)
}

func (s *lxdSpecSuite) TestSetGpuCDIUnknownVendor(c *check.C) {
	s.restoreLxdInfo = MockLxdServerInfo(func(ctx context.Context) (*api.Resources, error) {
		return &api.Resources{GPU: api.ResourcesGPU{
			Total: 1,
			Cards: []api.ResourcesGPUCard{
				{VendorID: "ffff", DRM: &api.ResourcesGPUCardDRM{ID: 0}},
			},
		}}, nil
	})

	spec, err := NewSpecification("testuser", "test-sdk")
	c.Assert(err, check.IsNil)

	err = spec.SetGpu(workshop.Gpu{Name: "gpu"})
	c.Assert(err, check.IsNil)
	c.Assert(spec.devices, check.HasLen, 0)
}

func (s *lxdSpecSuite) TestSetGpuCDIIntelWithoutDRM(c *check.C) {
	s.restoreLxdInfo = MockLxdServerInfo(func(ctx context.Context) (*api.Resources, error) {
		return &api.Resources{GPU: api.ResourcesGPU{
			Total: 1,
			Cards: []api.ResourcesGPUCard{
				{VendorID: "8086"},
			},
		}}, nil
	})

	spec, err := NewSpecification("testuser", "test-sdk")
	c.Assert(err, check.IsNil)

	err = spec.SetGpu(workshop.Gpu{Name: "gpu"})
	c.Assert(err, check.IsNil)
	c.Assert(spec.devices, check.HasLen, 0)
}

func (s *lxdSpecSuite) TestSetGpuCDIUnknownVendorWithoutDRM(c *check.C) {
	s.restoreLxdInfo = MockLxdServerInfo(func(ctx context.Context) (*api.Resources, error) {
		return &api.Resources{GPU: api.ResourcesGPU{
			Total: 1,
			Cards: []api.ResourcesGPUCard{
				{VendorID: "ffff"},
			},
		}}, nil
	})

	spec, err := NewSpecification("testuser", "test-sdk")
	c.Assert(err, check.IsNil)

	err = spec.SetGpu(workshop.Gpu{Name: "gpu"})
	c.Assert(err, check.IsNil)
	c.Assert(spec.devices, check.HasLen, 0)
}
