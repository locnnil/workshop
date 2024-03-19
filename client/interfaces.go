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

package client

// Plug represents the potential of a given snap to connect to a slot.
type Plug struct {
	ProjectId   string                 `json:"project-id"`
	Workshop    string                 `json:"workshop"`
	Sdk         string                 `json:"sdk"`
	Name        string                 `json:"plug"`
	Interface   string                 `json:"interface,omitempty"`
	Attrs       map[string]interface{} `json:"attrs,omitempty"`
	Label       string                 `json:"label,omitempty"`
	Connections []SlotRef              `json:"connections,omitempty"`
}

// PlugRef is a reference to a plug.
type PlugRef struct {
	ProjectId string `json:"project-id"`
	Workshop  string `json:"workshop"`
	Sdk       string `json:"sdk"`
	Name      string `json:"plug"`
}

// Slot represents a capacity offered by a snap.
type Slot struct {
	ProjectId   string                 `json:"project-id"`
	Workshop    string                 `json:"workshop"`
	Sdk         string                 `json:"sdk"`
	Name        string                 `json:"slot"`
	Interface   string                 `json:"interface,omitempty"`
	Attrs       map[string]interface{} `json:"attrs,omitempty"`
	Label       string                 `json:"label,omitempty"`
	Connections []PlugRef              `json:"connections,omitempty"`
}

// SlotRef is a reference to a slot.
type SlotRef struct {
	ProjectId string `json:"project-id"`
	Workshop  string `json:"workshop"`
	Sdk       string `json:"sdk"`
	Name      string `json:"slot"`
}

// Interface holds information about a given interface and its instances.
type Interface struct {
	Name    string `json:"name,omitempty"`
	Summary string `json:"summary,omitempty"`
	DocURL  string `json:"doc-url,omitempty"`
	Plugs   []Plug `json:"plugs,omitempty"`
	Slots   []Slot `json:"slots,omitempty"`
}

// InterfaceAction represents an action performed on the interface system.
type InterfaceAction struct {
	Action string `json:"action"`
	Forget bool   `json:"forget,omitempty"`
	Plugs  []Plug `json:"plugs,omitempty"`
	Slots  []Slot `json:"slots,omitempty"`
}

// InterfaceOptions represents opt-in elements include in responses.
type InterfaceOptions struct {
	Names     []string
	Doc       bool
	Plugs     bool
	Slots     bool
	Connected bool
}

// DisconnectOptions represents extra options for disconnect op
type DisconnectOptions struct {
	Forget bool
}
