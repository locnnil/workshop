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
	"github.com/canonical/workshop/internal/osutil/sys"
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

var knownPlugAttributes = []string{"workshop-target", "mode", "uid", "gid", "read-only"}
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

	if !filepath.IsAbs(target) {
		return fmt.Errorf(`mount plug "workshop-target" must be absolute: %q`, target)
	}
	if filepath.Clean(target) != target {
		return fmt.Errorf(`mount plug "workshop-target" is not clean: %q`, target)
	}

	if _, err := parseBool(plug.Attrs, "read-only", false); err != nil {
		return err
	}

	var fallbackUid, fallbackGid int64
	for _, prefix := range []string{"/home/workshop", "/project", "/run/user/1000"} {
		if target == prefix || strings.HasPrefix(target, prefix+string(filepath.Separator)) {
			fallbackUid = workshop.Uid
			fallbackGid = workshop.Gid
			break
		}
	}

	uid, err := parseInt(plug.Attrs, "uid", fallbackUid)
	if err != nil {
		return err
	}
	if uid < 0 || uid >= sys.FlagID {
		return fmt.Errorf(`invalid value %v in key "uid" for mount interface plug: must be between 0 and %#x`, uid, sys.FlagID)
	}

	gid, err := parseInt(plug.Attrs, "gid", fallbackGid)
	if err != nil {
		return err
	}
	if gid < 0 || gid >= sys.FlagID {
		return fmt.Errorf(`invalid value %v in key "gid" for mount interface plug: must be between 0 and %#x`, gid, sys.FlagID)
	}

	fallbackMode := os.ModePerm &^ workshop.NormalUmask
	if uid == 0 {
		fallbackMode = os.ModePerm &^ workshop.RootUmask
	}
	mode, err := parseInt(plug.Attrs, "mode", int64(fallbackMode))
	if err != nil {
		return err
	}
	if mode < 0 || uint64(mode)&^uint64(os.ModePerm) != 0 {
		return fmt.Errorf(`invalid value %#o in key "mode" for mount interface plug: permissions limited to %#o`, mode, os.ModePerm)
	}

	return nil
}

func parseBool(attrs map[string]interface{}, key string, fallback bool) (bool, error) {
	object, ok := attrs[key]
	if !ok {
		attrs[key] = fallback
		return fallback, nil
	}

	switch value := object.(type) {
	case bool:
		return value, nil
	case string:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return false, fmt.Errorf(`unknown value %q in key %q for mount interface plug. Accepted values are 'true' or 'false'. String representations (e.g., '"true"') are also permitted`, value, key)
		}
		attrs[key] = parsed
		return parsed, nil
	default:
		return false, fmt.Errorf(`unknown value type %T in key %q for mount interface plug. Accepted types are 'bool' or 'string'`, object, key)
	}
}

func parseInt(attrs map[string]interface{}, key string, fallback int64) (int64, error) {
	object, ok := attrs[key]
	if !ok {
		attrs[key] = fallback
		return fallback, nil
	}

	switch value := object.(type) {
	case int64:
		return value, nil
	case string:
		parsed, err := strconv.ParseInt(value, 0, 64)
		if err != nil {
			return 0, fmt.Errorf(`unknown value %q in key %q for mount interface plug. Accepted types are 'int64'`, value, key)
		}
		attrs[key] = parsed
		return parsed, nil
	default:
		return 0, fmt.Errorf(`unknown value type %T in key %q for mount interface plug. Accepted types are 'int64' or 'string'`, object, key)
	}
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
		return fmt.Errorf(`mount slot "workshop-source" must be absolute: %q`, path)
	}
	if filepath.Clean(path) != path {
		return fmt.Errorf(`mount slot "workshop-source" is not clean: %q`, path)
	}

	slot.Attrs["workshop-source"] = path
	return nil
}

func (iface *mountInterface) setPlugAttrs(mount *workshop.Mount, plug *interfaces.ConnectedPlug) {
	_ = plug.Attr("workshop-target", &mount.Where)
	mount.MakeWhere = true

	var value int64
	_ = plug.Attr("mode", &value)
	mount.Mode = os.FileMode(value)

	_ = plug.Attr("uid", &value)
	mount.Owner = sys.UserID(value)

	_ = plug.Attr("gid", &value)
	mount.Group = sys.GroupID(value)

	_ = plug.Attr("read-only", &mount.ReadOnly)
}

func (iface *mountInterface) setRegularSlotAttrs(mount *workshop.Mount, slot *interfaces.ConnectedSlot) {
	mount.Type = workshop.WorkshopWorkshop

	_ = slot.Attr("workshop-source", &mount.What)
}

func (iface *mountInterface) setSystemSlotAttrs(mount *workshop.Mount, spec *lxd_device.Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) {
	mount.Type = workshop.HostWorkshop

	if err := slot.Attr("host-source", &mount.What); err == nil {
		return
	}

	// default dir: <sdk>/<plug>
	userDataDir := workshop.UserDataRootDir(spec.User.HomeDir, spec.Environment)
	mount.What = workshop.SdkMountHostSource(userDataDir, slot.Sdk().ProjectId, slot.Sdk().Workshop, plug.Sdk().Name, plug.Name())
	mount.MakeWhat = true
}

func (iface *mountInterface) AutoConnect(plug *sdk.PlugInfo, slot *sdk.SlotInfo) bool {
	// allow what declarations allowed
	return true
}

// Interactions with the mount backend.
func (iface *mountInterface) MountConnectedPlug(spec *lxd_device.Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error {
	mount := workshop.Mount{Name: plug.Name()}
	iface.setPlugAttrs(&mount, plug)
	if slot.Sdk().Type == sdk.System {
		iface.setSystemSlotAttrs(&mount, spec, plug, slot)
	} else {
		iface.setRegularSlotAttrs(&mount, slot)
	}

	return spec.AddMountEntry(mount)
}

func init() {
	registerIface(&mountInterface{})
}
