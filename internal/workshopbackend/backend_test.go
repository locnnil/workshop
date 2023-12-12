package workshopbackend_test

import (
	"github.com/canonical/workshop/internal/workshopbackend"
	"gopkg.in/check.v1"
)

type BackendTest struct {
}

var _ = check.Suite(&BackendTest{})

func (f *BackendTest) SetUpTest(c *check.C) {
}

func (f *BackendTest) TestMountDevice(c *check.C) {
	mount := workshopbackend.Mount("sdk", "/source", "/target")
	c.Assert(mount.Name(), check.Equals, "sdk")
	c.Assert(workshopbackend.LxdDevices(&mount), check.DeepEquals, map[string]string{"type": "disk", "source": "/source", "path": "/target"})
}
