package workspacebackend

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/canonical/workspace/internal/sdk"
	"golang.org/x/exp/maps"
)

type WorkspaceState int
type WorkspaceStateReason int
type WorkspaceHookType int

const WorkspaceSdksDir = "/var/lib/workspace/sdk/"

type Workspace struct {
	backend WorkspaceBackend
	Name    string
	Devices map[string]map[string]string
	content map[string]*sdk.SdkInfo
	file    *WorkspaceFile

	state  WorkspaceState
	reason WorkspaceStateReason
}

func (w *Workspace) State() WorkspaceState {
	return w.state
}

func (w *Workspace) Reason() WorkspaceStateReason {
	return w.reason
}

func (w *Workspace) SetState(s WorkspaceState, r WorkspaceStateReason) {
	w.state, w.reason = s, r
}

func (w *Workspace) Content() []*sdk.SdkInfo {
	return maps.Values(w.content)
}

func (w *Workspace) File() *WorkspaceFile {
	return w.file
}

func (w *Workspace) LinkSdk(ctx context.Context, s *sdk.SdkInfo) error {
	w.content[s.Name] = s

	sequenceValue, err := json.Marshal(w.content)
	if err != nil {
		return err
	}

	err = w.backend.AddWorkspaceConfig(ctx, w.Name,
		&WorkspaceConfigValue{
			Name:  "user.workspace.sdk",
			Value: string(sequenceValue),
		})

	if err != nil {
		return err
	}

	/* Update the current link to point out to the newly installed SDK */
	sdkPath := filepath.Join(WorkspaceSdksDir, s.Name)

	fs, err := w.backend.GetWorkspaceFs(ctx, w.Name)
	if err != nil {
		return err
	}
	defer fs.Close()

	return fs.Symlink(filepath.Join(sdkPath, strconv.Itoa(int(s.Revision))),
		filepath.Join(sdkPath, "current"), true)
}

func (w *Workspace) UnlinkSdk(ctx context.Context, s *sdk.SdkInfo) error {
	delete(w.content, s.Name)
	newSequence, err := json.Marshal(w.content)
	if err != nil {
		return err
	}

	/* Update the workspace config */
	err = w.backend.AddWorkspaceConfig(ctx, w.Name,
		&WorkspaceConfigValue{
			Name:  "user.workspace.sdk",
			Value: string(newSequence),
		})
	if err != nil {
		return err
	}

	/* Remove the 'current' link */
	fs, err := w.backend.GetWorkspaceFs(ctx, s.Name)
	if err != nil {
		return err
	}
	defer fs.Close()

	return fs.Remove(SdkCurrentPath(s.Name))
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
	BrokenSdkRecord
)

func (s WorkspaceStateReason) String() string {
	return [...]string{"", "", "missing-project", "missing-file", "invalid-sdk"}[s]
}

const (
	SetupBase WorkspaceHookType = iota
	SaveState
	RestoreState
)

func (s WorkspaceHookType) String() string {
	return [...]string{"setup-base", "save-state", "restore-state"}[s]
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
