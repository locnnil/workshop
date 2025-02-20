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
		desktop.Wayland = &workshop.ProxyEntry{
			Name: plug.Sdk().Name + "-" + "wayland",
			Connect: workshop.ProxyTarget{
				Address:  filepath.Join(xdg, wayland),
				Protocol: "unix",
			},
			Listen: workshop.ProxyTarget{
				Address:  filepath.Join("/run/user", workshop.User.Uid, wayland),
				Protocol: "unix",
			},
			Direction: workshop.WorkshopToHost,
		}
	}

	// We pass through the X11 socket regardless of whether XAUTHORITY is present
	// on the host. This then gives users the option to modify their xhost
	// settings to allow connections from the container and container user.
	if display != "" {
		proxyTarget := workshop.ProxyTarget{
			Address:  filepath.Join("/tmp/.X11-unix", "X"+strings.TrimPrefix(display, ":")),
			Protocol: "unix",
		}
		desktop.X11 = &workshop.ProxyEntry{
			Name:      plug.Sdk().Name + "-" + "x11",
			Connect:   proxyTarget,
			Listen:    proxyTarget,
			Direction: workshop.WorkshopToHost,
		}
	}

	// We mount the Xauthority inside a parent folder to ensure that the mounted
	// cookie is updated when the host cookie changes (ie. reboot).
	// https://discuss.linuxcontainers.org/t/mount-single-file/17975
	workshopdXauth := filepath.Join(dirs.WorkshopdRunDir, spec.User.Uid, "Xauthority")
	xauth := env["XAUTHORITY"]
	if xauth != "" {
		m := workshop.Mount{}
		m.Name = plug.Sdk().Name + "-" + "xauth"
		m.Type = workshop.HostWorkshop
		m.What = workshopdXauth
		m.Where = filepath.Join(dirs.WorkshopRunDir, "Xauthority")
		spec.AddMountEntry(m)
	}

	return spec.SetDesktop(desktop)
}

func init() {
	registerIface(&desktopInterface{})
}
