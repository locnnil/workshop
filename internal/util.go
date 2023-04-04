package util

import (
	"encoding/binary"
	"errors"
	"fmt"

	"os"
	"strings"

	"path/filepath"

	"github.com/adrg/xdg"

	"math/rand"

	crypto_rand "crypto/rand"
)

var (
	ErrCancelled              = fmt.Errorf("LXD operation cancelled by user")
	ErrForcedCancel           = fmt.Errorf("LXD operation forcefully cancelled by user")
	ErrNoRelativePathsAllowed = errors.New("an absolute path must be used to refer to a project")
)

var (
	DataDir, SdksDir, StateDir, WorkspaceSdksDir string
)

type WorkspaceState int
type WorkspaceStateReason int
type WorkspaceHookType int

const (
	Inactive WorkspaceState = iota
	Ready
	Stopped
	Pending
	Error
)

var EvalSymlinks = filepath.EvalSymlinks

func (s WorkspaceState) String() string {
	return [...]string{"Inactive", "Ready", "Stopped", "Pending", "Error"}[s]
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
