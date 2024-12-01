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
	"path/filepath"
	"testing"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/interfaces/lxd_device"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	"gopkg.in/check.v1"
)

type contentSuite struct {
	iface     interfaces.Interface
	projectId string
}

func Test(t *testing.T) {
	check.TestingT(t)
}

var _ = check.Suite(&contentSuite{
	iface: builtin.MustInterface("mount"),
})

func (s *contentSuite) SetUpTest(c *check.C) {
	s.projectId = "42424242"
}

func (s *contentSuite) TestName(c *check.C) {
	c.Assert(s.iface.Name(), check.Equals, "mount")
}

func (s *contentSuite) TestSanitizeSlotSimple(c *check.C) {
	const mockSdkYaml = `name: content-slot-sdk
base: ubuntu@22.04
slots:
 content-slot:
  interface: mount
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	slot := info.Slots["content-slot"]
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.IsNil)
}

func (s *contentSuite) TestSanitizePlugSimple(c *check.C) {
	const mockSdkYaml = `name: content-slot-sdk
base: ubuntu@22.04
plugs:
 content-plug:
  interface: mount
  workshop-target: import
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	plug := info.Plugs["content-plug"]
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.IsNil)
}

func (s *contentSuite) TestSanitizePlugSimpleNoTarget(c *check.C) {
	const mockSdkYaml = `name: content-slot-sdk
base: ubuntu@22.04
plugs:
 content-plug:
  interface: mount
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	plug := info.Plugs["content-plug"]
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.ErrorMatches, "content plug must contain target path")
}

func (s *contentSuite) TestSanitizePlugSimpleTargetRelative(c *check.C) {
	const mockSdkYaml = `name: content-slot-sdk
base: ubuntu@22.04
plugs:
 content-plug:
  interface: mount
  workshop-target: ../foo
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	plug := info.Plugs["content-plug"]
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.ErrorMatches, "content interface path is not clean:.*")
}

func (s *contentSuite) TestSanitizeSlotOK(c *check.C) {
	const mockSdkYaml = `name: content-slot-sdk
base: ubuntu@22.04
slots:
 content-slot:
  interface: mount
  workshop-source: /images/low-res
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	slot := info.Slots["content-slot"]
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.IsNil)
}

func (s *contentSuite) TestSanitizeSlotNoSource(c *check.C) {
	const mockSdkYaml = `name: content-slot-sdk
base: ubuntu@22.04
slots:
 content-slot:
  interface: mount
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	slot := info.Slots["content-slot"]
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.IsNil)
}

func (s *contentSuite) TestSanitizeSlotAbsSourceFails(c *check.C) {
	const mockSdkYaml = `name: content-slot-sdk
base: ubuntu@22.04
slots:
 content-slot:
  interface: mount
  workshop-source: root
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	slot := info.Slots["content-slot"]
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.ErrorMatches, `content slot \"workshop-source\" must be absolute`)
}

func (s *contentSuite) TestSanitizeSlotNonLocalSourceFails(c *check.C) {
	const mockSdkYaml = `name: content-slot-sdk
base: ubuntu@22.04
slots:
 content-slot:
  interface: mount
  workshop-source: ../../../../../../../../root/
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	slot := info.Slots["content-slot"]
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.ErrorMatches, `content slot \"workshop-source\" must be absolute`)
}

func (s *contentSuite) TestInterfaces(c *check.C) {
	c.Check(builtin.Interfaces(), testutil.DeepContains, s.iface)
}

func (s *contentSuite) TestContentInterface(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 mount:
  workshop-target: /project/training
`, s.projectId, "ws", "consumer", "mount")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	slot := builtin.MockSlot(c, `name: producer
base: ubuntu@22.04
slots:
 mount:
`, s.projectId, "ws", "producer", "mount")
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)

	deviceSpec := lxd_device.NewSpecification(&testuser, "consumer")

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.IsNil)

	// Validate the mount specification.
	sourceDir := filepath.Join(testuser.HomeDir, ".local/share/workshop/project/42424242/mount/ws_consumer_mount.sdk")
	expectedMnt := workshop.Mount{Name: plug.Name, What: sourceDir, Where: "/project/training", Type: workshop.HostWorkshop}
	c.Assert(deviceSpec.Profile.Mounts, check.DeepEquals, map[string]workshop.Mount{plug.Name: expectedMnt})
}
