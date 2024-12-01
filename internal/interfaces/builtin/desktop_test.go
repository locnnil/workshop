package builtin_test

import (
	"os/user"
	"path/filepath"

	"github.com/canonical/workshop/internal/dirs"
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

func (s *desktopSuite) TestDesktopInterfaceWayland(c *check.C) {
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
	deviceSpec := lxd_device.NewSpecification(&testuser, "consumer")

	fake := testutil.FakeCommand(c, "sudo", `
echo "XDG_RUNTIME_DIR=/tmp"
echo "WAYLAND_DISPLAY=wayland-1"
exit 0`)
	defer fake.Restore()

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.IsNil)
	expectedProxy := &workshop.Desktop{}
	expectedProxy.Wayland.Name = "consumer-wayland"
	expectedProxy.Wayland.Connect = "/tmp/wayland-1"
	expectedProxy.Wayland.Listen = "/run/user/1000/wayland-1"
	c.Assert(deviceSpec.Profile.Desktop, check.DeepEquals, expectedProxy)
}

func (s *desktopSuite) TestDesktopInterfaceX11(c *check.C) {
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
	deviceSpec := lxd_device.NewSpecification(&testuser, "consumer")

	fake := testutil.FakeCommand(c, "sudo", `
echo "XDG_RUNTIME_DIR=/tmp"
echo "DISPLAY=:0"
exit 0`)
	defer fake.Restore()

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.IsNil)
	expectedProxy := &workshop.Desktop{}
	expectedProxy.X11.Name = "consumer-x11"
	expectedProxy.X11.Connect = "/tmp/.X11-unix/X0"
	expectedProxy.X11.Listen = "/tmp/.X11-unix/X0"
	c.Assert(deviceSpec.Profile.Desktop, check.DeepEquals, expectedProxy)
}

func (s *desktopSuite) TestDesktopInterfaceBoth(c *check.C) {
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
	deviceSpec := lxd_device.NewSpecification(&testuser, "consumer")

	fake := testutil.FakeCommand(c, "sudo", `
echo "XDG_RUNTIME_DIR=/tmp"
echo "DISPLAY=:0"
echo "WAYLAND_DISPLAY=wayland-0"
exit 0`)
	defer fake.Restore()

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.IsNil)
	expectedProxy := &workshop.Desktop{}
	expectedProxy.Wayland.Name = "consumer-wayland"
	expectedProxy.Wayland.Connect = "/tmp/wayland-0"
	expectedProxy.Wayland.Listen = "/run/user/1000/wayland-0"
	expectedProxy.X11.Name = "consumer-x11"
	expectedProxy.X11.Connect = "/tmp/.X11-unix/X0"
	expectedProxy.X11.Listen = "/tmp/.X11-unix/X0"
	c.Assert(deviceSpec.Profile.Desktop, check.DeepEquals, expectedProxy)
}

func (s *desktopSuite) TestDesktopInterfaceXauth(c *check.C) {
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
	deviceSpec := lxd_device.NewSpecification(&testuser, "consumer")

	fake := testutil.FakeCommand(c, "sudo", `
echo "XDG_RUNTIME_DIR=/tmp"
echo "DISPLAY=:0"
echo "XAUTHORITY=/tmp/.Xauthority"
exit 0`)
	defer fake.Restore()

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.IsNil)
	expectedMount := &workshop.Mount{Name: "consumer-xauth", What: filepath.Join(dirs.WorkshopdRunDir, deviceSpec.User.Uid, ".Xauthority"), Where: "/var/lib/workshop/run/.Xauthority"}
	c.Assert(deviceSpec.Profile.Mounts["consumer-xauth"], check.DeepEquals, *expectedMount)
}

func (s *desktopSuite) TestDesktopInterfaceXauthFail(c *check.C) {
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
	deviceSpec := lxd_device.NewSpecification(&testuser, "consumer")

	fake := testutil.FakeCommand(c, "sudo", `
echo "XDG_RUNTIME_DIR=/tmp"
echo "DISPLAY=:0"
exit 0`)
	defer fake.Restore()

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.IsNil)
	_, ok := deviceSpec.Profile.Mounts["consumer-xauth"]
	c.Assert(!ok, check.Equals, true)
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
	deviceSpec := lxd_device.NewSpecification(&testuser, "consumer")

	fake := testutil.FakeCommand(c, "sudo", `
echo XDG_RUNTIME_DIR="/tmp"
echo WAYLAND_DISPLAY=""
exit 0`)
	defer fake.Restore()

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.ErrorMatches, "neither DISPLAY nor WAYLAND_DISPLAY.*")
}

func (s *desktopSuite) TestDesktopEnvX11Fail(c *check.C) {
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
	deviceSpec := lxd_device.NewSpecification(&testuser, "consumer")

	fake := testutil.FakeCommand(c, "sudo", `
echo XDG_RUNTIME_DIR="/tmp"
echo DISPLAY=""
exit 0`)
	defer fake.Restore()

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.ErrorMatches, "neither DISPLAY nor WAYLAND_DISPLAY.*")
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
	deviceSpec := lxd_device.NewSpecification(&testuser, "consumer")

	fake := testutil.FakeCommand(c, "sudo", `
echo XDG_RUNTIME_DIR=""
echo WAYLAND_DISPLAY="wayland-1"
exit 0`)
	defer fake.Restore()

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.ErrorMatches, "XDG_RUNTIME_DIR is either empty.*")
}
