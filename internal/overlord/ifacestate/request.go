package ifacestate

import (
	"fmt"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/overlord/conflict"
	"github.com/canonical/workshop/internal/overlord/state"
)

// ErrAlreadyConnected describes the error that occurs when attempting to connect already connected interface.
type ErrAlreadyConnected struct {
	Connection interfaces.ConnRef
}

func (e ErrAlreadyConnected) Error() string {
	return fmt.Sprintf("already connected: %q", e.Connection.ID())
}

// Connect returns a set of tasks for connecting an interface.
func Connect(st *state.State, connRef *interfaces.ConnRef) (*state.TaskSet, error) {
	plugProject, plugWorkshop := connRef.PlugRef.ProjectId, connRef.PlugRef.Workshop
	err := conflict.CheckChangeConflict(st, plugProject, plugWorkshop, "")
	if err != nil {
		return nil, err
	}

	slotProject, slotWorkshop := connRef.SlotRef.ProjectId, connRef.SlotRef.Workshop
	err = conflict.CheckChangeConflict(st, slotProject, slotWorkshop, "")
	if err != nil {
		return nil, err
	}

	// check if the connection already exists
	conns, err := getConns(st)
	if err != nil {
		return nil, err
	}
	if conn, ok := conns[connRef.ID()]; ok && !conn.Undesired {
		return nil, &ErrAlreadyConnected{Connection: *connRef}
	}

	connectInterface := st.NewTask("connect", fmt.Sprintf("Connect %s/%s:%s to %s/%s:%s", plugWorkshop, connRef.PlugRef.Sdk, connRef.PlugRef.Name,
		slotWorkshop, connRef.SlotRef.Sdk, connRef.SlotRef.Name))

	connectInterface.Set("slot", connRef.SlotRef)
	connectInterface.Set("plug", connRef.PlugRef)
	connectInterface.Set("delayed-setup-profile", false)

	return state.NewTaskSet(connectInterface), nil
}

func Forget(st *state.State, conn *interfaces.ConnRef, forget bool) (*state.TaskSet, error) {
	plugProject, plugWorkshop := conn.PlugRef.ProjectId, conn.PlugRef.Workshop
	err := conflict.CheckChangeConflict(st, plugProject, plugWorkshop, "")
	if err != nil {
		return nil, err
	}

	slotProject, slotWorkshop := conn.SlotRef.ProjectId, conn.SlotRef.Workshop
	err = conflict.CheckChangeConflict(st, slotProject, slotWorkshop, "")
	if err != nil {
		return nil, err
	}

	disconnectTask := st.NewTask("disconnect", fmt.Sprintf("Disconnect %s/%s:%s from %s/%s:%s", plugWorkshop, conn.PlugRef.Sdk, conn.PlugRef.Name,
		slotWorkshop, conn.SlotRef.Sdk, conn.SlotRef.Name))

	disconnectTask.Set("slot", conn.SlotRef)
	disconnectTask.Set("plug", conn.PlugRef)
	disconnectTask.Set("forget", true)
	return state.NewTaskSet(disconnectTask), nil
}

// Disconnect returns a set of tasks for disconnecting an interface.
func Disconnect(st *state.State, conn *interfaces.Connection, forget bool) (*state.TaskSet, error) {
	plugProject, plugWorkshop := conn.Plug.Ref().ProjectId, conn.Plug.Ref().Workshop
	err := conflict.CheckChangeConflict(st, plugProject, plugWorkshop, "")
	if err != nil {
		return nil, err
	}

	slotProject, slotWorkshop := conn.Slot.Ref().ProjectId, conn.Slot.Ref().Workshop
	err = conflict.CheckChangeConflict(st, slotProject, slotWorkshop, "")
	if err != nil {
		return nil, err
	}

	disconnectTask := st.NewTask("disconnect", fmt.Sprintf("Disconnect %s/%s:%s from %s/%s:%s", plugWorkshop, conn.Plug.Ref().Sdk, conn.Plug.Name(),
		slotWorkshop, conn.Slot.Ref().Sdk, conn.Slot.Ref().Name))

	disconnectTask.Set("slot", conn.Slot.Ref())
	disconnectTask.Set("plug", conn.Plug.Ref())
	disconnectTask.Set("forget", forget)

	disconnectTask.Set("slot-static", conn.Slot.StaticAttrs())
	disconnectTask.Set("slot-dynamic", conn.Slot.DynamicAttrs())
	disconnectTask.Set("plug-static", conn.Plug.StaticAttrs())
	disconnectTask.Set("plug-dynamic", conn.Plug.DynamicAttrs())

	return state.NewTaskSet(disconnectTask), nil
}
