package device

import (
	"context"
	"fmt"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workspacebackend"
)

type Backend struct {
	wsbackend workspacebackend.WorkspaceBackend
}

func (b *Backend) Initialize(backend workspacebackend.WorkspaceBackend) error {
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

// Remove removes mount configuration of a given sdk.
//
// This method should be called after removing a sdk.
func (b *Backend) Remove(context context.Context, workspace, sdkName string) error {
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
