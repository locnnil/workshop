// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package ifacestate

import (
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/overlord/ifacestate/schema"
)

var (
	AutoConnectChecker = autoConnectChecker
	GetConns           = getConns
	SetConns           = setConns
	ReloadConnections  = (*InterfaceManager).reloadConnections
)

func UpdateConnectionInConnState(conns map[string]*schema.ConnState, conn *interfaces.Connection, autoConnect, undesired bool) {
	connRef := &interfaces.ConnRef{
		PlugRef: conn.Plug.Ref(),
		SlotRef: conn.Slot.Ref(),
	}

	conns[connRef.ID()] = &schema.ConnState{
		Interface:        conn.Interface(),
		StaticPlugAttrs:  conn.Plug.StaticAttrs(),
		DynamicPlugAttrs: conn.Plug.DynamicAttrs(),
		StaticSlotAttrs:  conn.Slot.StaticAttrs(),
		DynamicSlotAttrs: conn.Slot.DynamicAttrs(),
		Auto:             autoConnect,
		Undesired:        undesired,
	}
}
