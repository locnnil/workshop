package workspacebackend

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
	return fmt.Sprintf("command failed with an error code (%d)", e.Status)
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

	// Returns a list of projects known to the backend.
	Projects(ctx context.Context) (map[string]*Project, error)

	// Launch a barebone workspace instance using the base provided. Currently,
	// the supported bases are ubuntu@20.04 and ubuntu@22.04.
	LaunchWorkspace(ctx context.Context, name, base string) error

	// Delete workspace. Will fail if forceful flag is false and the workspace
	// is not in the Stopped state.
	DeleteWorkspace(ctx context.Context, name string, forceful bool) error

	// Used to execute an action for the workspace. Currently,
	// "start" and "stop" actions are supported. TODO: choose a more
	// correct naming for this method as "launch" is also an action but
	// it is represented as a separate method.
	SetWorkspaceState(ctx context.Context, name, action string) error

	// Make a stash of the workspace. The workspace will be stopped and will not
	// be available to other workspace operations, e.g. list, stop, start and so
	// on. A new workspace with the same name can be launched for the same
	// project-id.
	StashWorkspace(ctx context.Context, name string) error

	// Restore the workspace from the stash (if exists, see StashWorkspace). The
	// workspace will be restored and become visible to the backend operations.
	// Fails if a workspace with the same name exists.
	UnstashWorkspace(ctx context.Context, name string) error

	// Delete the workspace from stash (if exists).
	RemoveWorkspaceStash(ctx context.Context, name string) error

	// Create a temporary state storage volume for the workspace. It can be
	// mounted to the instance separately. This does not mount the device to the
	// workspace, it must be mounted to the required workspace as a separate
	// operation (see AddWorkspaceDevice).
	CreateStateStorage(ctx context.Context, name string) error

	// Delete a temporary state storage volume for the workspace. It does
	// not unmount the volume from the workspace if mounted.
	DeleteStateStorage(ctx context.Context, name string) error

	// Adds a workspace device described by the properties.
	AddWorkspaceDevice(ctx context.Context, name string, props WorkspaceDevice) error

	// Removes a workspace device.
	RemoveWorkspaceDevice(ctx context.Context, name string, device string) error

	// TODO: these methods are too generic and should be wrapped with a proper
	// interface method where required. We should not let the client to change
	// any workspace property arbitrarily.
	AddWorkspaceConfig(ctx context.Context, name string, item *WorkspaceConfigValue) error
	RemoveWorkspaceConfig(ctx context.Context, name string, key string) error

	// Loads a workspace instance.
	GetWorkspace(ctx context.Context, name string) (*Workspace, error)

	// Returns a workspace's file system interface.
	GetWorkspaceFs(ctx context.Context, name string) (WorkspaceFs, error)

	// Returns a list of workspaces for the project in context.
	GetProjectWorkspaces(ctx context.Context) ([]*WorkspaceFile, []*Workspace, error)

	// Execute a command in a given workspace. The implementation shall
	// differentiate between the errors that occured during the execution but
	// not related to the command (i.e. the workspace does not exist) and the
	// errors that were caused by the command itself (e.g. != 0 return code). If
	// the latter, an instance of ErrExec with the status code must be returned.
	Exec(ctx context.Context, name string, args *ExecArgs) (chan bool, error)
}
