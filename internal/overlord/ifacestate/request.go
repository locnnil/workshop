package ifacestate

import (
	"fmt"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/overlord/conflict"
	"github.com/canonical/workshop/internal/overlord/state"
)

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
