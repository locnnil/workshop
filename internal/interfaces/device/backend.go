package device

import (
	"context"
	"fmt"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshopbackend"
)

type Backend struct {
	wsbackend workshopbackend.WorkshopBackend
}

func (b *Backend) Initialize(backend workshopbackend.WorkshopBackend) error {
	b.wsbackend = backend
	return nil
}

// Name returns the name of the backend.
func (b *Backend) Name() interfaces.SecuritySystem {
	return interfaces.SecurityLxdDevice
}

// Setup creates mount profile specific to a given sdk.
func (b *Backend) Setup(context context.Context, sdkInfo *sdk.Info, repo *interfaces.Repository) error {
	s, err := repo.SdkSpecification(context, b.Name(), sdkInfo)
	if err != nil {
		return fmt.Errorf("cannot obtain device snippets for workshop %q: %s", sdkInfo.Workshop, err)
	}

	spec := s.(*Specification)
	for _, dev := range spec.devices {
		err = b.wsbackend.AddWorkshopDevice(context, sdkInfo.Workshop, *dev)
		if err != nil {
			return nil
		}
	}
	return nil
}

// Remove removes mount configuration of a given sdk.
//
// This method should be called after removing a sdk.
func (b *Backend) Remove(context context.Context, workshop, sdkName string) error {
	return nil
}

// NewSpecification returns a new mount specification.
func (b *Backend) NewSpecification(user, pid string) interfaces.Specification {
	return &Specification{
		devices: make(map[string]*workshopbackend.WorkshopDevice),
		user:    user,
		pid:     pid,
	}
}
