package dirs

import (
	crypto_rand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
)

// defaultBaseDir is the Workshop directory used if $WORKSHOP is not set. It is
// created by the daemon ("workshopd run") if it doesn't exist, and also used by
// the workshop client.
const defaultBaseDir = "/var/lib/workshop"

// Variables for paths inside a workshop
var (
	// base directory inside a workshop
	WorkshopBaseDir = defaultBaseDir

	// SDKs directory to install an SDK in a workshop
	WorkshopSdksDir = filepath.Join(WorkshopBaseDir, "sdk")

	// Base directory for the workshop state storage
	WorkshopStateDir = "/var/lib/workshop/state"
)

// Variables for workshopd (host paths)
var (
	// Work directory
	ExecDir string
	// Base directory for workshopd
	BaseDir string
	// The directory to store downloaded SDKs
	SdkDir string
	// Path to the daemon's unix socket
	SocketPath string
	// Base for the XDG runtime directory of a host user
	XdgRuntimeDirBase string
)

func getEnvPaths() (workshopdDir string, socketPath string) {
	workshopdDir = os.Getenv("WORKSHOP")
	if workshopdDir == "" {
		workshopdDir = defaultBaseDir
	}
	socketPath = os.Getenv("WORKSHOP_SOCKET")
	if socketPath == "" {
		socketPath = filepath.Join(workshopdDir, ".workshop.socket")
	}
	return workshopdDir, socketPath
}

func init() {
	var err error
	var execPath string
	execPath, err = os.Executable()
	if err != nil {
		panic("cannot get working directory")
	}

	ExecDir = filepath.Dir(execPath)
	XdgRuntimeDirBase = "/run/user"
	BaseDir, SocketPath = getEnvPaths()
	SetRootDir(BaseDir)

	var b [8]byte
	_, err = crypto_rand.Read(b[:])
	if err != nil {
		panic("cannot seed math/rand package")
	}
	rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))
}

func SetRootDir(rootdir string) {
	if !filepath.IsAbs(rootdir) {
		panic(fmt.Sprintf("cannot set root dir: path %q is not absolute", rootdir))
	}

	SdkDir = filepath.Join(rootdir, "sdk")
}

func CreateDirs() error {
	if err := os.MkdirAll(BaseDir, 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(SdkDir, 0755); err != nil {
		return err
	}
	return nil
}
