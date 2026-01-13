// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2019 Canonical Ltd
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

import (
	"bytes"
	"encoding/json"
	"net/url"
)

// Connection describes a connection between a plug and a slot.
type Connection struct {
	Slot      SlotRef `json:"slot"`
	Plug      PlugRef `json:"plug"`
	Interface string  `json:"interface"`
	// Manual is set for connections that were established manually.
	Manual bool `json:"manual"`
	// SlotAttrs is the list of attributes of the slot side of the connection.
	SlotAttrs map[string]any `json:"slot-attrs,omitempty"`
	// PlugAttrs is the list of attributes of the plug side of the connection.
	PlugAttrs map[string]any `json:"plug-attrs,omitempty"`
}

// Connections contains information about connections, as well as related plugs
// and slots.
type Connections struct {
	// Established is the list of connections that are currently present.
	Established []Connection `json:"established"`
	// Undersired is a list of connections that are manually denied.
	Undesired []Connection `json:"undesired"`
	Plugs     []Plug       `json:"plugs"`
	Slots     []Slot       `json:"slots"`
}

// ConnectionOptions contains criteria for selecting matching connections, plugs
// and slots.
type ConnectionOptions struct {
	ProjectId string
	// Workshop selects connections with the workshop on one of the sides, as well
	// as plugs and slots of a given workshop.
	Workshop string
	// Interface selects connections, plugs or slots using given interface.
	Interface string
	// All when true, selects established and undesired connections as well
	// as all disconnected plugs and slots.
	All bool
}

// Connections returns matching plugs, slots and their connections. Unless
// specified by matching options, returns established connections.
func (client *Client) Connections(opts *ConnectionOptions) (Connections, error) {
	var conns Connections
	query := url.Values{}
	if opts != nil && opts.ProjectId != "" {
		query.Set("project-id", opts.ProjectId)
	}
	if opts != nil && opts.Workshop != "" {
		query.Set("workshop", opts.Workshop)
	}
	if opts != nil && opts.Interface != "" {
		query.Set("interface", opts.Interface)
	}
	if opts != nil && opts.All {
		query.Set("select", "all")
	}
	_, err := client.doSync("GET", "/v1/connections", query, nil, nil, &conns)
	return conns, err
}

// performInterfaceAction performs a single action on the interface system.
func (client *Client) performInterfaceAction(sa *InterfaceAction) (changeID string, err error) {
	b, err := json.Marshal(sa)
	if err != nil {
		return "", err
	}
	return client.doAsync("POST", "/v1/connections", nil, nil, bytes.NewReader(b))
}

// Disconnect breaks the connection between a plug and a slot.
func (client *Client) Disconnect(plugProjectId, plugWorkshop, plugSdkName, plugName, slotProjectId, slotWorkshop, slotSdkName, slotName string, opts *DisconnectOptions) (changeID string, err error) {
	return client.performInterfaceAction(&InterfaceAction{
		Action: "disconnect",
		Forget: opts != nil && opts.Forget,
		Plugs:  []Plug{{ProjectId: plugProjectId, Workshop: plugWorkshop, Sdk: plugSdkName, Name: plugName}},
		Slots:  []Slot{{ProjectId: slotProjectId, Workshop: slotWorkshop, Sdk: slotSdkName, Name: slotName}},
	})
}

// Connects a plug and a slot.
func (client *Client) Connect(plugProjectId, plugWorkshop, plugSdkName, plugName, slotProjectId, slotWorkshop, slotSdkName, slotName string, opts *DisconnectOptions) (changeID string, err error) {
	return client.performInterfaceAction(&InterfaceAction{
		Action: "connect",
		Plugs:  []Plug{{ProjectId: plugProjectId, Workshop: plugWorkshop, Sdk: plugSdkName, Name: plugName}},
		Slots:  []Slot{{ProjectId: slotProjectId, Workshop: slotWorkshop, Sdk: slotSdkName, Name: slotName}},
	})
}
