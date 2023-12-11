// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016 Canonical Ltd
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
	"os/user"
	"path/filepath"
	"strings"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/device"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshopbackend"
)

const contentSummary = `allows sharing host code and data with SDKs`

const contentBaseDeclarationSlots = `
  content:
    allow-installation:
      slot-type:
        - core
    allow-connection: true
    allow-auto-connection: true
`

// contentInterface allows sharing content between sdks
type contentInterface struct{}

func (iface *contentInterface) Name() string {
	return "content"
}

func (iface *contentInterface) StaticInfo() interfaces.StaticInfo {
	return interfaces.StaticInfo{
		Summary:              contentSummary,
		BaseDeclarationSlots: contentBaseDeclarationSlots,
		ImplicitOnCore:       true,
		AffectsPlugOnRefresh: true,
	}
}

func cleanSubPath(path string) bool {
	return filepath.Clean(path) == path && path != ".." && !strings.HasPrefix(path, "../")
}

func validatePath(path string) error {
	if ok := cleanSubPath(path); !ok {
		return fmt.Errorf("content interface path is not clean: %q", path)
	}
	return nil
}

func (iface *contentInterface) BeforePrepareSlot(slot *sdk.SlotInfo) error {
	return nil
}

func (iface *contentInterface) BeforePreparePlug(plug *sdk.PlugInfo) error {
	target, ok := plug.Attrs["target"].(string)
	if !ok || len(target) == 0 {
		return fmt.Errorf("content plug must contain target path")
	}
	if err := validatePath(target); err != nil {
		return err
	}

	return nil
}

func (iface *contentInterface) target(attrs interfaces.Attrer) string {
	var target string

	if err := attrs.Attr("target", &target); err == nil {
		return target
	}
	return ""
}

func (iface *contentInterface) source(user *user.User, plug *interfaces.ConnectedPlug) string {
	source := filepath.Join(user.HomeDir, ".local", "share", "workshop", "project", plug.Ref().ProjectId, "content",
		strings.Join([]string{plug.Sdk().Workshop, plug.Sdk().Name, plug.Name()}, "_")+".sdk")
	return source
}

func (iface *contentInterface) AutoConnect(plug *sdk.PlugInfo, slot *sdk.SlotInfo) bool {
	// allow what declarations allowed
	return true
}

func (iface *contentInterface) MountConnectedSlot(spec *device.Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error {
	return nil
}

// Interactions with the mount backend.
func (iface *contentInterface) MountConnectedPlug(spec *device.Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error {
	user, err := workshopbackend.LookupUsername(spec.User())
	if err != nil {
		return err
	}
	source := iface.source(user, plug)

	uid, gid, err := osutil.UidGid(user)
	if err != nil {
		return err
	}

	if err = osutil.MkdirAllChown(source, 0744, uid, gid); err != nil {
		return nil
	}
	return spec.AddDeviceEntry(workshopbackend.Mount(plug.Name(), source, iface.target(plug)))
}

func init() {
	registerIface(&contentInterface{})
}
