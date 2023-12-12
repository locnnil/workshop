package ifacestate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/canonical/workshop/internal/asserts"
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/policy"
	"github.com/canonical/workshop/internal/interfaces/utils"
	"github.com/canonical/workshop/internal/jsonutil"
	"github.com/canonical/workshop/internal/overlord/ifacestate/schema"
	"github.com/canonical/workshop/internal/overlord/state"
)

func autoConnectCheck(plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) (bool, error) {
	baseDecl := asserts.BuiltinBaseDeclaration()
	if baseDecl == nil {
		return false, fmt.Errorf("internal error: cannot find base declaration")
	}

	ic := policy.ConnectCandidate{
		Plug:            plug,
		Slot:            slot,
		BaseDeclaration: baseDecl,
	}
	_, err := ic.CheckAutoConnect()
	return true, err
}

// getConns returns information about connections from the state.
func getConns(st *state.State) (conns map[string]*schema.ConnState, err error) {
	var raw *json.RawMessage
	err = st.Get("conns", &raw)
	if err != nil && !errors.Is(err, state.ErrNoState) {
		return nil, fmt.Errorf("cannot obtain raw data about existing connections: %s", err)
	}
	if raw != nil {
		err = jsonutil.DecodeWithNumber(bytes.NewReader(*raw), &conns)
		if err != nil {
			return nil, fmt.Errorf("cannot decode data about existing connections: %s", err)
		}
	}
	if conns == nil {
		conns = make(map[string]*schema.ConnState)
	}
	remapped := make(map[string]*schema.ConnState, len(conns))
	for id, cstate := range conns {
		cref, err := interfaces.ParseConnRef(id)
		if err != nil {
			return nil, err
		}
		cstate.StaticSlotAttrs = utils.NormalizeInterfaceAttributes(cstate.StaticSlotAttrs).(map[string]interface{})
		cstate.DynamicSlotAttrs = utils.NormalizeInterfaceAttributes(cstate.DynamicSlotAttrs).(map[string]interface{})
		cstate.StaticPlugAttrs = utils.NormalizeInterfaceAttributes(cstate.StaticPlugAttrs).(map[string]interface{})
		cstate.DynamicPlugAttrs = utils.NormalizeInterfaceAttributes(cstate.DynamicPlugAttrs).(map[string]interface{})
		remapped[cref.ID()] = cstate
	}
	return remapped, nil
}

// setConns sets information about connections in the state.
func setConns(st *state.State, conns map[string]*schema.ConnState) {
	remapped := make(map[string]*schema.ConnState, len(conns))
	for id, cstate := range conns {
		cref, err := interfaces.ParseConnRef(id)
		if err != nil {
			// We cannot fail here
			panic(err)
		}
		remapped[cref.ID()] = cstate
	}
	st.Set("conns", remapped)
}
