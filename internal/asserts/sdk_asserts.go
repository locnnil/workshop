// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2015-2022 Canonical Ltd
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

package asserts

import (
	"bytes"
	"fmt"
	"time"

	_ "golang.org/x/crypto/sha3"
)

func compilePlugRules(plugs map[string]interface{}, compiled func(iface string, plugRule *PlugRule)) error {
	for iface, rule := range plugs {
		plugRule, err := compilePlugRule(iface, rule)
		if err != nil {
			return err
		}
		compiled(iface, plugRule)
	}
	return nil
}

func compileSlotRules(slots map[string]interface{}, compiled func(iface string, slotRule *SlotRule)) error {
	for iface, rule := range slots {
		slotRule, err := compileSlotRule(iface, rule)
		if err != nil {
			return err
		}
		compiled(iface, slotRule)
	}
	return nil
}

// BaseDeclaration holds a base-declaration assertion, declaring the
// policies (to start with interface ones) applying to all sdks of
// a series.
type BaseDeclaration struct {
	assertionBase
	plugRules map[string]*PlugRule
	slotRules map[string]*SlotRule
	timestamp time.Time
}

// Series returns the series whose sdks are governed by the declaration.
func (basedcl *BaseDeclaration) Series() string {
	return basedcl.HeaderString("series")
}

// Timestamp returns the time when the base-declaration was issued.
func (basedcl *BaseDeclaration) Timestamp() time.Time {
	return basedcl.timestamp
}

// PlugRule returns the plug-side rule about the given interface if one was included in the plugs stanza of the declaration, otherwise it returns nil.
func (basedcl *BaseDeclaration) PlugRule(interfaceName string) *PlugRule {
	return basedcl.plugRules[interfaceName]
}

// SlotRule returns the slot-side rule about the given interface if one was included in the slots stanza of the declaration, otherwise it returns nil.
func (basedcl *BaseDeclaration) SlotRule(interfaceName string) *SlotRule {
	return basedcl.slotRules[interfaceName]
}

func assembleBaseDeclaration(assert assertionBase) (Assertion, error) {
	var plugRules map[string]*PlugRule
	plugs, err := checkMap(assert.headers, "plugs")
	if err != nil {
		return nil, err
	}
	if plugs != nil {
		plugRules = make(map[string]*PlugRule, len(plugs))
		err := compilePlugRules(plugs, func(iface string, rule *PlugRule) {
			plugRules[iface] = rule
		})
		if err != nil {
			return nil, err
		}
	}

	var slotRules map[string]*SlotRule
	slots, err := checkMap(assert.headers, "slots")
	if err != nil {
		return nil, err
	}
	if slots != nil {
		slotRules = make(map[string]*SlotRule, len(slots))
		err := compileSlotRules(slots, func(iface string, rule *SlotRule) {
			slotRules[iface] = rule
		})
		if err != nil {
			return nil, err
		}
	}

	timestamp, err := checkRFC3339Date(assert.headers, "timestamp")
	if err != nil {
		return nil, err
	}

	return &BaseDeclaration{
		assertionBase: assert,
		plugRules:     plugRules,
		slotRules:     slotRules,
		timestamp:     timestamp,
	}, nil
}

var builtinBaseDeclaration *BaseDeclaration

// BuiltinBaseDeclaration exposes the initialized builtin base-declaration assertion. This is used by overlord/assertstate, other code should use assertstate.BaseDeclaration.
func BuiltinBaseDeclaration() *BaseDeclaration {
	return builtinBaseDeclaration
}

var (
	builtinBaseDeclarationCheckOrder      = []string{"type", "authority-id", "series"}
	builtinBaseDeclarationExpectedHeaders = map[string]interface{}{
		"type":         "base-declaration",
		"authority-id": "canonical",
		"series":       "1",
	}
)

// InitBuiltinBaseDeclaration initializes the builtin base-declaration based on headers (or resets it if headers is nil).
func InitBuiltinBaseDeclaration(headers []byte) error {
	if headers == nil {
		builtinBaseDeclaration = nil
		return nil
	}
	trimmed := bytes.TrimSpace(headers)
	h, err := parseHeaders(trimmed)
	if err != nil {
		return err
	}
	for _, name := range builtinBaseDeclarationCheckOrder {
		expected := builtinBaseDeclarationExpectedHeaders[name]
		if h[name] != expected {
			return fmt.Errorf("the builtin base-declaration %q header is not set to expected value %q", name, expected)
		}
	}
	revision, err := checkRevision(h)
	if err != nil {
		return fmt.Errorf("cannot assemble the builtin-base declaration: %v", err)
	}
	h["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	a, err := assembleBaseDeclaration(assertionBase{
		headers:   h,
		body:      nil,
		revision:  revision,
		content:   trimmed,
		signature: []byte("$builtin"),
	})
	if err != nil {
		return fmt.Errorf("cannot assemble the builtin base-declaration: %v", err)
	}
	builtinBaseDeclaration = a.(*BaseDeclaration)
	return nil
}
