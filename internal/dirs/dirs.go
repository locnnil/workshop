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

var (
	// defaultWorkshopdDir is the Workshop directory used if $WORKSHOP is not set. It is
	// created by the daemon ("workshopd run") if it doesn't exist, and also used by
	// the workshop client.
	defaultWorkshopdDir = "/var/lib/workshop/default"

	// base directory inside a workshop
	WorkshopBaseDir = "/var/lib/workshop"

	// SDKs directory to install an SDK in a workshop
	WorkshopSdksDir = filepath.Join(WorkshopBaseDir, "sdk")
)

var (
	SdkDir     string
	StateDir   string
	BaseDir    string
	SocketPath string
)

func getEnvPaths() (workshopdDir string, socketPath string) {
	workshopdDir = os.Getenv("WORKSHOP")
	if workshopdDir == "" {
		workshopdDir = defaultWorkshopdDir
	}
	socketPath = os.Getenv("WORKSHOP_SOCKET")
	if socketPath == "" {
		socketPath = filepath.Join(workshopdDir, ".workshop.socket")
	}
	return workshopdDir, socketPath
}

func init() {
	BaseDir, SocketPath = getEnvPaths()
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
