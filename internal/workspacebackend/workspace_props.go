package workspacebackend

import (
	"fmt"
	"path/filepath"
	"strings"
)

type WorkspaceState int
type WorkspaceStateReason int
type WorkspaceHookType int

const WorkspaceSdksDir = "/var/lib/workspace/sdk/"

type WorkspaceProps struct {
	Name    string
	Devices map[string]map[string]string
	Config  map[string]string

	state  WorkspaceState
	reason WorkspaceStateReason
}

func (w *WorkspaceProps) State() WorkspaceState {
	return w.state
}

func (w *WorkspaceProps) Reason() WorkspaceStateReason {
	return w.reason
}

func (w *WorkspaceProps) SetState(s WorkspaceState, r WorkspaceStateReason) {
	w.state, w.reason = s, r
}

const (
	Off WorkspaceState = iota
	Ready
	Stopped
	Pending
	Error
)

func (s WorkspaceState) String() string {
	return [...]string{"Off", "Ready", "Stopped", "Pending", "Error"}[s]
}

const (
	None WorkspaceStateReason = iota
	Unknown
	MissingProject
	MissingFile
)

func (s WorkspaceStateReason) String() string {
	return [...]string{"", "", "missing-project", "missing-file"}[s]
}

const (
	SetupBase WorkspaceHookType = iota
)

func (s WorkspaceHookType) String() string {
	return [...]string{"setup-base"}[s]
}

func WorkspaceFilePath(dir, name string) string {
	return filepath.Join(dir, WorkspaceFileName(name))
}

func WorkspaceFileName(name string) string {
	return fmt.Sprintf(".workspace.%s.yaml", name)
}

func InstanceName(name string, project_id string) string {
	return fmt.Sprintf("%s-%s", name, project_id)
}

func WorkspaceName(instance string) string {
	idx := strings.LastIndex(instance, "-")
	if idx == -1 {
		return ""
	}

	// drop the project id from the name
	return instance[:idx]
}

func SdkCurrentPath(sdkName string) string {
	return filepath.Join(WorkspaceSdksDir, sdkName, "current")
}

func SdkHooksPath(sdkName string) string {
	return filepath.Join(WorkspaceSdksDir, sdkName, "current", "hooks")
}
