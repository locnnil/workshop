package builtin_test

import (
	"os/user"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/interfaces/lxd_device"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	"gopkg.in/check.v1"
)

type desktopSuite struct {
	iface     interfaces.Interface
	projectId string
	restore   func()
}

var _ = check.Suite(&desktopSuite{
	iface: builtin.MustInterface("desktop"),
})

func (s *desktopSuite) SetUpTest(c *check.C) {
	s.projectId = "42424242"
	usr, err := user.Current()
	c.Assert(err, check.IsNil)
	homeDir := c.MkDir()
	s.restore = testutil.FakeFunc(func(name string) (*user.User, error) {
		u := &user.User{
			Name:     usr.Name,
			Username: usr.Name,
			Uid:      usr.Uid,
			Gid:      usr.Gid,
			HomeDir:  homeDir,
		}
		return u, nil
	}, &workshop.LookupUsername)
}

func (s *desktopSuite) TearDown(c *check.C) {
	s.restore()
}

func (s *desktopSuite) TestName(c *check.C) {
	c.Assert(s.iface.Name(), check.Equals, "desktop")
}

func (s *desktopSuite) TestInterfaces(c *check.C) {
	c.Check(builtin.Interfaces(), testutil.DeepContains, s.iface)
}

func (s *desktopSuite) TestDesktopInterface(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 desktop:
  interface: desktop
`, s.projectId, "ws", "consumer", "desktop")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	slot := builtin.MockSlot(c, `name: producer
base: ubuntu@22.04
slots:
  desktop:
`, s.projectId, "ws", "producer", "desktop")
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)
	deviceSpec := lxd_device.NewSpecification("testuser", s.projectId, "consumer")

	fake := testutil.FakeCommand(c, "sudo", `
echo "XDG_RUNTIME_DIR=/tmp"
echo "WAYLAND_DISPLAY=wayland-1"
exit 0`)
	defer fake.Restore()

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.IsNil)
	expectedProxy := &workshop.Desktop{Name: "consumer-" + plug.Name, Connect: "/tmp/wayland-1", Listen: "/run/user/1000/wayland-1"}
	c.Assert(deviceSpec.Profile.Desktop, check.DeepEquals, expectedProxy)
}

func (s *desktopSuite) TestDesktopEnvWaylandFail(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 desktop:
  interface: desktop
`, s.projectId, "ws", "consumer", "desktop")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	slot := builtin.MockSlot(c, `name: producer
base: ubuntu@22.04
slots:
  desktop:
`, s.projectId, "ws", "producer", "desktop")
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)
	deviceSpec := lxd_device.NewSpecification("testuser", s.projectId, "consumer")

	fake := testutil.FakeCommand(c, "sudo", `
echo XDG_RUNTIME_DIR="/tmp"
echo WAYLAND_DISPLAY=""
exit 0`)
	defer fake.Restore()

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.ErrorMatches, "WAYLAND_DISPLAY is either empty.*")
}

func (s *desktopSuite) TestDesktopEnvXDGFail(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 desktop:
  interface: desktop
`, s.projectId, "ws", "consumer", "desktop")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	slot := builtin.MockSlot(c, `name: producer
base: ubuntu@22.04
slots:
  desktop:
`, s.projectId, "ws", "producer", "desktop")
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)
	deviceSpec := lxd_device.NewSpecification("testuser", s.projectId, "consumer")

	fake := testutil.FakeCommand(c, "sudo", `
echo XDG_RUNTIME_DIR=""
echo WAYLAND_DISPLAY="wayland-1"
exit 0`)
	defer fake.Restore()

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.ErrorMatches, "XDG_RUNTIME_DIR is either empty.*")
}
