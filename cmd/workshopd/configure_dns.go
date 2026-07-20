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
	"fmt"

	"github.com/spf13/cobra"

	"github.com/canonical/workshop/internal/dns"
	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
)

var shortConfigureDNSHelp = "Configure bridge device with a route-only domain"
var longConfigureDNSHelp = `
This command configures the system DNS resolver to use a workshop bridge device
as the resolver for a particular top-level domain.

It currently supports systemd-resolved, setting the link-specific DNS address
to the given interface's IP address and configuring the given domain as a
routing-only domain for the link.
`

var (
	networkManager = lxdbackend.NewNetworkManager
	resolver       = dns.NewSystemdResolved
)

type cmdConfigureDNS struct{}

func (c *cmdConfigureDNS) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "configure-dns <INTERFACE> <DOMAIN>",
		Args:  cobra.ExactArgs(2),
		Short: shortConfigureDNSHelp,
		Long:  longConfigureDNSHelp,
		RunE:  c.Run,
	}
}

func (c *cmdConfigureDNS) Run(cmd *cobra.Command, av []string) error {
	ctx := cmd.Context()
	addrs, err := networkManager().InterfaceAddrs(ctx, av[0])
	if err != nil {
		return err
	}
	if len(addrs) == 0 {
		return fmt.Errorf("cannot find network address for %q", av[0])
	}
	return resolver().ConfigureDNS(ctx, av[0], addrs, av[1])
}
