package dirs

import (
	crypto_rand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

const (
	// defaultWorkspaceDir is the Workspace directory used if $WORKSPACE is not set. It is
	// created by the daemon ("workspaced run") if it doesn't exist, and also used by
	// the workspace client.
	defaultWorkspaceDir = "/var/lib/workspace/default"

	// default root directory path for the SDKs to be installed into in a workspace
	WorkspaceSdksDir = "/var/lib/workspace/sdk"
)

var (
	SdkDir          string
	StateDir        string
	BaseDir         string
	WorkspaceSocket string
)

func getEnvPaths() (workspaceDir string, socketPath string) {
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
	BaseDir, WorkspaceSocket = getEnvPaths()
	SetRootDir(BaseDir)

	var b [8]byte
	_, err := crypto_rand.Read(b[:])
	if err != nil {
		panic("cannot seed math/rand package")
	}
	rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))
}

func SetRootDir(rootdir string) {
	if !filepath.IsAbs(rootdir) {
		panic(fmt.Sprintf("supplied path is not absolute %q", rootdir))
	}

	StateDir = rootdir
	SdkDir = filepath.Join(rootdir, "sdk")
}

func CreateDirs() error {
	localFs := afero.NewOsFs()

	err := localFs.MkdirAll(StateDir, 0755)
	if err != nil {
		return err
	}

	if err := localFs.MkdirAll(SdkDir, 0755); err != nil {
		return err
	}
	return nil
}
