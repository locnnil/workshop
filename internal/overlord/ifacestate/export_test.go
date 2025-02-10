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
