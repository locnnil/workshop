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
	CreateOrLoadProject(ctx context.Context, path string) (*Project, bool, error)
	Projects(ctx context.Context) (map[string]*Project, error)

	LaunchWorkspace(ctx context.Context, name, base string) error
	DeleteWorkspace(ctx context.Context, name string, forceful bool) error
	RenameWorkspace(ctx context.Context, current, new string) error
	SetWorkspaceState(ctx context.Context, name, action string) error

	AddWorkspaceDevice(ctx context.Context, name string, props WorkspaceDevice) error
	RemoveWorkspaceDevice(ctx context.Context, name string, props string) error

	AddWorkspaceConfig(ctx context.Context, name string, item *WorkspaceConfigValue) error
	RemoveWorkspaceConfig(ctx context.Context, name string, key string) error

	GetWorkspace(ctx context.Context, name string) (*Workspace, error)
	GetWorkspaceFs(ctx context.Context, name string) (WorkspaceFs, error)
	GetProjectWorkspaces(ctx context.Context) ([]*WorkspaceFile, []*Workspace, error)

	Exec(ctx context.Context, name string, args *ExecArgs) (chan bool, error)
}
