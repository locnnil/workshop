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

	"github.com/canonical/workshop/internal/asserts"
	"github.com/canonical/workshop/internal/sdk"
	"golang.org/x/exp/slices"
)

// check helpers

func checkSlotType(sdkInfo *sdk.Info, types []string) error {
	if !slices.Contains(types, string(sdkInfo.Type)) {
		return fmt.Errorf("invalid sdk type %q", sdkInfo.Type)
	}
	return nil
}

func checkSlotInstallationConstraints(ic *InstallCandidate, slot *sdk.SlotInfo, constraints *asserts.SlotInstallationConstraints) error {
	if err := constraints.SlotAttributes.Check(slot, nil); err != nil {
		return err
	}

	if err := checkSlotType(slot.Sdk, constraints.SlotTypes); err != nil {
		return err
	}
	return nil
}

func checkSlotInstallationAltConstraints(ic *InstallCandidate, slot *sdk.SlotInfo, altConstraints []*asserts.SlotInstallationConstraints) error {
	var firstErr error
	// OR of constraints
	for _, constraints := range altConstraints {
		err := checkSlotInstallationConstraints(ic, slot, constraints)
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
	if err := constraints.PlugAttributes.Check(connc.Plug, connc); err != nil {
		return err
	}
	if err := constraints.SlotAttributes.Check(connc.Slot, connc); err != nil {
		return err
	}
	return nil
}
