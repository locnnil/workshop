package ifacestate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/canonical/workshop/internal/asserts"
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/policy"
	"github.com/canonical/workshop/internal/interfaces/utils"
	"github.com/canonical/workshop/internal/jsonutil"
	"github.com/canonical/workshop/internal/overlord/ifacestate/schema"
	"github.com/canonical/workshop/internal/overlord/state"
)

type checker = func(plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) (bool, error)

func autoConnectChecker(workshopConns []interfaces.ConnRef) checker {
	return func(p *interfaces.ConnectedPlug, s *interfaces.ConnectedSlot) (bool, error) {
		conn := interfaces.ConnRef{PlugRef: *p.Ref(), SlotRef: *s.Ref()}
		// The pair is explicitly connected in a workshop file.
		if slices.Contains(workshopConns, conn) {
			// The interface's policy will be able to determine whether the
			// allow-auto-connection rule is still passable based on the
			// "auto-explicit" property (meaning the connection was explicit in
			// the workshop definition).
			if err := p.SetAttr("auto-explicit", "true"); err != nil {
				return false, err
			}
			return autoConnectCheck(p, s)
		}

		// The pair is not explicitly connected but the plug is used in other
		// explicit workshop connections; reject this candidate pair. Example:
		// content interface plug connected explicitly should reject connections
		// to other slots.
		if slices.ContainsFunc(workshopConns, func(conn interfaces.ConnRef) bool {
			return conn.PlugRef == *p.Ref()
		}) {
			return false, nil
		}

		// Not mentioned in the workshop file connections list; fallback to a
		// regular policy check.
		return autoConnectCheck(p, s)
	}
}

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

func connectCheck(plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) (bool, error) {
	baseDecl := asserts.BuiltinBaseDeclaration()
	if baseDecl == nil {
		return false, fmt.Errorf("internal error: cannot find base declaration")
	}

	ic := policy.ConnectCandidate{
		Plug:            plug,
		Slot:            slot,
		BaseDeclaration: baseDecl,
	}
	_, err := ic.Check()
	return true, err
}

// getConns returns information about connections from the state.
func getConns(st *state.State) (conns map[string]*schema.ConnState, err error) {
	var raw *json.RawMessage
	err = st.Get("conns", &raw)
	if err != nil && !errors.Is(err, state.ErrNoState) {
		return nil, fmt.Errorf("cannot obtain raw data about existing connections: %w", err)
	}
	if raw != nil {
		err = jsonutil.DecodeWithNumber(bytes.NewReader(*raw), &conns)
		if err != nil {
			return nil, fmt.Errorf("cannot decode data about existing connections: %w", err)
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
