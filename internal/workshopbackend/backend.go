package workshopbackend

import (
	"context"
	"fmt"
)

type ContextKeyProjectId string
type ContextKeyUser string

type WorkspaceConfigFilter func(config map[string]string) bool
type WorkspaceDeviceFilter func(devices map[string]map[string]string) bool

const (
	ContextProjectId = ContextKeyProjectId("project-id")
	ContextUser      = ContextKeyUser("user")
)

func NewWorkspaceConfigFilter(key string, value string) WorkspaceConfigFilter {
	return func(config map[string]string) bool {
		return config[key] == value
	}
}

func EveryWorkspace() WorkspaceConfigFilter {
	return func(config map[string]string) bool {
		return true
	}
}

type ErrExec struct {
	Status int
}

func (e *ErrExec) Error() string {
	return fmt.Sprintf("command exit code %d", e.Status)
}

type WorkspaceDevice struct {
	Name       string
	Properties map[string]string
}

type WorkspaceConfigValue struct {
	Name  string
	Value string
}

type WorkspaceBackend interface {
	// The backend will attempt to load a project for the given path
	// using its mapping between the path and a project id. If the project
	// is not found, e.g. .lock file was removed by the user, but there is still
	// an existing record for the path, it will first attempt to restore .lock.
	// If unsuccessful, a new project will be created for the path provided.
	CreateOrLoadProject(ctx context.Context, path string) (*Project, bool, error)

	// Returns a list of projects known to the backend. The returned map
	// has a username that the corresponding projects belong to as a key.
	Projects(ctx context.Context) (map[string][]*Project, error)

	// Launch a barebone workshop instance using the base provided. The
	// supported bases are ubuntu@20.04 and ubuntu@22.04.
	LaunchWorkspace(ctx context.Context, name, base string) error

	// Delete workshop. Stop the workshop forcefully if not in Stopped before deleting
	RemoveWorkspace(ctx context.Context, name string) error

	// Starts a workshop and waits until it is ready
	// to accept commands
	StartWorkspace(ctx context.Context, name string) error

	// Stops workshop gracefully (i.e. waits for the graceful instance and all
	// its running services termination) unless force is used.
	StopWorkspace(ctx context.Context, name string, force bool) error

	// Make a stash of the workshop. The workshop will be stopped and will not
	// be available to other workshop operations, e.g. list, stop, start and so
	// on. A new workshop with the same name can be launched for the same
	// project-id.
	StashWorkspace(ctx context.Context, name string) error

	// Restore the workshop from the stash (if exists, see StashWorkspace). The
	// workshop will be restored and become visible to the backend operations.
	// Fails if a workshop with the same name exists.
	UnstashWorkspace(ctx context.Context, name string) error

	// Delete the workshop from stash (if exists).
	RemoveWorkspaceStash(ctx context.Context, name string) error

	// Create a temporary state storage volume for the workshop. It can be
	// mounted to the instance separately. This does not mount the device to the
	// workshop, it must be mounted to the required workshop as a separate
	// operation (see AddWorkspaceDevice).
	CreateStateStorage(ctx context.Context, name string) error

	// Delete a temporary state storage volume for the workshop. It does
	// not unmount the volume from the workshop if mounted.
	DeleteStateStorage(ctx context.Context, name string) error

	// Adds a workshop device described by the properties.
	AddWorkspaceDevice(ctx context.Context, name string, props WorkspaceDevice) error

	// Removes a workshop device.
	RemoveWorkspaceDevice(ctx context.Context, name string, device string) error

	// TODO: these methods are too generic and should be wrapped with a proper
	// interface method where required. We should not let the client to change
	// any workshop property arbitrarily.
	AddWorkspaceConfig(ctx context.Context, name string, item *WorkspaceConfigValue) error
	RemoveWorkspaceConfig(ctx context.Context, name string, key string) error

	// Loads a workshop instance.
	GetWorkspace(ctx context.Context, name string) (*Workshop, error)

	// Returns a workshop's file system interface.
	GetWorkspaceFs(ctx context.Context, name string) (WorkspaceFs, error)

	// Returns a list of workshops for the project in context.
	GetProjectWorkspaces(ctx context.Context) ([]*WorkspaceFile, []*Workshop, error)

	// Execute a command in a given workshop. The client should differentiate
	// between the errors that occured during the execution but not related to
	// the command (i.e. the workshop does not exist) and the errors that were
	// produced by the command itself (i.e. return code != 0). If the latter, an
	// instance of ErrExec with the status code will be returned.

	// The callback ExecContext.WaitExecution will initite the command execution
	// and redirect its IO using args.ExecControls. ExecContext.Environment will
	// contain full (actual)
	Exec(ctx context.Context, name string, args *Execution) (ExecContext, error)
}
