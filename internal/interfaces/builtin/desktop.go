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
	"path/filepath"
	"strings"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/lxd_device"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/systemd"
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

func (iface *desktopInterface) MountConnectedPlug(spec *lxd_device.Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error {
	env, err := systemd.UserEnvironment(spec.User)
	if err != nil {
		return err
	}

	xdg := env["XDG_RUNTIME_DIR"]
	if xdg == "" {
		return fmt.Errorf("XDG_RUNTIME_DIR is either empty or unset for user %q", spec.User.Username)
	}

	desktop := workshop.Desktop{}

	wayland := env["WAYLAND_DISPLAY"]
	display := env["DISPLAY"]

	if wayland == "" && display == "" {
		return fmt.Errorf("neither DISPLAY nor WAYLAND_DISPLAY are set for user %q", spec.User.Username)
	}

	if wayland != "" {
		// Add wayland to the profile string
		w := &desktop.Wayland
		w.Name = plug.Sdk().Name + "-" + "wayland"
		w.Connect = filepath.Join(xdg, wayland)
		w.Listen = filepath.Join("/run/user/1000/", wayland)
	}

	// We pass through the X11 socket regardless of whether XAUTHORITY is present
	// on the host. This then gives users the option to modify their xhost
	// settings to allow connections from the container and container user.
	if display != "" {
		// Add X11 to the profile string
		x := &desktop.X11
		x.Name = plug.Sdk().Name + "-" + "x11"
		x.Connect = filepath.Join("/tmp/.X11-unix", "X"+strings.TrimPrefix(display, ":"))
		x.Listen = x.Connect
	}

	workshopdXauth := filepath.Join(dirs.WorkshopdRunDir, spec.User.Uid, ".Xauthority")
	xauth := env["XAUTHORITY"]
	if xauth != "" {
		m := workshop.Mount{}
		m.Name = plug.Sdk().Name + "-" + "xauth"
		m.Type = 0
		m.What = workshopdXauth
		m.Where = filepath.Join(dirs.WorkshopRunDir, ".Xauthority")
		spec.AddMountEntry(m)
	}

	return spec.SetDesktop(&desktop)
}

func init() {
	registerIface(&desktopInterface{})
}
