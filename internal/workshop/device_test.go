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

package workshop_test

import (
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/workshop"
)

// deviceSuite covers the default devices wired into every workshop.
type deviceSuite struct {
	restoreSocketPath string
}

var _ = check.Suite(&deviceSuite{})

func (s *deviceSuite) SetUpTest(c *check.C) {
	s.restoreSocketPath = dirs.SocketPath
}

func (s *deviceSuite) TearDownTest(c *check.C) {
	dirs.SocketPath = s.restoreSocketPath
}

func (s *deviceSuite) socketProxy(c *check.C) workshop.ProxyEntry {
	_, proxies := workshop.DefaultDevices("proj-id", "ws")
	for _, p := range proxies {
		if p.Name == "workshop.socket" {
			return p
		}
	}
	c.Fatalf("no workshop.socket proxy in default devices")
	return workshop.ProxyEntry{}
}

// The daemon socket must always be exposed inside the workshop at a fixed
// path, regardless of the daemon's host socket name. Deriving the in-workshop
// path from the host socket (as a non-default WORKSHOP_SOCKET, e.g. under
// "go tool try") used to leave workshopctl and hooks looking for a socket the
// proxy never created.
func (s *deviceSuite) TestSocketProxyListensOnFixedWorkshopPath(c *check.C) {
	dirs.SocketPath = "/tmp/workshop-7f3c9a.sock"

	proxy := s.socketProxy(c)
	c.Check(proxy.Direction, check.Equals, workshop.WorkshopToHost)
	c.Check(proxy.Listen.Address, check.Equals, dirs.WorkshopSocketPath+".untrusted")
	c.Check(proxy.Connect.Address, check.Equals, dirs.SocketPath+".untrusted")
}

// The in-workshop listen path is independent of the daemon's host socket
// name: two daemons with different sockets expose the socket at the same
// fixed path inside their workshops.
func (s *deviceSuite) TestSocketProxyListenPathIsStable(c *check.C) {
	dirs.SocketPath = "/var/lib/workshop/workshop.socket"
	defaultListen := s.socketProxy(c).Listen.Address

	dirs.SocketPath = "/tmp/workshop-7f3c9a.sock"
	tryListen := s.socketProxy(c).Listen.Address

	c.Check(tryListen, check.Equals, defaultListen)
}
