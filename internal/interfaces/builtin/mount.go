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
	"slices"
	"strconv"
	"strings"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/lxd_device"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
)

const mountSummary = `allows sharing host code and data with SDKs`

const mountBaseDeclarationSlots = `
  mount:
    allow-installation:
      -
        slot-sdk-type:
          - system
        slot-names:
          - $INTERFACE
      -
        slot-sdk-type:
          - regular
    allow-connection: true
    allow-auto-connection: true
`

const mountBaseDeclarationPlugs = `
  mount:
    allow-installation:
      plug-sdk-type:
        - regular
    allow-connection: true
    allow-auto-connection:
      -
        slot-sdk-type:
          - system
      -
        plug-attributes:
          auto-explicit: true
`

var knownPlugAttributes = []string{"workshop-target", "read-only"}
var knownSlotAttributes = []string{"workshop-source"}

// mountInterface allows sharing content between sdks
type mountInterface struct{}

func (iface *mountInterface) Name() string {
	return "mount"
}

func (iface *mountInterface) StaticInfo() interfaces.StaticInfo {
	return interfaces.StaticInfo{
		Summary:              mountSummary,
		BaseDeclarationPlugs: mountBaseDeclarationPlugs,
		BaseDeclarationSlots: mountBaseDeclarationSlots,
		AffectsPlugOnRefresh: true,
	}
}

func cleanSubPath(path string) bool {
	return filepath.Clean(path) == path && path != ".." && !strings.HasPrefix(path, "../")
}

func validatePath(path string) error {
	if ok := cleanSubPath(path); !ok {
		return fmt.Errorf("mount interface path is not clean: %q", path)
	}
	return nil
}

func (iface *mountInterface) BeforePreparePlug(plug *sdk.PlugInfo) error {
	for name := range plug.Attrs {
		if !slices.Contains(knownPlugAttributes, name) {
			return fmt.Errorf(`unknown attribute for mount interface plug: %q`, name)
		}
	}

	target, ok := plug.Attrs["workshop-target"].(string)
	if !ok || len(target) == 0 {
		return fmt.Errorf("mount plug must contain target path")
	}

	if err := validatePath(target); err != nil {
		return err
	}

	ro, ok := plug.Attrs["read-only"]
	if !ok {
		ro = false
	}

	switch ro := ro.(type) {
	case bool:
		plug.Attrs["read-only"] = ro
	case string:
		roBool, err := strconv.ParseBool(ro)
		if err != nil {
			return fmt.Errorf(`unknown value %q in key "read-only" for mount interface plug. Accepted values are 'true' or 'false'. String representations (e.g., '"true"') are also permitted`, ro)
		}
		plug.Attrs["read-only"] = roBool
	default:
		return fmt.Errorf(`unknown value type %T in key "read-only" for mount interface plug. Accepted types are 'bool' or 'string'`, ro)
	}
	return nil
}

func (iface *mountInterface) BeforePrepareSlot(slot *sdk.SlotInfo) error {
	if slot.Sdk.Type == sdk.System {
		for name := range slot.Attrs {
			return fmt.Errorf(`unknown attribute for system mount interface slot: %q`, name)
		}
		return nil
	}

	for name := range slot.Attrs {
		if !slices.Contains(knownSlotAttributes, name) {
			return fmt.Errorf(`unknown attribute for mount interface slot: %q`, name)
		}
	}
	source, exist := slot.Attrs["workshop-source"]
	if !exist {
		return fmt.Errorf("mount slot must contain source path")
	}
	path, ok := source.(string)
	if !ok {
		return fmt.Errorf(`mount slot "workshop-source" is not a string (found %T)`, source)
	}

	var err error
	path = os.Expand(path, func(s string) string {
		switch s {
		case "SDK":
			return sdk.SdkDir(slot.Sdk.Name)
		case "$":
			// Unescape $$ -> $.
			return "$"
		default:
			err = fmt.Errorf("unexpected variable %q", s)
			return ""
		}
	})
	if err != nil {
		return err
	}

	if !filepath.IsAbs(path) {
		return fmt.Errorf(`mount slot "workshop-source" must be absolute`)
	}
	return nil
}

func (iface *mountInterface) target(attrs interfaces.Attrer) string {
	var target string

	if err := attrs.Attr("workshop-target", &target); err == nil {
		return target
	}
	return ""
}

func (iface *mountInterface) readOnly(attrs interfaces.Attrer) bool {
	var ro bool
	attrs.Attr("read-only", &ro)
	return ro
}

func (iface *mountInterface) workshopSource(slot *interfaces.ConnectedSlot) (string, error) {
	var source string
	err := slot.Attr("workshop-source", &source)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(source, "$SDK/") {
		return strings.Replace(source, "$SDK", sdk.SdkDir(slot.Sdk().Name), 1), nil
	}
	return source, nil
}

func (iface *mountInterface) hostSource(spec *lxd_device.Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) (string, bool) {
	var source string
	if err := slot.Attr("host-source", &source); err == nil {
		return source, false
	}

	// default dir: <sdk>/<plug>
	userDataDir := workshop.UserDataRootDir(spec.User.HomeDir, spec.Environment)
	source = workshop.SdkMountHostSource(userDataDir, slot.Sdk().ProjectId, slot.Sdk().Workshop, plug.Sdk().Name, plug.Name())
	return source, true
}

func (iface *mountInterface) AutoConnect(plug *sdk.PlugInfo, slot *sdk.SlotInfo) bool {
	// allow what declarations allowed
	return true
}

// Interactions with the mount backend.
func (iface *mountInterface) MountConnectedPlug(spec *lxd_device.Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error {
	if slot.Sdk().Type == sdk.System {
		source, auto := iface.hostSource(spec, plug, slot)
		return spec.AddMountEntry(workshop.Mount{
			Name:      plug.Name(),
			What:      source,
			Where:     iface.target(plug),
			MakeWhat:  auto,
			MakeWhere: true,
			Type:      workshop.HostWorkshop,
			ReadOnly:  iface.readOnly(plug),
		})
	}

	source, err := iface.workshopSource(slot)
	if err != nil {
		return err
	}
	return spec.AddMountEntry(workshop.Mount{
		Name:     plug.Name(),
		What:     source,
		Where:    iface.target(plug),
		Type:     workshop.WorkshopWorkshop,
		ReadOnly: iface.readOnly(plug),
	})
}

func init() {
	registerIface(&mountInterface{})
}
