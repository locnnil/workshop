package lxdbackend_test

import (
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/workshop"
	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
)

type ProfileSuite struct {
}

var _ = check.Suite(&ProfileSuite{})

func (f *ProfileSuite) TestWorkshopDeviceTypes(c *check.C) {
	mount := lxdbackend.HostWorkshopMount("mount", "/tmp/project", "/project")
	c.Assert(mount.Name, check.Equals, "mount")
	c.Assert(mount.Type, check.Equals, workshop.HostWorkshopMount)

	volume := lxdbackend.Volume("volume", "/tmp/project", "state-volume")
	c.Assert(volume.Name, check.Equals, "volume")
	c.Assert(volume.Type, check.Equals, workshop.DiskVolume)
}
