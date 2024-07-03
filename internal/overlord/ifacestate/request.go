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
func Connect(st *state.State, plugW *workshop.Workshop, connRef *interfaces.ConnRef) (*state.TaskSet, error) {
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

	master, affected := maybeBound(plugW, connRef.PlugRef)
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
		// mark the plug's connection as bound
		bref := interfaces.ConnRef{PlugRef: master, SlotRef: connRef.SlotRef}
		slave.Set("plug-dynamic", map[string]interface{}{"bind": bref.ID()})

		slave.WaitFor(prev)
		prev = slave
		ts.AddTask(slave)
	}

	return ts, nil
}

func disconnect(st *state.State, plugW *workshop.Workshop, conn *interfaces.ConnRef, forget bool) *state.TaskSet {
	master, affected := maybeBound(plugW, conn.PlugRef)

	mtask := st.NewTask("disconnect", fmt.Sprintf("Disconnect %s from %s", master, conn.SlotRef.ShortRef()))
	mtask.Set("plug", master)
	mtask.Set("slot", conn.SlotRef)
	mtask.Set("forget", forget)
	ts := state.NewTaskSet(mtask)
	prev := mtask

	for _, p := range affected {
		stask := st.NewTask("disconnect", fmt.Sprintf("Disconnect %s from %s", p, conn.SlotRef.ShortRef()))
		stask.Set("plug", p)
		stask.Set("slot", conn.SlotRef)
		stask.Set("forget", forget)

		stask.WaitFor(prev)
		prev = stask
		ts.AddTask(stask)
	}
	return ts
}

// Disconnect returns a set of tasks for disconnecting an interface.
func Disconnect(st *state.State, plugW *workshop.Workshop, conn *interfaces.ConnRef, forget bool) (*state.TaskSet, error) {
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
	return disconnect(st, plugW, conn, forget), nil
}
