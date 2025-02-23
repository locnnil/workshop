// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package builtin_test

import (
	"fmt"
	"os/user"
	"path/filepath"
	"testing"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/asserts"
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/interfaces/lxd_device"
	"github.com/canonical/workshop/internal/interfaces/policy"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
)

type mountSuite struct {
	iface       interfaces.Interface
	projectId   string
	restoreUser func()
}

func Test(t *testing.T) {
	check.TestingT(t)
}

var _ = check.Suite(&mountSuite{
	iface: builtin.MustInterface("mount"),
})

func (s *mountSuite) SetUpSuite(c *check.C) {
	s.projectId = "42424242"
	testuser.HomeDir = c.MkDir()
	s.restoreUser = workshop.FakeUserLookup(func(name string) (*user.User, error) {
		return &testuser, nil
	})
}

func (s *mountSuite) TearDownSuite(c *check.C) {
	s.restoreUser()
}

func (s *mountSuite) TestName(c *check.C) {
	c.Assert(s.iface.Name(), check.Equals, "mount")
}

func (s *mountSuite) TestInterfaces(c *check.C) {
	c.Check(builtin.Interfaces(), testutil.DeepContains, s.iface)
}

func (s *mountSuite) TestInstallSystemSdkSlot(c *check.C) {
	info := sdk.MockInfo(c, `name: system
base: ubuntu@22.04
type: system
slots:
 mount:
`, s.projectId, "ws")

	ic := policy.InstallCandidate{
		Sdk:             info,
		BaseDeclaration: asserts.BuiltinBaseDeclaration(),
	}
	c.Check(ic.Check(), check.IsNil)
}

func (s *mountSuite) TestInstallOtherSystemSdkSlot(c *check.C) {
	info := sdk.MockInfo(c, `name: system
base: ubuntu@22.04
type: system
slots:
 m:
  interface: mount
`, s.projectId, "ws")

	ic := policy.InstallCandidate{
		Sdk:             info,
		BaseDeclaration: asserts.BuiltinBaseDeclaration(),
	}
	c.Check(ic.Check(), check.ErrorMatches, `installation not allowed by "m" slot rule of interface "mount"`)
}

func (s *mountSuite) TestInstallRegularSdkSlot(c *check.C) {
	info := sdk.MockInfo(c, `name: producer
base: ubuntu@22.04
slots:
 m:
  interface: mount
  workshop-source: /opt
`, s.projectId, "ws")

	ic := policy.InstallCandidate{
		Sdk:             info,
		BaseDeclaration: asserts.BuiltinBaseDeclaration(),
	}
	c.Check(ic.Check(), check.IsNil)
}

func (s *mountSuite) TestInstallSystemSdkPlug(c *check.C) {
	info := sdk.MockInfo(c, `name: system
base: ubuntu@22.04
type: system
plugs:
 mount:
`, s.projectId, "ws")

	ic := policy.InstallCandidate{
		Sdk:             info,
		BaseDeclaration: asserts.BuiltinBaseDeclaration(),
	}
	c.Check(ic.Check(), check.ErrorMatches, `installation not allowed by "mount" plug rule of interface "mount"`)
}

func (s *mountSuite) TestInstallRegularSdkPlug(c *check.C) {
	info := sdk.MockInfo(c, `name: consumer
base: ubuntu@22.04
plugs:
 mount:
  interface: mount
  workshop-target: /mnt
`, s.projectId, "ws")

	ic := policy.InstallCandidate{
		Sdk:             info,
		BaseDeclaration: asserts.BuiltinBaseDeclaration(),
	}
	c.Check(ic.Check(), check.IsNil)
}

func (s *mountSuite) TestSanitizeSlotSystem(c *check.C) {
	const mockSdkYaml = `name: mount-slot-sdk
base: ubuntu@22.04
type: system
slots:
 mount-slot:
  interface: mount
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	slot := info.Slots["mount-slot"]
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.IsNil)
}

func (s *mountSuite) TestSanitizeSlotSystemWorkshopSource(c *check.C) {
	const mockSdkYaml = `name: mount-slot-sdk
base: ubuntu@22.04
type: system
slots:
 mount-slot:
  interface: mount
  workshop-source: /opt
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	slot := info.Slots["mount-slot"]
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.ErrorMatches, `unknown attribute for system mount interface slot: "workshop-source"`)
}

func (s *mountSuite) TestSanitizeSlotSystemHostSource(c *check.C) {
	const mockSdkYaml = `name: mount-slot-sdk
base: ubuntu@22.04
type: system
slots:
 mount-slot:
  interface: mount
  host-source: /usr
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	slot := info.Slots["mount-slot"]
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.ErrorMatches, `unknown attribute for system mount interface slot: "host-source"`)
}

func (s *mountSuite) TestSanitizePlugSimple(c *check.C) {
	const mockSdkYaml = `name: mount-slot-sdk
base: ubuntu@22.04
plugs:
 mount-plug:
  interface: mount
  workshop-target: import
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	plug := info.Plugs["mount-plug"]
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.IsNil)
}

func (s *mountSuite) TestSanitizePlugSimpleNoTarget(c *check.C) {
	const mockSdkYaml = `name: mount-slot-sdk
base: ubuntu@22.04
plugs:
 mount-plug:
  interface: mount
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	plug := info.Plugs["mount-plug"]
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.ErrorMatches, "mount plug must contain target path")
}

func (s *mountSuite) TestSanitizePlugSimpleTargetRelative(c *check.C) {
	const mockSdkYaml = `name: mount-slot-sdk
base: ubuntu@22.04
plugs:
 mount-plug:
  interface: mount
  workshop-target: ../foo
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	plug := info.Plugs["mount-plug"]
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.ErrorMatches, "mount interface path is not clean:.*")
}

func (s *mountSuite) TestSanitizePlugReadOnlyBool(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 mount:
  interface: mount
  workshop-target: /project/training
  read-only: true
`, s.projectId, "ws", "consumer", "mount")
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.IsNil)
}

func (s *mountSuite) TestSanitizePlugReadOnlyString(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 mount:
  interface: mount
  workshop-target: /project/training
  read-only: "true"
`, s.projectId, "ws", "consumer", "mount")
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.IsNil)
}

func (s *mountSuite) TestSanitizePlugReadOnlyInvalid(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 mount:
  interface: mount
  workshop-target: /project/training
  read-only: "invalid"
`, s.projectId, "ws", "consumer", "mount")
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.ErrorMatches, "unknown value \"invalid\" in key \"read-only\" for mount interface plug.*")
}

func (s *mountSuite) TestSanitizeSlotOK(c *check.C) {
	const mockSdkYaml = `name: mount-slot-sdk
base: ubuntu@22.04
slots:
 mount-slot:
  interface: mount
  workshop-source: /images/low-res
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	slot := info.Slots["mount-slot"]
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.IsNil)
}

func (s *mountSuite) TestSanitizeSlotNoSource(c *check.C) {
	const mockSdkYaml = `name: mount-slot-sdk
base: ubuntu@22.04
slots:
 mount-slot:
  interface: mount
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	slot := info.Slots["mount-slot"]
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.ErrorMatches, "mount slot must contain source path")
}

func (s *mountSuite) TestSanitizeSlotAbsSourceFails(c *check.C) {
	const mockSdkYaml = `name: mount-slot-sdk
base: ubuntu@22.04
slots:
 mount-slot:
  interface: mount
  workshop-source: root
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	slot := info.Slots["mount-slot"]
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.ErrorMatches, `mount slot \"workshop-source\" must be absolute`)
}

func (s *mountSuite) TestSanitizeSlotNonLocalSourceFails(c *check.C) {
	const mockSdkYaml = `name: mount-slot-sdk
base: ubuntu@22.04
slots:
 mount-slot:
  interface: mount
  workshop-source: ../../../../../../../../root/
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	slot := info.Slots["mount-slot"]
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.ErrorMatches, `mount slot \"workshop-source\" must be absolute`)
}

func (s *mountSuite) TestConnectHostWorkshopXdgUnset(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 mount:
  interface: mount
  workshop-target: /project/training
`, s.projectId, "ws", "consumer", "mount")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	slot := builtin.MockSlot(c, `name: system
base: ubuntu@22.04
type: system
slots:
 mount:
`, s.projectId, "ws", "system", "mount")
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)

	deviceSpec := lxd_device.NewSpecification(&testuser, "consumer")

	systemCmd := testutil.FakeCommand(c, "sudo", `
  echo XDG_DATA_HOME=""
  exit 0
  `)
	defer systemCmd.Restore()

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.IsNil)

	// Validate the mount specification.
	sourceDir := filepath.Join(testuser.HomeDir, ".local/share/workshop/id/42424242/ws/mount/consumer/mount")
	expectedMnt := workshop.Mount{Name: plug.Name, What: sourceDir, Where: "/project/training", Type: workshop.HostWorkshop}
	c.Assert(deviceSpec.Profile.Mounts, check.DeepEquals, map[string]workshop.Mount{plug.Name: expectedMnt})
}

func (s *mountSuite) TestConnectHostWorkshopXdgSet(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 mount:
  interface: mount
  workshop-target: /project/training
`, s.projectId, "ws", "consumer", "mount")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	slot := builtin.MockSlot(c, `name: system
base: ubuntu@22.04
type: system
slots:
 mount:
`, s.projectId, "ws", "system", "mount")
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)

	deviceSpec := lxd_device.NewSpecification(&testuser, "consumer")

	xdgDir := c.MkDir()
	systemCmd := testutil.FakeCommand(c, "sudo", fmt.Sprintf(`
  echo XDG_DATA_HOME=%s
  exit 0
  `, xdgDir))
	defer systemCmd.Restore()

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.IsNil)

	// Validate the mount specification.
	sourceDir := filepath.Join(xdgDir, "/workshop/id/42424242/ws/mount/consumer/mount")
	expectedMnt := workshop.Mount{Name: plug.Name, What: sourceDir, Where: "/project/training", Type: workshop.HostWorkshop}
	c.Assert(deviceSpec.Profile.Mounts, check.DeepEquals, map[string]workshop.Mount{plug.Name: expectedMnt})
}

func (s *mountSuite) TestConnectWorkshopWorkshop(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 mount:
  interface: mount
  workshop-target: /project/training
`, s.projectId, "ws", "consumer", "mount")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	slot := builtin.MockSlot(c, `name: producer
base: ubuntu@22.04
slots:
 mount:
  interface: mount
  workshop-source: $SDK/training
`, s.projectId, "ws", "producer", "mount")
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)

	deviceSpec := lxd_device.NewSpecification(&testuser, "consumer")

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.IsNil)

	// Validate the mount specification.
	expectedMnt := workshop.Mount{Name: plug.Name, What: "/var/lib/workshop/sdk/producer/current/training", Where: "/project/training", Type: workshop.WorkshopWorkshop}
	c.Assert(deviceSpec.Profile.Mounts, check.DeepEquals, map[string]workshop.Mount{plug.Name: expectedMnt})
}

func (s *mountSuite) TestAutoConnectHostWorkshop(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 mount:
  interface: mount
  workshop-target: /project/training
`, s.projectId, "ws", "consumer", "mount")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	slot := builtin.MockSlot(c, `name: system
base: ubuntu@22.04
type: system
slots:
 mount:
`, s.projectId, "ws", "system", "mount")
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)

	ic := policy.ConnectCandidate{
		Plug:            connectedPlug,
		Slot:            connectedSlot,
		BaseDeclaration: asserts.BuiltinBaseDeclaration(),
	}
	_, err := ic.CheckAutoConnect()
	c.Check(err, check.IsNil)
}

func (s *mountSuite) TestAutoConnectHostWorkshopExplicit(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 mount:
  interface: mount
  workshop-target: /project/training
`, s.projectId, "ws", "consumer", "mount")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	// Simulate a workshop definition file with:
	// connections:
	//   - plug: consumer:mount
	//     slot: system:mount
	connectedPlug.SetAttr("auto-explicit", "true")

	slot := builtin.MockSlot(c, `name: system
base: ubuntu@22.04
type: system
slots:
 mount:
`, s.projectId, "ws", "system", "mount")
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)

	ic := policy.ConnectCandidate{
		Plug:            connectedPlug,
		Slot:            connectedSlot,
		BaseDeclaration: asserts.BuiltinBaseDeclaration(),
	}
	_, err := ic.CheckAutoConnect()
	c.Check(err, check.IsNil)
}

func (s *mountSuite) TestAutoConnectWorkshopWorkshop(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 mount:
  interface: mount
  workshop-target: /project/training
`, s.projectId, "ws", "consumer", "mount")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	slot := builtin.MockSlot(c, `name: producer
base: ubuntu@22.04
slots:
 mount:
  interface: mount
  workshop-source: $SDK/training
`, s.projectId, "ws", "system", "mount")
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)

	ic := policy.ConnectCandidate{
		Plug:            connectedPlug,
		Slot:            connectedSlot,
		BaseDeclaration: asserts.BuiltinBaseDeclaration(),
	}
	_, err := ic.CheckAutoConnect()
	c.Check(err, check.ErrorMatches, `auto-connection not allowed by plug rule of interface "mount"`)
}

func (s *mountSuite) TestAutoConnectWorkshopWorkshopExplicit(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 mount:
  interface: mount
  workshop-target: /project/training
`, s.projectId, "ws", "consumer", "mount")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	// Simulate a workshop definition file with:
	// connections:
	//   - plug: consumer:mount
	//     slot: producer:mount
	connectedPlug.SetAttr("auto-explicit", "true")

	slot := builtin.MockSlot(c, `name: producer
base: ubuntu@22.04
slots:
 mount:
  interface: mount
  workshop-source: $SDK/training
`, s.projectId, "ws", "system", "mount")
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)

	ic := policy.ConnectCandidate{
		Plug:            connectedPlug,
		Slot:            connectedSlot,
		BaseDeclaration: asserts.BuiltinBaseDeclaration(),
	}
	_, err := ic.CheckAutoConnect()
	c.Check(err, check.IsNil)
}
