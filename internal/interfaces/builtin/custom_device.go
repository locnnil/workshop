// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2026 Canonical Ltd
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
	"slices"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/lxd_device"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
)

const customDeviceSummary = `allows sharing custom devices with SDKs`

const customDeviceBaseDeclarationSlots = `
  custom-device:
    allow-installation:
      slot-sdk-type:
        - system
      slot-names:
        - $INTERFACE
    allow-connection: true
    deny-auto-connection: true
`

const customDeviceBaseDeclarationPlugs = `
  custom-device:
    allow-installation:
      plug-sdk-type:
        - regular
    allow-connection: true
    deny-auto-connection: true
`

var knownCustomDeviceAttributes = []string{"subsystem", "files"}

// attrReader is implemented by both *sdk.PlugInfo and *interfaces.ConnectedPlug.
type attrReader interface {
	Attr(key string, val any) error
}

// customDeviceFiles returns the optional "files" allowlist for the plug. It
// returns an empty slice when the attribute is absent.
func customDeviceFiles(plug attrReader) ([]string, error) {
	var files []string
	var attrErr *sdk.AttributeNotFoundError
	if err := plug.Attr("files", &files); err != nil && !errors.As(err, &attrErr) {
		return nil, err
	}
	return files, nil
}

type customDeviceInterface struct{}

func (iface *customDeviceInterface) Name() string {
	return "custom-device"
}

func (iface *customDeviceInterface) StaticInfo() interfaces.StaticInfo {
	return interfaces.StaticInfo{
		Summary:              customDeviceSummary,
		BaseDeclarationPlugs: customDeviceBaseDeclarationPlugs,
		BaseDeclarationSlots: customDeviceBaseDeclarationSlots,
		AffectsPlugOnRefresh: true,
	}
}

func (iface *customDeviceInterface) BeforePreparePlug(plug *sdk.PlugInfo) error {
	for name := range plug.Attrs {
		if !slices.Contains(knownCustomDeviceAttributes, name) {
			return fmt.Errorf("unknown attribute for custom-device interface plug: %q", name)
		}
	}

	var subsystem string
	if err := plug.Attr("subsystem", &subsystem); err != nil {
		return err
	}
	if subsystem == "" {
		return fmt.Errorf(`custom-device plug "subsystem" is empty`)
	}

	files, err := customDeviceFiles(plug)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file == "" {
			return fmt.Errorf(`custom-device plug "files" entry is empty`)
		}
		if !filepath.IsAbs(filepath.Clean(file)) {
			return fmt.Errorf(`custom-device plug "files" entry %q is not an absolute path`, file)
		}
	}

	return nil
}

func (iface *customDeviceInterface) AutoConnect(plug *sdk.PlugInfo, slot *sdk.SlotInfo) bool {
	// allow what declarations allowed
	return true
}

func (iface *customDeviceInterface) MountConnectedPlug(spec *lxd_device.Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error {
	var subsystem string
	if err := plug.Attr("subsystem", &subsystem); err != nil {
		return err
	}
	files, err := customDeviceFiles(plug)
	if err != nil {
		return err
	}
	return spec.AddCustomDevice(workshop.CustomDevice{Name: plug.Name(), Subsystem: subsystem, Files: files})
}

func init() {
	registerIface(&customDeviceInterface{})
}
