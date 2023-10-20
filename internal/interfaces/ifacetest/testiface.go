package ifacetest

import (
	"context"

	"github.com/canonical/workspace/internal/interfaces"
	"github.com/canonical/workspace/internal/sdk"
	"github.com/canonical/workspace/internal/workspacebackend"
)

func CreateTestContext(username, projectId string) context.Context {
	ctx := context.WithValue(context.Background(), workspacebackend.ContextUser, username)
	ctx = context.WithValue(ctx, workspacebackend.ContextProjectId, projectId)
	return ctx
}

// TestInterface is a interface for various kind of tests.
// It is public so that it can be consumed from other packages.
type TestInterface struct {
	// InterfaceName is the name of this interface
	InterfaceName       string
	InterfaceStaticInfo interfaces.StaticInfo

	// AutoConnectCallback is the callback invoked inside AutoConnect
	AutoConnectCallback func(*sdk.PlugInfo, *sdk.SlotInfo) bool

	// BeforePreparePlugCallback is the callback invoked inside BeforePreparePlug()
	BeforePreparePlugCallback func(plug *sdk.PlugInfo) error
	// BeforePrepareSlotCallback is the callback invoked inside BeforePrepareSlot()
	BeforePrepareSlotCallback func(slot *sdk.SlotInfo) error

	BeforeConnectPlugCallback func(plug *interfaces.ConnectedPlug) error
	BeforeConnectSlotCallback func(slot *interfaces.ConnectedSlot) error

	// Support for interacting with the test backend.
	TestConnectedPlugCallback func(spec *Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error
	TestConnectedSlotCallback func(spec *Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error
	TestPermanentPlugCallback func(spec *Specification, plug *sdk.PlugInfo) error
	TestPermanentSlotCallback func(spec *Specification, slot *sdk.SlotInfo) error
}

func (t *TestInterface) StaticInfo() interfaces.StaticInfo {
	return t.InterfaceStaticInfo
}

func (t *TestInterface) String() string {
	return t.Name()
}

// Name returns the name of the test interface.
func (t *TestInterface) Name() string {
	return t.InterfaceName
}

func (t *TestInterface) AutoConnect(plug *sdk.PlugInfo, slot *sdk.SlotInfo) bool {
	if t.AutoConnectCallback != nil {
		return t.AutoConnectCallback(plug, slot)
	}
	return true
}

// BeforePreparePlug checks and possibly modifies a plug.
func (t *TestInterface) BeforePreparePlug(plug *sdk.PlugInfo) error {
	if t.BeforePreparePlugCallback != nil {
		return t.BeforePreparePlugCallback(plug)
	}
	return nil
}

// BeforePrepareSlot checks and possibly modifies a slot.
func (t *TestInterface) BeforePrepareSlot(slot *sdk.SlotInfo) error {
	if t.BeforePrepareSlotCallback != nil {
		return t.BeforePrepareSlotCallback(slot)
	}
	return nil
}

func (t *TestInterface) BeforeConnectPlug(plug *interfaces.ConnectedPlug) error {
	if t.BeforeConnectPlugCallback != nil {
		return t.BeforeConnectPlugCallback(plug)
	}
	return nil
}

func (t *TestInterface) BeforeConnectSlot(slot *interfaces.ConnectedSlot) error {
	if t.BeforeConnectSlotCallback != nil {
		return t.BeforeConnectSlotCallback(slot)
	}
	return nil
}

// Support for interacting with the test backend.

func (t *TestInterface) TestConnectedPlug(spec *Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error {
	if t.TestConnectedPlugCallback != nil {
		return t.TestConnectedPlugCallback(spec, plug, slot)
	}
	return nil
}

func (t *TestInterface) TestConnectedSlot(spec *Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error {
	if t.TestConnectedSlotCallback != nil {
		return t.TestConnectedSlotCallback(spec, plug, slot)
	}
	return nil
}

func (t *TestInterface) TestPermanentPlug(spec *Specification, plug *sdk.PlugInfo) error {
	if t.TestPermanentPlugCallback != nil {
		return t.TestPermanentPlugCallback(spec, plug)
	}
	return nil
}

func (t *TestInterface) TestPermanentSlot(spec *Specification, slot *sdk.SlotInfo) error {
	if t.TestPermanentSlotCallback != nil {
		return t.TestPermanentSlotCallback(spec, slot)
	}
	return nil
}
