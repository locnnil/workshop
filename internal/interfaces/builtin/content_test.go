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
	"github.com/canonical/workshop/internal/workspacebackend"
	"gopkg.in/check.v1"
)

type ContentSuite struct {
	iface interfaces.Interface
}

func Test(t *testing.T) {
	check.TestingT(t)
}

var _ = check.Suite(&ContentSuite{
	iface: builtin.MustInterface("content"),
})

func (s *ContentSuite) TestName(c *check.C) {
	c.Assert(s.iface.Name(), check.Equals, "content")
}

func (s *ContentSuite) TestSanitizeSlotSimple(c *check.C) {
	const mockSdkYaml = `name: content-slot-sdk
base: ubuntu@22.04
slots:
 content-slot:
  interface: content
`
	info := sdk.MockInfo(c, mockSdkYaml, sdk.Setup{})
	slot := info.Slots["content-slot"]
	c.Assert(interfaces.BeforePrepareSlot(s.iface, slot), check.IsNil)
}

func (s *ContentSuite) TestSanitizePlugSimple(c *check.C) {
	const mockSdkYaml = `name: content-slot-sdk
base: ubuntu@22.04
plugs:
 content-plug:
  interface: content
  target: import
`
	info := sdk.MockInfo(c, mockSdkYaml, sdk.Setup{})
	plug := info.Plugs["content-plug"]
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.IsNil)
}

func (s *ContentSuite) TestSanitizePlugSimpleNoTarget(c *check.C) {
	const mockSdkYaml = `name: content-slot-sdk
base: ubuntu@22.04
plugs:
 content-plug:
  interface: content
  content: mycont
`
	info := sdk.MockInfo(c, mockSdkYaml, sdk.Setup{})
	plug := info.Plugs["content-plug"]
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.ErrorMatches, "content plug must contain target path")
}

func (s *ContentSuite) TestSanitizePlugSimpleTargetRelative(c *check.C) {
	const mockSdkYaml = `name: content-slot-sdk
base: ubuntu@22.04
plugs:
 content-plug:
  interface: content
  content: mycont
  target: ../foo
`
	info := sdk.MockInfo(c, mockSdkYaml, sdk.Setup{})
	plug := info.Plugs["content-plug"]
	c.Assert(interfaces.BeforePreparePlug(s.iface, plug), check.ErrorMatches, "content interface path is not clean:.*")
}

func (s *ContentSuite) TestInterfaces(c *check.C) {
	c.Check(builtin.Interfaces(), testutil.DeepContains, s.iface)
}

func (s *ContentSuite) TestContentInterface(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
 content:
  target: /project/training
`, sdk.Setup{Workshop: "ws"}, "content")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	slot := builtin.MockSlot(c, `name: producer
base: ubuntu@22.04
slots:
 content:
`, sdk.Setup{Workshop: "ws"}, "content")
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
	}, &workspacebackend.LookupUsername)
	defer restore()

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.IsNil)

	// Analyze the mount specification.
	expectedMnt := []*workspacebackend.WorkspaceDevice{{
		Name: plug.Name,
		Properties: map[string]string{
			"type":   "disk",
			"source": filepath.Join(homeDir, "/.local/share/workshop/project/content/ws_consumer_content.sdk"),
			"path":   "/project/training",
		},
	}}
	c.Assert(deviceSpec.DeviceEntries(), check.DeepEquals, expectedMnt)

}
