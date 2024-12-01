// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2024 Canonical Ltd
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

package builtin

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/lxd_device"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
)

const desktopSummary = `allows SDKs to use the host's wayland compositor`

const desktopBaseDeclarationSlots = `
  desktop:
    allow-installation:
      slot-sdk-type:
        - system
      slot-names:
        - $INTERFACE
    allow-connection: true
    deny-auto-connection: true
`

const desktopDeclarationPlugs = `
  desktop:
    allow-installation:
      plug-sdk-type:
        - regular
      plug-names:
        - $INTERFACE
    allow-connection: true
    deny-auto-connection: true
`

type desktopInterface struct{}

func (iface *desktopInterface) Name() string {
	return "desktop"
}

func (iface *desktopInterface) StaticInfo() interfaces.StaticInfo {
	return interfaces.StaticInfo{
		Summary:              desktopSummary,
		BaseDeclarationPlugs: desktopDeclarationPlugs,
		BaseDeclarationSlots: desktopBaseDeclarationSlots,
		AffectsPlugOnRefresh: true,
	}
}

func (iface *desktopInterface) AutoConnect(plug *sdk.PlugInfo, slot *sdk.SlotInfo) bool {
	return true
}

func (iface *desktopInterface) MountConnectedPlug(
	spec *lxd_device.Specification,
	plug *interfaces.ConnectedPlug,
	slot *interfaces.ConnectedSlot,
) error {

	user, err := workshop.LookupUsername(spec.User())
	if err != nil {
		return err
	}

	uid, _, err := osutil.UidGid(user)
	if err != nil {
		return err
	}

	// Systemd is responsible for generating the "WAYLAND_DISPLAY" environment
	// variable. Because of this we must parse the environment as it's set by
	// systemd.
	cmd := exec.Command("sudo", "-E", "-u", spec.User(), "systemctl", "--user", "show-environment")
	// XDG_RUNTIME_DIR may not be set if a command invoked by sudo or
	// systemd-run; set it here to the default location. It is required for the
	// systemctl to work with --user. See:
	// https://unix.stackexchange.com/questions/346841/why-does-sudo-i-not-set-xdg-runtime-dir-for-the-target-user
	defaultXdg := filepath.Join(dirs.XdgRuntimeDirBase, strconv.FormatUint(uint64(uid), 10))
	cmd.Env = append(cmd.Env, "XDG_RUNTIME_DIR="+defaultXdg)
	out, errOut, err := osutil.RunCmd(cmd)
	if err != nil {
		return fmt.Errorf(string(errOut))
	}

	rawEnv := strings.FieldsFunc(string(out), func(r rune) bool { return r == '\n' })
	env, err := osutil.ParseEnvironment(rawEnv)
	if err != nil {
		return err
	}

	wayland, ok := env["WAYLAND_DISPLAY"]
	if !ok || wayland == "" {
		return fmt.Errorf("WAYLAND_DISPLAY is either empty or unset for user %q. Is this a Wayland session?", user.Username)
	}

	xdg, ok := env["XDG_RUNTIME_DIR"]
	if !ok || xdg == "" {
		return fmt.Errorf("XDG_RUNTIME_DIR is either empty or unset for user %q", user.Username)
	}

	name := plug.Sdk().Name + "-" + plug.Name()

	fromSocket := xdg + "/" + wayland
	// The container XDG_RUNTIME_DIR is always /run/user/1000 for the workshop
	// user.
	// Use the same WAYLAND_DISPLAY identifier as the host.
	toSocket := "/run/user/1000/" + wayland

	return spec.SetDesktop(workshop.Desktop{Name: name, Connect: fromSocket, Listen: toSocket})
}

func init() {
	registerIface(&desktopInterface{})
}
