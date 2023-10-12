package device

import (
	"context"
	"fmt"

	"github.com/canonical/workspace/internal/interfaces"
	"github.com/canonical/workspace/internal/sdk"
	"github.com/canonical/workspace/internal/workspacebackend"
)

// Backend is responsible for maintaining mount files for snap-confine
type Backend struct {
	wsbackend workspacebackend.WorkspaceBackend
}

// Initialize does nothing.
func (b *Backend) Initialize(backend workspacebackend.WorkspaceBackend) error {
	b.wsbackend = backend
	return nil
}

// Name returns the name of the backend.
func (b *Backend) Name() interfaces.SecuritySystem {
	return interfaces.SecurityLxdDevice
}

// Setup creates mount mount profile files specific to a given snap.
func (b *Backend) Setup(context context.Context, sdkInfo *sdk.Info, repo *interfaces.Repository) error {
	s, err := repo.SdkSpecification(context, b.Name(), sdkInfo)
	if err != nil {
		return fmt.Errorf("cannot obtain device snippets for workspace %q: %s", sdkInfo.Workspace, err)
	}

	spec := s.(*Specification)
	for _, dev := range spec.devices {
		err = b.wsbackend.AddWorkspaceDevice(context, sdkInfo.Workspace, *dev)
		if err != nil {
			return nil
		}
	}
	return nil
}

// Remove removes mount configuration files of a given snap.
//
// This method should be called after removing a snap.
func (b *Backend) Remove(sdkName string) error {
	return nil
}

// NewSpecification returns a new mount specification.
func (b *Backend) NewSpecification(user, pid string) interfaces.Specification {
	return &Specification{
		devices: make(map[string]*workspacebackend.WorkspaceDevice),
		user:    user,
		pid:     pid,
	}
}
