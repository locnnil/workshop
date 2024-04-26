package workshopbackend_test

import (
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/workshopbackend"
)

type ProfileSuite struct {
}

var _ = check.Suite(&ProfileSuite{})

func (f *ProfileSuite) TestWorkshopDeviceTypes(c *check.C) {
	mount := workshopbackend.Mount("mount", "/tmp/project", "/project")
	c.Assert(mount.Name(), check.Equals, "mount")
	c.Assert(mount.Type(), check.Equals, workshopbackend.BindMount)

	volume := workshopbackend.Volume("volume", "/tmp/project", "state-volume")
	c.Assert(volume.Name(), check.Equals, "volume")
	c.Assert(volume.Type(), check.Equals, workshopbackend.DiskVolume)
}
