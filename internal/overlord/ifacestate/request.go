package ifacestate

import (
	"fmt"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/overlord/conflict"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshop"
)

// ErrAlreadyConnected describes the error that occurs when attempting to connect already connected interface.
type ErrAlreadyConnected struct {
	Connection interfaces.ConnRef
}

func (e ErrAlreadyConnected) Error() string {
	return fmt.Sprintf("already connected: %q", e.Connection.ID())
}

// Connect returns a set of tasks for connecting an interface.
func Connect(st *state.State, w *workshop.Workshop, connRef *interfaces.ConnRef) (*state.TaskSet, error) {
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

	master, affected := maybeBound(w, connRef.PlugRef)
	masterTask := st.NewTask("connect", fmt.Sprintf("Connect %s to %s", master.ShortRef(), connRef.SlotRef.ShortRef()))

	masterTask.Set("slot", connRef.SlotRef)
	masterTask.Set("plug", master)
	masterTask.Set("delayed-setup-profile", false)

	ts := state.NewTaskSet(masterTask)
	prev := masterTask
	for _, p := range affected {
		slave := st.NewTask("connect", fmt.Sprintf("Connect %s to %s", p.ShortRef(), connRef.SlotRef.ShortRef()))
		slave.Set("slot", connRef.SlotRef)
		slave.Set("plug", p)
		slave.Set("delayed-setup-profile", true)
		slave.Set("plug-dynamic", map[string]interface{}{
			"bind": map[string]interface{}{"plug": connRef.PlugRef, "slot": connRef.SlotRef}})

		slave.WaitFor(prev)
		prev = slave
		ts.AddTask(slave)
	}

	return ts, nil
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

	return state.NewTaskSet(disconnectTask), nil
}
