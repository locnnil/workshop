package builtin_test

import (
	"os/user"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/interfaces/lxd_device"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
)

type cameraSuite struct {
	iface             interfaces.Interface
	projectId         string
	restoreUserLookup func()
	restoreUserEnv    func()
}

var _ = check.Suite(&cameraSuite{
	iface: builtin.MustInterface("camera"),
})

func (s *cameraSuite) SetUpSuite(c *check.C) {
	s.projectId = "42424242"
	s.restoreUserLookup = osutil.FakeUserLookup(func(name string) (*user.User, error) {
		return &testuser, nil
	})
	s.restoreUserEnv = osutil.FakeUserEnvironment(func(user *user.User) (map[string]string, error) {
		return nil, nil
	})
}

func (s *cameraSuite) TearDownSuite(c *check.C) {
	s.restoreUserEnv()
	s.restoreUserLookup()
}

func (s *cameraSuite) TestName(c *check.C) {
	c.Assert(s.iface.Name(), check.Equals, "camera")
}

func (s *cameraSuite) TestInterfaces(c *check.C) {
	c.Check(builtin.Interfaces(), testutil.DeepContains, s.iface)
}

func (s *cameraSuite) TestCameraInterface(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 camera:
  interface: camera
`, s.projectId, "ws", "consumer", "camera")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	slot := builtin.MockSlot(c, `name: producer
base: ubuntu@22.04
slots:
  camera:
`, s.projectId, "ws", "producer", "camera")
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)
	deviceSpec, err := lxd_device.NewSpecification(testuser.Username, "consumer")
	c.Assert(err, check.IsNil)

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.IsNil)

	// Validate the device specification.
	expectedDevice := &workshop.Camera{Name: "camera"}
	c.Assert(deviceSpec.Profile.Camera, check.DeepEquals, expectedDevice)
}
