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

type gpuSuite struct {
	iface          interfaces.Interface
	projectId      string
	restoreUserEnv func()
}

var _ = check.Suite(&gpuSuite{
	iface: builtin.MustInterface("gpu"),
})

func (s *gpuSuite) SetUpSuite(c *check.C) {
	s.projectId = "42424242"
	s.restoreUserEnv = osutil.FakeUserAndEnv(func(name string) (*user.User, map[string]string, error) {
		return &testuser, nil, nil
	})
}

func (s *gpuSuite) TearDownSuite(c *check.C) {
	s.restoreUserEnv()
}

func (s *gpuSuite) TestName(c *check.C) {
	c.Assert(s.iface.Name(), check.Equals, "gpu")
}

func (s *gpuSuite) TestInterfaces(c *check.C) {
	c.Check(builtin.Interfaces(), testutil.DeepContains, s.iface)
}

func (s *gpuSuite) TestGpuInterface(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 gpu:
  interface: gpu
`, s.projectId, "ws", "consumer", "gpu")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	slot := builtin.MockSlot(c, `name: producer
base: ubuntu@22.04
slots:
 gpu:
`, s.projectId, "ws", "producer", "gpu")
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)

	deviceSpec, err := lxd_device.NewSpecification(testuser.Username, "consumer")
	c.Assert(err, check.IsNil)

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.IsNil)

	// Validate the mount specification.
	expectedDevice := &workshop.Gpu{Name: plug.Name}
	c.Assert(deviceSpec.Profile.Gpu, check.DeepEquals, expectedDevice)
}
