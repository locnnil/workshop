// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016-2018 Canonical Ltd
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

package policy_test

import (
	"fmt"
	"testing"

	"gopkg.in/check.v1"
	. "gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/asserts"
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/interfaces/policy"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
)

type baseDeclSuite struct {
	baseDecl        *asserts.BaseDeclaration
	restoreSanitize func()
}

var _ = Suite(&baseDeclSuite{})

func Test(t *testing.T) {
	check.TestingT(t)
}

func (s *baseDeclSuite) SetUpSuite(c *C) {
	s.restoreSanitize = sdk.MockSanitizePlugsSlots(func(sdkInfo *sdk.Info) {})
	s.baseDecl = asserts.BuiltinBaseDeclaration()
}

func (s *baseDeclSuite) TearDownSuite(c *C) {
	s.restoreSanitize()
}

func (s *baseDeclSuite) connectCand(c *C, iface, slotYaml, plugYaml string) *policy.ConnectCandidate {
	if slotYaml == "" {
		slotYaml = fmt.Sprintf(`name: slot-sdk
base: ubuntu@22.04
slots:
  %s:
`, iface)
	}
	if plugYaml == "" {
		plugYaml = fmt.Sprintf(`name: plug-sdk
base: ubuntu@22.04
plugs:
  %s:
`, iface)
	}
	slotSdk := sdk.MockInfo(c, slotYaml, "ws", sdk.Setup{})
	plugSdk := sdk.MockInfo(c, plugYaml, "ws", sdk.Setup{})
	return &policy.ConnectCandidate{
		Plug:            interfaces.NewConnectedPlug(plugSdk.Plugs[iface], nil, nil),
		Slot:            interfaces.NewConnectedSlot(slotSdk.Slots[iface], nil, nil),
		BaseDeclaration: s.baseDecl,
	}
}

func (s *baseDeclSuite) installSlotCand(c *C, iface string, sdkType sdk.Type, yaml string) *policy.InstallCandidate {
	if yaml == "" {
		yaml = fmt.Sprintf(`name: install-slot-sdk
base: ubuntu@22.04
type: %s
slots:
  %s:
`, sdkType, iface)
	}
	sdk := sdk.MockInfo(c, yaml, "ws", sdk.Setup{})
	return &policy.InstallCandidate{
		Sdk:             sdk,
		BaseDeclaration: s.baseDecl,
	}
}

func (s *baseDeclSuite) installPlugCand(c *C, iface string, sdkType sdk.Type, yaml string) *policy.InstallCandidate {
	if yaml == "" {
		yaml = fmt.Sprintf(`name: install-plug-sdk
base: ubuntu@22.04
type: %s
plugs:
  %s:
`, sdkType, iface)
	}
	sdk := sdk.MockInfo(c, yaml, "ws", sdk.Setup{})
	return &policy.InstallCandidate{
		Sdk:             sdk,
		BaseDeclaration: s.baseDecl,
	}
}

func (s *baseDeclSuite) TestContentAutoConnection(c *C) {
	slotYaml := fmt.Sprintf(`name: slot-sdk
base: ubuntu@22.04
slots:
    %s:
`, "content")

	cand := s.connectCand(c, "content", slotYaml, "")
	arity, err := cand.CheckAutoConnect()
	c.Check(err, check.IsNil)
	c.Check(arity.SlotsPerPlugAny(), check.Equals, false)
}

func (s *baseDeclSuite) TestAutoConnectPlugSlot(c *C) {
	all := builtin.Interfaces()

	for _, iface := range all {
		c.Check(iface.AutoConnect(nil, nil), Equals, true)
	}
}

func (s *baseDeclSuite) TestContentSlotInstallation(c *C) {
	// test content specially
	ic := s.installSlotCand(c, "content", sdk.Sdk, ``)
	err := ic.Check()
	c.Assert(err, Not(IsNil))
	c.Assert(err, ErrorMatches, "installation not allowed by \"content\" slot rule of interface \"content\"")

	ic = s.installSlotCand(c, "content", sdk.Core, ``)
	err = ic.Check()
	c.Assert(err, IsNil)
}

func (s *baseDeclSuite) TestComposeBaseDeclaration(c *C) {
	decl, err := policy.ComposeBaseDeclaration(nil)
	c.Assert(err, IsNil)
	c.Assert(string(decl), testutil.Contains, `
type: base-declaration
authority-id: canonical
series: 1
revision: 0
`)
}

func (s *baseDeclSuite) TestDoesNotPanic(c *C) {
	// In case there are any issues in the actual interfaces we'd get a panic
	// on startup. This test prevents this from happing unnoticed.
	_, err := policy.ComposeBaseDeclaration(builtin.Interfaces())
	c.Assert(err, IsNil)
}
