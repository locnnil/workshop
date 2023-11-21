package workshopbackend

import (
	"context"
	"fmt"
)

type ContextKeyProjectId string
type ContextKeyUser string

type WorkshopConfigFilter func(config map[string]string) bool
type WorkshopDeviceFilter func(devices map[string]map[string]string) bool

const (
	ContextProjectId = ContextKeyProjectId("project-id")
	ContextUser      = ContextKeyUser("user")
)

func NewWorkshopConfigFilter(key string, value string) WorkshopConfigFilter {
	return func(config map[string]string) bool {
		return config[key] == value
	}
}

type ErrExec struct {
	Status int
}

func (e *ErrExec) Error() string {
	return fmt.Sprintf("command exit code %d", e.Status)
}

type WorkshopDevice struct {
	Name       string
	Properties map[string]string
}

type WorkshopConfigValue struct {
	Name  string
	Value string
}

type Stash interface {
	// Make a stash of the workshop. The workshop will be stopped and will not
	// be available to other workshop operations, e.g. list, stop, start and so
	// on. A new workshop with the same name can be launched for the same
	// project-id.
	StashWorkshop(ctx context.Context, name string) error

	// Restore the workshop from the stash (if exists, see StashWorkshop). The
	// workshop will be restored and become visible to the backend operations.
	// Fails if a workshop with the same name exists.
	UnstashWorkshop(ctx context.Context, name string) error

	// Delete the workshop from stash (if exists).
	RemoveWorkshopStash(ctx context.Context, name string) error
}

type WorkshopBackend interface {
	Stash
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
	LaunchWorkshop(ctx context.Context, name, base string) error

	// Delete workshop. Stop the workshop forcefully if not in Stopped before deleting
	RemoveWorkshop(ctx context.Context, name string) error

	// Starts a workshop and waits until it is ready
	// to accept commands
	StartWorkshop(ctx context.Context, name string) error

	// Stops workshop gracefully (i.e. waits for the graceful instance and all
	// its running services termination) unless force is used.
	StopWorkshop(ctx context.Context, name string, force bool) error

	// Create a temporary state storage volume for the workshop. It can be
	// mounted to the instance separately. This does not mount the device to the
	// workshop, it must be mounted to the required workshop as a separate
	// operation (see AddWorkshopDevice).
	CreateStateStorage(ctx context.Context, name string) error

	// Delete a temporary state storage volume for the workshop. It does
	// not unmount the volume from the workshop if mounted.
	DeleteStateStorage(ctx context.Context, name string) error

	// Adds a workshop device described by the properties.
	AddWorkshopDevice(ctx context.Context, name string, props WorkshopDevice) error

	// Removes a workshop device.
	RemoveWorkshopDevice(ctx context.Context, name string, device string) error

	// TODO: these methods are too generic and should be wrapped with a proper
	// interface method where required. We should not let the client to change
	// any workshop property arbitrarily.
	AddWorkshopConfig(ctx context.Context, name string, item *WorkshopConfigValue) error
	RemoveWorkshopConfig(ctx context.Context, name string, key string) error

	// Loads a workshop instance.
	Workshop(ctx context.Context, name string) (*Workshop, error)

	// Returns a workshop's file system interface.
	WorkshopFs(ctx context.Context, name string) (WorkshopFs, error)

	// Returns a list of workshops for the project in context.
	ProjectWorkshops(ctx context.Context) ([]*WorkshopFile, []*Workshop, error)

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
