package lxdbackend_test

import (
	"gopkg.in/check.v1"

	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
)

type devTest struct {
}

var _ = check.Suite(&devTest{})

func (f *devTest) SetUpTest(c *check.C) {
}

func (f *devTest) TestMountDevice(c *check.C) {
	mount := lxdbackend.HostWorkshopMount("sdk", "/source", "/target")
	c.Assert(mount.Name, check.Equals, "sdk")
	c.Assert(mount.Properties, check.DeepEquals, map[string]string{"type": "disk", "source": "/source", "path": "/target"})
}
