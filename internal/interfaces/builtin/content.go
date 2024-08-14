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
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/device"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
)

const contentSummary = `allows sharing host code and data with SDKs`

const contentBaseDeclarationSlots = `
  content:
    allow-installation:
      slot-sdk-type:
        - host
    allow-connection: true
    allow-auto-connection:
      slot-names:
        - $INTERFACE
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

func (iface *contentInterface) BeforePrepareSlot(slot *sdk.SlotInfo) error {
	source, exist := slot.Attrs["source"]
	if !exist {
		// perfectly fine scenario for the default content slot
		return nil
	}
	path, ok := source.(string)
	if !ok {
		return fmt.Errorf(`content slot "source" is not a string (found %T)`, source)
	}
	if !filepath.IsLocal(path) {
		return fmt.Errorf(`content slot "source" must be within project subtree`)
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

func (iface *contentInterface) source(baseDir string, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) (string, error) {
	var source string
	err := slot.Attr("source", &source)
	if err == nil {
		return source, nil
	}
	if !errors.Is(err, sdk.AttributeNotFoundError{}) {
		return source, err
	}
	// default dir: <workshop>_<sdk>_plug.sdk
	return sdk.SdkContentSource(baseDir, slot.Sdk().ProjectId, slot.Sdk().Workshop, plug.Sdk().Name, plug.Name()), nil
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
	user, err := workshop.LookupUsername(spec.User())
	if err != nil {
		return err
	}

	source, err := iface.source(user.HomeDir, plug, slot)
	if err != nil {
		return err
	}

	spec.AddDeviceEntry(lxdbackend.Mount(plug.Name(), source, iface.target(plug)))
	return nil
}

func init() {
	registerIface(&contentInterface{})
}
