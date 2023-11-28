package device

import (
	"context"
	"fmt"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshopbackend"
)

type Backend struct {
	wsbackend workshopbackend.Profile
}

func (b *Backend) Initialize(backend workshopbackend.Profile) error {
	b.wsbackend = backend
	return nil
}

// Name returns the name of the backend.
func (b *Backend) Name() interfaces.SecuritySystem {
	return interfaces.SecurityLxdDevice
}

// Setup creates mount profile specific to a given sdk.
func (b *Backend) Setup(context context.Context, sdkInfo *sdk.Info, repo *interfaces.Repository) error {
	// Don't generate an SDK profile for the core SDK
	if sdkInfo.Type == sdk.Core {
		return nil
	}

	s, err := repo.SdkSpecification(context, b.Name(), sdkInfo)
	if err != nil {
		return fmt.Errorf("cannot obtain device snippets for workshop %q: %s", sdkInfo.Workshop, err)
	}

	spec := s.(*Specification)
	profile := workshopbackend.NewSdkProfile(sdkInfo.Name)
	for _, dev := range spec.devices {
		profile.AddDevice(dev)
	}
	return b.wsbackend.AssignProfile(context, sdkInfo.Workshop, profile)
}

// Remove removes profile of a given sdk.
//
// This method should be called after removing a sdk.
func (b *Backend) Remove(context context.Context, workshop, sdkName string) error {
	return b.wsbackend.RemoveProfile(context, workshop, sdkName)
}

// NewSpecification returns a new mount specification.
func (b *Backend) NewSpecification(user, pid string) interfaces.Specification {
	return &Specification{
		devices: make(map[string]workshopbackend.WorkshopDevice),
		user:    user,
		pid:     pid,
	}
}
