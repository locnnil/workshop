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

type WorkspaceErrorType int

const (
	WorkspaceStateDir = "/var/lib/workspace/state"
)

type WorkspaceState int

const (
	WorkspaceOff WorkspaceState = iota
	WorkspaceReady
	WorkspaceStopped
	WorkspacePending
	WorkspaceError
)

func (s WorkspaceState) String() string {
	return [...]string{"Off", "Ready", "Stopped", "Pending", "Error"}[s]
}

func ParseWorkspaceState(s string) WorkspaceState {
	refreshMap := map[string]WorkspaceState{
		WorkspaceOff.String():     WorkspaceOff,
		WorkspaceReady.String():   WorkspaceReady,
		WorkspaceStopped.String(): WorkspaceStopped,
		WorkspacePending.String(): WorkspacePending,
		WorkspaceError.String():   WorkspaceError,
	}
	return refreshMap[s]
}

func NewWorkspace(backend WorkspaceBackend, name, projectId string) *Workspace {
	return &Workspace{
		Name:      name,
		projectId: projectId,
		backend:   backend,
	}
}

type Workspace struct {
	backend   WorkspaceBackend
	file      *WorkspaceFile
	projectId string

	Name    string
	Devices map[string]map[string]string
	content map[string]*sdk.SdkInfo
	errs    []WorkspaceErrorType
	running bool
	state   WorkspaceState
}

func (w *Workspace) IsRunning() bool {
	return w.running
}

func (w *Workspace) SetRunning(run bool) {
	w.running = run
}

func (w *Workspace) ProjectId() string {
	return w.projectId
}

func (w *Workspace) Errors() []WorkspaceErrorType {
	return w.errs
}

func (w *Workspace) AddError(err WorkspaceErrorType) {
	w.errs = append(w.errs, err)
}

func (w *Workspace) Content() []*sdk.SdkInfo {
	return maps.Values(w.content)
}

func (w *Workspace) File() *WorkspaceFile {
	return w.file
}

func (w *Workspace) State() WorkspaceState {
	return w.state
}

func (w *Workspace) SetState(st WorkspaceState) {
	w.state = st
}

func (w *Workspace) LinkSdk(ctx context.Context, s *sdk.SdkInfo) error {
	w.content[s.Name] = s

	sequenceValue, err := json.Marshal(w.content)
	if err != nil {
		return err
	}

	err = w.backend.AddWorkspaceConfig(ctx, w.Name,
		&WorkspaceConfigValue{
			Name:  "user.workspace.content",
			Value: string(sequenceValue),
		})

	if err != nil {
		return err
	}

	/* Update the current link to point out to the newly installed SDK */
	sdkPath := filepath.Join(sdk.WorkspaceSdksDir, s.Name)

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
			Name:  "user.workspace.content",
			Value: string(newSequence),
		})
	if err != nil {
		return err
	}

	/* Remove the 'current' link */
	fs, err := w.backend.GetWorkspaceFs(ctx, w.Name)
	if err != nil {
		return err
	}
	defer fs.Close()

	return fs.Remove(sdk.SdkCurrentPath(s.Name))
}

const (
	None WorkspaceErrorType = iota
	MissingProject
	MissingFile
	BrokenSdkRecord
	WaitOnError
)

func (s WorkspaceErrorType) String() string {
	return [...]string{"", "missing-project", "missing-file", "invalid-sdk", "wait-on-error"}[s]
}

func ParseWorkspaceError(s string) WorkspaceErrorType {
	wserrs := map[string]WorkspaceErrorType{
		None.String():            None,
		MissingProject.String():  MissingProject,
		MissingFile.String():     MissingFile,
		BrokenSdkRecord.String(): BrokenSdkRecord,
		WaitOnError.String():     WaitOnError,
	}
	return wserrs[s]
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

func WorkspaceStateVolumeName(ws, pid string) string {
	return fmt.Sprintf("%s-state-volume", InstanceName(ws, pid))
}
