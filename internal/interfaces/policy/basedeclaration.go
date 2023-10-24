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

package policy

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/canonical/workshop/internal/asserts"
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
)

// The headers of the builtin base-declaration describing the default
// interface policies for all sdks. The base declaration focuses on the slot
// side for almost all interfaces. Importantly, items are not merged between
// the slots and plugs or between the base declaration and sdk declaration
// for a particular type of rule. This means that if you specify an
// installation rule for both slots and plugs in the base declaration, only
// the plugs side is used (plugs is preferred over slots).
//
// The interfaces listed in the base declaration can be broadly categorized
// into:
//
// - manually connected implicit slots (eg, bluetooth-control)
// - auto-connected implicit slots (eg, network)
// - manually connected app-provided slots (eg, bluez)
// - auto-connected app-provided slots (eg, mir)
//
// such that they will follow this pattern:
//
//   slots:
//     manual-connected-implicit-slot:
//       allow-installation:
//         slot-type:
//           - core                     # implicit slot
//       deny-auto-connection: true     # force manual connect
//
//     auto-connected-implicit-slot:
//       allow-installation:
//         slot-type:
//           - core                     # implicit slot
//       allow-auto-connection: true    # allow auto-connect
//
//     manual-connected-provided-slot:
//       allow-installation:
//         slot-type:
//           - sdk                      # sdk provided slot
//       deny-connection: true          # require allow-connection in sdk decl
//       deny-auto-connection: true     # force manual connect
//
//     auto-connected-provided-slot:
//       allow-installation:
//         slot-type:
//           - sdk                      # sdk provided slot
//       deny-connection: true          # require allow-connection in sdk decl

const baseDeclarationHeader = `
type: base-declaration
authority-id: canonical
series: 1
revision: 0
`

const baseDeclarationPlugs = `
plugs:
`

const baseDeclarationSlots = `
slots:
`

func trimTrailingNewline(s string) string {
	return strings.TrimRight(s, "\n")
}

func composeBaseDeclaration(ifaces []interfaces.Interface) ([]byte, error) {
	var buf bytes.Buffer
	// Trim newlines at the end of the string. All the elements may have
	// spurious trailing newlines. All elements start with a leading newline.
	// We don't want any blanks as that would no longer parse.
	if _, err := buf.WriteString(trimTrailingNewline(baseDeclarationHeader)); err != nil {
		return nil, err
	}
	if _, err := buf.WriteString(trimTrailingNewline(baseDeclarationSlots)); err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		slotPolicy := interfaces.StaticInfoOf(iface).BaseDeclarationSlots
		if _, err := buf.WriteString(trimTrailingNewline(slotPolicy)); err != nil {
			return nil, err
		}
	}
	if _, err := buf.WriteRune('\n'); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func init() {
	decl, err := composeBaseDeclaration(builtin.Interfaces())
	if err != nil {
		panic(fmt.Sprintf("cannot compose base-declaration: %v", err))
	}
	if err := asserts.InitBuiltinBaseDeclaration(decl); err != nil {
		panic(fmt.Sprintf("cannot initialize the builtin base-declaration: %v", err))
	}
}
