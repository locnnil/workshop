package device

import (
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshopbackend"
	"golang.org/x/exp/maps"
)

type Specification struct {
	devices map[string]workshopbackend.WorkshopDevice
	user    string
	pid     string
}

func (s *Specification) User() string {
	return s.user
}

func (s *Specification) ProjectId() string {
	return s.pid
}

// AddPermanentSlot records side-effects of having a slot.
func (s *Specification) AddPermanentSlot(iface interfaces.Interface, slot *sdk.SlotInfo) error {
	return nil
}

// AddPermanentPlug records side-effects of having a plug.
func (s *Specification) AddPermanentPlug(iface interfaces.Interface, plug *sdk.PlugInfo) error {
	return nil
}

// AddConnectedSlot records side-effects of having a connected slot.
func (s *Specification) AddConnectedSlot(iface interfaces.Interface, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error {
	return nil
}

// AddConnectedPlug records side-effects of having a connected plug.
func (s *Specification) AddConnectedPlug(iface interfaces.Interface, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error {
	type definer interface {
		MountConnectedPlug(spec *Specification, plug *interfaces.ConnectedPlug, slot *interfaces.ConnectedSlot) error
	}

	if iface, ok := iface.(definer); ok {
		return iface.MountConnectedPlug(s, plug, slot)
	}
	return nil
}

func (s *Specification) DeviceEntries() []workshopbackend.WorkshopDevice {
	return maps.Values(s.devices)
}

func (s *Specification) AddDeviceEntry(dev workshopbackend.WorkshopDevice) error {
	if s.devices == nil {
		s.devices = make(map[string]workshopbackend.WorkshopDevice)
	}
	s.devices[dev.Name()] = dev
	return nil
}
