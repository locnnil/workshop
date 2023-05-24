package util

import (
	crypto_rand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

var (
	ErrCancelled              = fmt.Errorf("LXD operation cancelled by user")
	ErrForcedCancel           = fmt.Errorf("LXD operation forcefully cancelled by user")
	ErrNoRelativePathsAllowed = errors.New("an absolute path must be used to refer to a project")
)

var (
	DataDir, SdksDir, StateDir, WorkspaceSdksDir string
)

// defaultWorkspaceDir is the Workspace directory used if $PEBBLE is not set. It is
// created by the daemon ("workspaced run") if it doesn't exist, and also used by
// the workspace client.
const defaultWorkspaceDir = "/var/lib/workspace/default"

type WorkspaceState int
type WorkspaceStateReason int
type WorkspaceHookType int

const (
	Off WorkspaceState = iota
	Ready
	Stopped
	Pending
	Error
)

var EvalSymlinks = filepath.EvalSymlinks

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

func ToPathname(dir, name string) string {
	return filepath.Join(dir, ToFileName(name))
}

func ToFileName(name string) string {
	return fmt.Sprintf(".workspace.%s.yaml", name)
}

func ToInstanceName(name string, project_id string) string {
	return fmt.Sprintf("%s-%s", name, project_id)
}

func ToWorkspaceName(instance string) string {
	idx := strings.LastIndex(instance, "-")
	if idx == -1 {
		return ""
	}

	// drop the project id from the name
	return instance[:idx]
}

func ToCurrentPath(sdkName string) string {
	return filepath.Join(WorkspaceSdksDir, sdkName, "current")
}

func ToHooksPath(sdkName string) string {
	return filepath.Join(WorkspaceSdksDir, sdkName, "current", "hooks")
}

func CleanProjectPath(path string) (string, error) {
	var err error
	if !filepath.IsAbs(path) {
		return "", ErrNoRelativePathsAllowed
	}

	path, err = EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return path, nil
}

func GetEnvPaths() (workspaceDir string, socketPath string) {
	workspaceDir = os.Getenv("WORKSPACE")
	if workspaceDir == "" {
		workspaceDir = defaultWorkspaceDir
	}
	socketPath = os.Getenv("WORKSPACE_SOCKET")
	if socketPath == "" {
		socketPath = filepath.Join(workspaceDir, ".workspace.socket")
	}
	return workspaceDir, socketPath
}

func init() {
	var b [8]byte
	_, err := crypto_rand.Read(b[:])
	if err != nil {
		panic("cannot seed math/rand package")
	}
	rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))

	xdg.Reload()
	DataDir = filepath.Join(xdg.DataHome, "workspace")
	StateDir = filepath.Join(xdg.StateHome, "workspace")
	SdksDir = filepath.Join(DataDir, "sdk")

	WorkspaceSdksDir = "/var/lib/workspace/sdk/"

	if err := os.MkdirAll(SdksDir, 0755); err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(StateDir, 0755); err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}
}
