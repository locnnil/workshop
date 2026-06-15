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

//go:build integration

package lxdbackend_integration_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"gopkg.in/check.v1"

	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
)

type networkSuite struct {
	ctx   context.Context
	iface string
}

var _ = check.Suite(&networkSuite{})

func (s *networkSuite) SetUpTest(c *check.C) {
	s.ctx = context.Background()
	s.iface = "testlxdbr0"
}

func (s *networkSuite) TearDownTest(c *check.C) {
}

func (s *networkSuite) TestInterfaceAddrsOK(c *check.C) {
	conn, err := lxd.ConnectLXDUnixWithContext(s.ctx, "", nil)
	c.Assert(err, check.IsNil)
	defer conn.Disconnect()

	op, err := conn.CreateNetwork(api.NetworksPost{
		Name: s.iface,
		Type: "bridge",
	})
	c.Assert(err, check.IsNil)
	c.Assert(op.Wait(), check.IsNil)
	defer func() {
		op1, err1 := conn.DeleteNetwork(s.iface)
		if c.Check(err1, check.IsNil) {
			c.Check(op1.Wait(), check.IsNil)
		}
	}()

	addrs, err := lxdbackend.NewNetworkManager().InterfaceAddrs(s.ctx, s.iface)
	c.Assert(err, check.IsNil)
	c.Check(addrs, check.Not(check.HasLen), 0)
}

var oneshotServiceTemplate = `
[Unit]
After=sys-subsystem-net-devices-testlxdbr0.device

[Service]
Type=oneshot
ExecStart=/usr/bin/kill --signal=USR1 %v

[Install]
WantedBy=sys-subsystem-net-devices-testlxdbr0.device
`[1:]

func (s *networkSuite) TestInterfaceAddrsAvailableImmediately(c *check.C) {
	sigusr1 := make(chan os.Signal, 1)
	signal.Notify(sigusr1, syscall.SIGUSR1)
	defer signal.Stop(sigusr1)

	pid := os.Getpid()
	name := fmt.Sprintf("workshop-signal-test%v.service", pid)
	unit := fmt.Appendf(nil, oneshotServiceTemplate, pid)

	err := os.WriteFile("/run/systemd/system/"+name, unit, 0644)
	c.Assert(err, check.IsNil)
	defer func() {
		err1 := os.Remove("/run/systemd/system/" + name)
		c.Check(err1, check.IsNil)
		_, err1 = exec.Command("systemctl", "daemon-reload").Output()
		c.Assert(err1, check.IsNil)
	}()

	_, err = exec.Command("systemctl", "daemon-reload").Output()
	c.Assert(err, check.IsNil)
	_, err = exec.Command("systemctl", "enable", name).Output()
	c.Assert(err, check.IsNil)
	defer func() {
		_, err1 := exec.Command("systemctl", "disable", name).Output()
		c.Check(err1, check.IsNil)
	}()

	var wg sync.WaitGroup
	wg.Go(func() {
		<-sigusr1
		addrs, err := lxdbackend.NewNetworkManager().InterfaceAddrs(s.ctx, s.iface)
		c.Assert(err, check.IsNil)
		c.Check(addrs, check.Not(check.HasLen), 0)
	})

	conn, err := lxd.ConnectLXDUnixWithContext(s.ctx, "", nil)
	c.Assert(err, check.IsNil)
	defer conn.Disconnect()

	op, err := conn.CreateNetwork(api.NetworksPost{
		Name: s.iface,
		Type: "bridge",
	})
	c.Assert(err, check.IsNil)
	c.Assert(op.Wait(), check.IsNil)
	defer func() {
		op1, err1 := conn.DeleteNetwork(s.iface)
		if c.Check(err1, check.IsNil) {
			c.Check(op1.Wait(), check.IsNil)
		}
	}()

	wg.Wait()
}
