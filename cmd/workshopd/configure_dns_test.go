// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/dns"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	"github.com/canonical/workshop/internal/workshop/fakebackend"
)

func Test(t *testing.T) { check.TestingT(t) }

type configureDNSSuite struct {
	networkManager *fakebackend.FakeNetworkManager
	resolver       *dns.FakeResolver

	restoreNetworkManager func()
	restoreResolver       func()
}

var _ = check.Suite(&configureDNSSuite{})

func (s *configureDNSSuite) SetUpTest(*check.C) {
	s.networkManager = &fakebackend.FakeNetworkManager{
		Addrs: []netip.Addr{netip.MustParseAddr("10.42.42.1")},
	}
	s.resolver = &dns.FakeResolver{}

	s.restoreNetworkManager = testutil.FakeFunc(func() workshop.NetworkManager {
		return s.networkManager
	}, &networkManager)
	s.restoreResolver = testutil.FakeFunc(func() dns.Resolver {
		return s.resolver
	}, &resolver)
}

func (s *configureDNSSuite) TearDownTest(*check.C) {
	s.restoreResolver()
	s.restoreNetworkManager()
}

func (s *configureDNSSuite) TestConfigureOK(c *check.C) {
	cmd := (&cmdConfigureDNS{}).Command()
	cmd.SetArgs([]string{"workshopbr0", "wp"})
	err := cmd.Execute()
	c.Assert(err, check.IsNil)

	want := []dns.ConfigureDNSCall{{
		Interface: "workshopbr0",
		Addrs:     s.networkManager.Addrs,
		Domain:    "wp",
	}}
	c.Check(s.resolver.ConfigureDNSCalls, check.DeepEquals, want)
}

func (s *configureDNSSuite) TestAddrsError(c *check.C) {
	s.networkManager.Addrs = nil
	s.networkManager.Err = errors.New("no such interface")

	cmd := (&cmdConfigureDNS{}).Command()
	cmd.SetArgs([]string{"workshopbr0", "wp"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.Execute()
	c.Assert(err, check.ErrorMatches, "no such interface")
}

func (s *configureDNSSuite) TestNoAddrs(c *check.C) {
	s.networkManager.Addrs = nil

	cmd := (&cmdConfigureDNS{}).Command()
	cmd.SetArgs([]string{"workshopbr0", "wp"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.Execute()
	c.Assert(err, check.ErrorMatches, `cannot find network address for "workshopbr0"`)
}

func (s *configureDNSSuite) TestConfigureError(c *check.C) {
	s.resolver.ConfigureDNSCallback = func(ctx context.Context, iface string, addrs []netip.Addr, domain string) error {
		return errors.New("invalid domain")
	}

	cmd := (&cmdConfigureDNS{}).Command()
	cmd.SetArgs([]string{"workshopbr0", "wp"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.Execute()
	c.Assert(err, check.ErrorMatches, "invalid domain")
}
