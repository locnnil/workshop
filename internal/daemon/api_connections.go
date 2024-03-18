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

package daemon

import (
	"context"
	"net/http"
	"sort"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/overlord/ifacestate"
	"github.com/canonical/workshop/internal/overlord/workshopstate"
)

type collectFilter struct {
	projectId string
	workshop  string
	ifaceName string
	connected bool
}

func (c *collectFilter) plugOrConnectedSlotMatches(plug *interfaces.PlugRef, connectedSlots []interfaces.SlotRef) bool {
	for _, slot := range connectedSlots {
		if c.slotOrConnectedPlugMatches(&slot, nil) {
			return true
		}
	}
	if (c.projectId != plug.ProjectId) || (c.workshop != "" && plug.Workshop != c.workshop) {
		return false
	}
	return true
}

func (c *collectFilter) slotOrConnectedPlugMatches(slot *interfaces.SlotRef, connectedPlugs []interfaces.PlugRef) bool {
	for _, plug := range connectedPlugs {
		if c.plugOrConnectedSlotMatches(&plug, nil) {
			return true
		}
	}
	if (c.projectId != slot.ProjectId) || (c.workshop != "" && slot.Workshop != c.workshop) {
		return false
	}
	return true
}

func (c *collectFilter) ifaceMatches(ifaceName string) bool {
	if c.ifaceName != "" && c.ifaceName != ifaceName {
		return false
	}
	return true
}

type bySlotRef []interfaces.SlotRef

func (b bySlotRef) Len() int      { return len(b) }
func (b bySlotRef) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b bySlotRef) Less(i, j int) bool {
	return b[i].SortsBefore(b[j])
}

type byPlugRef []interfaces.PlugRef

func (b byPlugRef) Len() int      { return len(b) }
func (b byPlugRef) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byPlugRef) Less(i, j int) bool {
	return b[i].SortsBefore(b[j])
}

// mergeAttrs merges attributes from 2 disjoint sets of static and dynamic slot or
// plug attributes into a single map.
func mergeAttrs(one map[string]interface{}, other map[string]interface{}) map[string]interface{} {
	merged := make(map[string]interface{}, len(one)+len(other))
	for k, v := range one {
		merged[k] = v
	}
	for k, v := range other {
		merged[k] = v
	}
	return merged
}

func collectConnections(ifaceMgr *ifacestate.InterfaceManager, filter collectFilter) (*connectionsJSON, error) {
	repo := ifaceMgr.Repository()
	ifaces := repo.Interfaces()

	var connsjson connectionsJSON
	var connStates map[string]ifacestate.ConnectionState
	plugConns := map[string][]interfaces.SlotRef{}
	slotConns := map[string][]interfaces.PlugRef{}

	var err error
	connStates, err = ifaceMgr.ConnectionStates()
	if err != nil {
		return nil, err
	}

	connsjson.Established = make([]connectionJSON, 0, len(connStates))
	connsjson.Plugs = make([]*plugJSON, 0, len(ifaces.Plugs))
	connsjson.Slots = make([]*slotJSON, 0, len(ifaces.Slots))

	for crefStr, cstate := range connStates {
		if cstate.Undesired && filter.connected {
			continue
		}

		cref, err := interfaces.ParseConnRef(crefStr)
		if err != nil {
			return nil, err
		}

		if repo.Plug(cref.PlugRef.ProjectId, cref.PlugRef.Workshop, cref.PlugRef.Sdk, cref.PlugRef.Name) == nil ||
			repo.Slot(cref.SlotRef.ProjectId, cref.SlotRef.Workshop, cref.SlotRef.Sdk, cref.SlotRef.Name) == nil {
			continue
		}

		if !filter.plugOrConnectedSlotMatches(&cref.PlugRef, nil) && !filter.slotOrConnectedPlugMatches(&cref.SlotRef, nil) {
			continue
		}
		if !filter.ifaceMatches(cstate.Interface) {
			continue
		}
		plugRef := interfaces.PlugRef{ProjectId: cref.PlugRef.ProjectId, Workshop: cref.PlugRef.Workshop, Sdk: cref.PlugRef.Sdk, Name: cref.PlugRef.Name}
		slotRef := interfaces.SlotRef{ProjectId: cref.SlotRef.ProjectId, Workshop: cref.SlotRef.Workshop, Sdk: cref.SlotRef.Sdk, Name: cref.SlotRef.Name}
		plugID := plugRef.String()
		slotID := slotRef.String()

		cj := connectionJSON{
			Slot:      slotRef,
			Plug:      plugRef,
			Manual:    !cstate.Auto,
			Interface: cstate.Interface,
			PlugAttrs: mergeAttrs(cstate.StaticPlugAttrs, cstate.DynamicPlugAttrs),
			SlotAttrs: mergeAttrs(cstate.StaticSlotAttrs, cstate.DynamicSlotAttrs),
		}
		if cstate.Undesired {
			// explicitly disconnected are always manual
			cj.Manual = true
			connsjson.Undesired = append(connsjson.Undesired, cj)
		} else {
			plugConns[plugID] = append(plugConns[plugID], slotRef)
			slotConns[slotID] = append(slotConns[slotID], plugRef)

			connsjson.Established = append(connsjson.Established, cj)
		}
	}

	for _, plug := range ifaces.Plugs {
		plugRef := interfaces.PlugRef{ProjectId: plug.Sdk.ProjectId, Workshop: plug.Sdk.Workshop, Sdk: plug.Sdk.Name, Name: plug.Name}
		connectedSlots, connected := plugConns[plugRef.String()]
		if !connected && filter.connected {
			continue
		}
		if !filter.ifaceMatches(plug.Interface) || !filter.plugOrConnectedSlotMatches(&plugRef, connectedSlots) {
			continue
		}
		sort.Sort(bySlotRef(connectedSlots))
		pj := &plugJSON{
			ProjectId:   plugRef.ProjectId,
			Workshop:    plugRef.Workshop,
			Sdk:         plugRef.Sdk,
			Name:        plugRef.Name,
			Interface:   plug.Interface,
			Attrs:       plug.Attrs,
			Label:       plug.Label,
			Connections: connectedSlots,
		}
		connsjson.Plugs = append(connsjson.Plugs, pj)
	}
	for _, slot := range ifaces.Slots {
		slotRef := interfaces.SlotRef{ProjectId: slot.Sdk.ProjectId, Workshop: slot.Sdk.Workshop, Sdk: slot.Sdk.Name, Name: slot.Name}
		connectedPlugs, connected := slotConns[slotRef.String()]
		if !connected && filter.connected {
			continue
		}
		if !filter.ifaceMatches(slot.Interface) || !filter.slotOrConnectedPlugMatches(&slotRef, connectedPlugs) {
			continue
		}
		sort.Sort(byPlugRef(connectedPlugs))
		sj := &slotJSON{
			ProjectId:   slotRef.ProjectId,
			Workshop:    slotRef.Workshop,
			Sdk:         slotRef.Sdk,
			Name:        slotRef.Name,
			Interface:   slot.Interface,
			Attrs:       slot.Attrs,
			Label:       slot.Label,
			Connections: connectedPlugs,
		}
		connsjson.Slots = append(connsjson.Slots, sj)
	}
	return &connsjson, nil
}

type byCrefConnJSON []connectionJSON

func (b byCrefConnJSON) Len() int      { return len(b) }
func (b byCrefConnJSON) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byCrefConnJSON) Less(i, j int) bool {
	icj := b[i]
	jcj := b[j]
	iCref := interfaces.ConnRef{PlugRef: icj.Plug, SlotRef: icj.Slot}
	jCref := interfaces.ConnRef{PlugRef: jcj.Plug, SlotRef: jcj.Slot}
	sortsBefore := iCref.SortsBefore(&jCref)
	return sortsBefore
}

func checkWorkshopExists(ctx context.Context, manager *workshopstate.WorkshopManager, projectId, name string) error {
	_, err := manager.Workshop(ctx, name, projectId)
	return err
}

func v1GetConnections(c *Command, r *http.Request, _ *userState) Response {
	query := r.URL.Query()
	projectId := query.Get("project-id")
	workshop := query.Get("workshop")
	ifaceName := query.Get("interface")
	qselect := query.Get("select")

	if projectId == "" {
		return statusBadRequest("project-id must not be empty")
	}

	if qselect != "all" && qselect != "" {
		return statusBadRequest("unsupported select qualifier")
	}
	onlyConnected := qselect == ""

	if workshop != "" {
		if err := checkWorkshopExists(r.Context(), c.d.overlord.WorkshopManager(), projectId, workshop); err != nil {
			return statusNotFound("cannot access workshop: %v", err)
		}
	}

	connsjson, err := collectConnections(c.d.overlord.InterfaceManager(), collectFilter{
		projectId: projectId,
		workshop:  workshop,
		ifaceName: ifaceName,
		connected: onlyConnected,
	})
	if err != nil {
		return statusInternalError("collecting connection information failed: %v", err)
	}
	sort.Sort(byCrefConnJSON(connsjson.Established))
	sort.Sort(byCrefConnJSON(connsjson.Undesired))

	return SyncResponse(connsjson, http.StatusOK)
}
