// Copyright (c) 2014-2020 Canonical Ltd
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

package systemd

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/canonical/workshop/internal/osutil"
)

var notifySocket string

func FakeNotifySocket(socket string) func() {
	oldSocket := notifySocket
	notifySocket = socket
	return func() { notifySocket = oldSocket }
}

func init() {
	notifySocket = os.Getenv("NOTIFY_SOCKET")
	// Unset so that subprocesses don't try to notify systemd; this prevents
	// systemctl show-environment from spamming the workshopd journal. See
	// https://github.com/systemd/systemd/blob/v260/src/shared/main-func.c#L23.
	if err := os.Unsetenv("NOTIFY_SOCKET"); err != nil {
		panic(fmt.Errorf("cannot unset NOTIFY_SOCKET: %w", err))
	}
}

func SocketAvailable() bool {
	return notifySocket != "" && osutil.FileExists(notifySocket)
}

// SdNotify sends the given state string notification to systemd.
//
// inspired by libsystemd/sd-daemon/sd-daemon.c from the systemd source
func SdNotify(notifyState string) error {
	if notifyState == "" {
		return fmt.Errorf("cannot use empty notify state")
	}

	if notifySocket == "" {
		return fmt.Errorf("$NOTIFY_SOCKET not defined")
	}
	if !strings.HasPrefix(notifySocket, "@") && !strings.HasPrefix(notifySocket, "/") {
		return fmt.Errorf("cannot use $NOTIFY_SOCKET value: %q", notifySocket)
	}

	raddr := &net.UnixAddr{
		Name: notifySocket,
		Net:  "unixgram",
	}
	conn, err := net.DialUnix("unixgram", nil, raddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write([]byte(notifyState))
	return err
}
