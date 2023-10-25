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
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/device"
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

func (iface *contentInterface) mount(attrs interfaces.Attrer) string {
	var source string

	if err := attrs.Attr("target", &source); err == nil {
		return source
	}
	return ""
}

func (iface *contentInterface) AutoConnect(plug *sdk.PlugInfo, slot *sdk.SlotInfo) bool {
	// allow what declarations allowed
	return true
}

// Interactions with the mount backend.
func (iface *contentInterface) MountConnectedPlug(spec *device.Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error {
	user, err := workshopbackend.LookupUsername(spec.User())
	if err != nil {
		return err
	}

	source := filepath.Join(user.HomeDir, ".local", "share", "workshop", "project", spec.ProjectId(), "content",
		strings.Join([]string{plug.Sdk().Workshop, plug.Sdk().Name, plug.Name()}, "_")+".sdk")

	if err = os.MkdirAll(source, 0744); err != nil {
		return err
	}

	uid, err := strconv.Atoi(user.Uid)
	if err != nil {
		return err
	}

	gid, err := strconv.Atoi(user.Gid)
	if err != nil {
		return err
	}

	if err = os.Chown(source, uid, gid); err != nil {
		return err
	}

	var entry = workshopbackend.WorkshopDevice{
		Name: plug.Name(),
		Properties: map[string]string{"type": "disk", "source": source,
			"path": iface.mount(plug)},
	}
	return spec.AddDeviceEntry(&entry)
}

func init() {
	registerIface(&contentInterface{})
}
