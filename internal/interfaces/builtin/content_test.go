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
	"os/user"
	"path/filepath"
	"testing"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/interfaces/device"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
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
	iface: builtin.MustInterface("content"),
})

func (s *contentSuite) SetUpTest(c *check.C) {
	s.projectId = "42424242"
}

func (s *contentSuite) TestName(c *check.C) {
	c.Assert(s.iface.Name(), check.Equals, "content")
}

func (s *contentSuite) TestSanitizeSlotSimple(c *check.C) {
	const mockSdkYaml = `name: content-slot-sdk
base: ubuntu@22.04
slots:
 content-slot:
  interface: content
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
  interface: content
  target: import
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
  interface: content
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
  interface: content
  target: ../foo
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
  interface: content
  source: images/low-res
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
  interface: content
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
  interface: content
  source: /root
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	slot := info.Slots["content-slot"]
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.ErrorMatches, `content slot \"source\" must be within project subtree`)
}

func (s *contentSuite) TestSanitizeSlotNonLocalSourceFails(c *check.C) {
	const mockSdkYaml = `name: content-slot-sdk
base: ubuntu@22.04
slots:
 content-slot:
  interface: content
  source: ../../../../../../../../root/
`
	info := sdk.MockInfo(c, mockSdkYaml, s.projectId, "ws")
	slot := info.Slots["content-slot"]
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.ErrorMatches, `content slot \"source\" must be within project subtree`)
}

func (s *contentSuite) TestInterfaces(c *check.C) {
	c.Check(builtin.Interfaces(), testutil.DeepContains, s.iface)
}

func (s *contentSuite) TestContentInterface(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 content:
  target: /project/training
`, s.projectId, "ws", "consumer", "content")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	slot := builtin.MockSlot(c, `name: producer
base: ubuntu@22.04
slots:
 content:
`, s.projectId, "ws", "producer", "content")
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)
	deviceSpec := &device.Specification{}

	homeDir := c.MkDir()
	usr, err := user.Current()
	c.Assert(err, check.IsNil)

	restore := testutil.FakeFunc(func(name string) (*user.User, error) {
		u := &user.User{
			Name:     usr.Name,
			Username: usr.Name,
			Uid:      usr.Uid,
			Gid:      usr.Gid,
			HomeDir:  homeDir,
		}
		return u, nil
	}, &workshop.LookupUsername)
	defer restore()

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.IsNil)

	// Validate the mount specification.
	sourceDir := filepath.Join(homeDir, "/.local/share/workshop/project/42424242/content/ws_consumer_content.sdk")
	expectedMnt := lxdbackend.Mount(plug.Name, sourceDir, "/project/training")
	c.Assert(deviceSpec.DeviceEntries(), check.DeepEquals, []workshop.Device{expectedMnt})
}
