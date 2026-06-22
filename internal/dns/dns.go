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

package dns

import (
	"context"
	"net/netip"
)

// Resolver represents the host's DNS resolver.
type Resolver interface {
	// ConfigureDNS configures DNS server addresses for the given interface,
	// and ensures they are used to query hosts in the given domain.
	ConfigureDNS(ctx context.Context, iface string, addrs []netip.Addr, domain string) error
}

type FakeResolver struct {
	ConfigureDNSCalls    []ConfigureDNSCall
	ConfigureDNSCallback func(ctx context.Context, iface string, addrs []netip.Addr, domain string) error
}

type ConfigureDNSCall struct {
	Interface string
	Addrs     []netip.Addr
	Domain    string
}

func (f *FakeResolver) ConfigureDNS(ctx context.Context, iface string, addrs []netip.Addr, domain string) error {
	f.ConfigureDNSCalls = append(f.ConfigureDNSCalls, ConfigureDNSCall{Interface: iface, Addrs: addrs, Domain: domain})
	if f.ConfigureDNSCallback != nil {
		return f.ConfigureDNSCallback(ctx, iface, addrs, domain)
	}
	return nil
}
