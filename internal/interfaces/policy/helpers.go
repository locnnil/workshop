// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016-2017 Canonical Ltd
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

package policy

import (
	"fmt"

	"golang.org/x/exp/slices"

	"github.com/canonical/workshop/internal/asserts"
	"github.com/canonical/workshop/internal/sdk"
)

func checkPlugInstallationConstraints1(plug *sdk.PlugInfo, constraints *asserts.PlugInstallationConstraints) error {
	if err := checkNameConstraints(constraints.PlugNames, plug.Interface, "plug name", plug.Name); err != nil {
		return err
	}

	// TODO: allow evaluated attr constraints here too?
	if err := constraints.PlugAttributes.Check(plug, nil); err != nil {
		return err
	}
	if err := checkSdkType(plug.Sdk, constraints.PlugSdkTypes); err != nil {
		return err
	}
	return nil
}

func checkPlugInstallationAltConstraints(plug *sdk.PlugInfo, altConstraints []*asserts.PlugInstallationConstraints) error {
	var firstErr error
	// OR of constraints
	for _, constraints := range altConstraints {
		err := checkPlugInstallationConstraints1(plug, constraints)
		if err == nil {
			return nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func checkPlugConnectionConstraints1(connc *ConnectCandidate, constraints *asserts.PlugConnectionConstraints) error {
	if err := checkNameConstraints(constraints.PlugNames, connc.Plug.Interface(), "plug name", connc.Plug.Name()); err != nil {
		return err
	}
	if err := checkNameConstraints(constraints.SlotNames, connc.Slot.Interface(), "slot name", connc.Slot.Name()); err != nil {
		return err
	}

	if err := constraints.PlugAttributes.Check(connc.Plug, connc); err != nil {
		return err
	}
	if err := constraints.SlotAttributes.Check(connc.Slot, connc); err != nil {
		return err
	}
	plugSdk, slotSdk := connc.Plug.Sdk(), connc.Slot.Sdk()
	if plugSdk.ProjectId != slotSdk.ProjectId || plugSdk.Workshop != slotSdk.Workshop {
		return fmt.Errorf("%q cannot be connected to the %q (SDK from a different workshop)", connc.Plug.Ref().String(), connc.Slot.Ref().String())
	}
	return nil
}

func checkPlugConnectionAltConstraints(connc *ConnectCandidate, altConstraints []*asserts.PlugConnectionConstraints) (*asserts.PlugConnectionConstraints, error) {
	var firstErr error
	// OR of constraints
	for _, constraints := range altConstraints {
		err := checkPlugConnectionConstraints1(connc, constraints)
		if err == nil {
			return constraints, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, firstErr
}

// check helpers
func checkSdkType(sdkInfo *sdk.Info, types []string) error {
	if len(types) == 0 {
		return nil
	}
	if !slices.Contains(types, string(sdkInfo.Type)) {
		return fmt.Errorf("invalid SDK type %q", sdkInfo.Type)
	}
	return nil
}

func checkNameConstraints(c *asserts.NameConstraints, iface, which, name string) error {
	if c == nil {
		return nil
	}
	special := map[string]string{
		"$INTERFACE": iface,
	}
	return c.Check(which, name, special)
}

func checkSlotInstallationConstraints(slot *sdk.SlotInfo, constraints *asserts.SlotInstallationConstraints) error {
	if err := checkNameConstraints(constraints.SlotNames, slot.Interface, "slot name", slot.Name); err != nil {
		return err
	}

	if err := constraints.SlotAttributes.Check(slot, nil); err != nil {
		return err
	}

	if err := checkSdkType(slot.Sdk, constraints.SlotSdkTypes); err != nil {
		return err
	}
	return nil
}

func checkSlotInstallationAltConstraints(slot *sdk.SlotInfo, altConstraints []*asserts.SlotInstallationConstraints) error {
	var firstErr error
	// OR of constraints
	for _, constraints := range altConstraints {
		err := checkSlotInstallationConstraints(slot, constraints)
		if err == nil {
			return nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func checkSlotConnectionAltConstraints(connc *ConnectCandidate, altConstraints []*asserts.SlotConnectionConstraints) (*asserts.SlotConnectionConstraints, error) {
	var firstErr error
	// OR of constraints
	for _, constraints := range altConstraints {
		err := checkSlotConnectionConstraints1(connc, constraints)
		if err == nil {
			return constraints, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, firstErr
}

func checkSlotConnectionConstraints1(connc *ConnectCandidate, constraints *asserts.SlotConnectionConstraints) error {
	if err := checkNameConstraints(constraints.PlugNames, connc.Plug.Interface(), "plug name", connc.Plug.Name()); err != nil {
		return err
	}
	if err := checkNameConstraints(constraints.SlotNames, connc.Slot.Interface(), "slot name", connc.Slot.Name()); err != nil {
		return err
	}
	if err := constraints.PlugAttributes.Check(connc.Plug, connc); err != nil {
		return err
	}
	if err := constraints.SlotAttributes.Check(connc.Slot, connc); err != nil {
		return err
	}

	plugSdk, slotSdk := connc.Plug.Sdk(), connc.Slot.Sdk()
	if plugSdk.ProjectId != slotSdk.ProjectId || plugSdk.Workshop != slotSdk.Workshop {
		return fmt.Errorf("%q cannot be connected to the %q (SDK from a different workshop)", connc.Plug.Ref().String(), connc.Slot.Ref().String())
	}

	return nil
}
