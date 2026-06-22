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

package lxdbackend

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	lxd "github.com/canonical/lxd/client"

	"github.com/canonical/workshop/internal/workshop"
)

type networkManager struct{}

func NewNetworkManager() workshop.NetworkManager {
	return &networkManager{}
}

func (*networkManager) InterfaceAddrs(ctx context.Context, name string) ([]netip.Addr, error) {
	conn, err := lxd.ConnectLXDUnixWithContext(ctx, "", nil)
	if err != nil {
		return nil, ErrorLxdBackend(err)
	}
	defer conn.Disconnect()

	network, _, err := conn.GetNetwork(name)
	if err != nil {
		return nil, fmt.Errorf("cannot get network %q: %w", name, err)
	}

	subnets := []string{network.Config["ipv4.address"], network.Config["ipv6.address"]}
	addrs := make([]netip.Addr, 0, len(subnets))
	for _, s := range subnets {
		if s == "" || s == "none" {
			continue
		}
		addr, err := parsePrefix(s)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, addr)
	}
	return addrs, nil
}

// parsePrefix extracts only the IP address from a CIDR block. We avoid
// netip.ParsePrefix because LXD uses the more relaxed net.ParseCIDR.
func parsePrefix(address string) (netip.Addr, error) {
	idx := strings.LastIndexByte(address, '/')
	if idx < 0 {
		prefix, err := netip.ParsePrefix(address)
		return prefix.Addr(), err
	}
	return netip.ParseAddr(address[:idx])
}
