// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package dirs

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// defaultBaseDir is the Workshop directory used if $WORKSHOP is not set. It is
	// created by the daemon ("workshopd run") if it doesn't exist, and also used by
	// the workshop client.
	defaultBaseDir = "/var/lib/workshop"

	// defaultCacheDir is the Workshop directory used if $WORKSHOP_CACHE is not
	// set. It is created by the daemon ("workshopd run") if it doesn't exist.
	defaultCacheDir = "/var/cache/workshop"
)

// Variables for paths inside a workshop
var (
	// base directory inside a workshop
	WorkshopBaseDir = defaultBaseDir

	// Directory for mounted binaries (i.e. workshopctl)
	WorkshopGuestBinDir = filepath.Join(WorkshopBaseDir, "bin")

	// SDKs directory to install an SDK in a workshop
	WorkshopSdksDir = filepath.Join(WorkshopBaseDir, "sdk")

	// Base directory for the state storage
	WorkshopStateDir = filepath.Join(WorkshopBaseDir, "state")

	// Run directory inside workshop
	WorkshopRunDir = filepath.Join(WorkshopBaseDir, "run")

	// Path to the daemon's unix socket as seen from inside a workshop. The
	// host daemon's untrusted socket is proxied to this fixed path so that
	// workshopctl and hooks always find it regardless of the daemon's host
	// socket name (which varies, e.g. under "go tool try").
	WorkshopSocketPath = filepath.Join(WorkshopRunDir, "workshop.socket")

	// Directory for actions inside workshop
	WorkshopActionsDir = filepath.Join(WorkshopRunDir, "actions")

	// Cache directory for deb packages
	AptCacheDir = "/var/cache/apt/archives"
)

// Variables for workshopd (host paths)
var (
	// Base directory for workshopd
	BaseDir string
	// Cache directory for workshopd
	CacheDir string
	// Path for workshopctl executable
	WorkshopCtlPath string
	// The directory to store downloaded base images and associated metadata
	BaseDownloads string
	// The directory to store downloaded SDKs
	SdkDownloads string
	// Path to the daemon's unix socket
	SocketPath string
	// State lock file
	WorkshopStateLockFile string
	// Base for the XDG runtime directory of a host user
	XdgRuntimeDirBase string
	// Run directory
	WorkshopdRunDir string
	// Locks directory
	WorkshopdLocksDir string
	// SSH keys
	WorkshopSSHDir string
	// Certificates
	WorkshopTlsDir string
)

func getEnvPaths() (workshopdDir, cacheDir, socketPath string) {
	workshopdDir = os.Getenv("WORKSHOP")
	if workshopdDir == "" {
		workshopdDir = defaultBaseDir
	}
	cacheDir = os.Getenv("WORKSHOP_CACHE")
	if cacheDir == "" {
		cacheDir = defaultCacheDir
	}
	socketPath = os.Getenv("WORKSHOP_SOCKET")
	if socketPath == "" {
		socketPath = filepath.Join(workshopdDir, "workshop.socket")
	}
	return workshopdDir, cacheDir, socketPath
}

func getWorkshopCtlPath() string {
	execPath, err := os.Executable()
	if err != nil {
		panic(fmt.Errorf("cannot get executable path: %w", err))
	}

	// Packages use a dedicated $prefix/lib/workshop/guest directory.
	binDir := filepath.Dir(execPath)
	workshopctl := filepath.Join(filepath.Dir(binDir), "lib", "workshop", "guest", "workshopctl")
	if _, err := os.Stat(workshopctl); err == nil {
		return workshopctl
	}

	// Local development often uses `go install`, which places all binaries in
	// the same directory.
	return filepath.Join(binDir, "workshopctl")
}

func init() {
	XdgRuntimeDirBase = "/run/user"
	BaseDir, CacheDir, SocketPath = getEnvPaths()
	SetRootDir(BaseDir)
	SetCacheDir(CacheDir)
	WorkshopCtlPath = getWorkshopCtlPath()
}

func SetRootDir(rootdir string) {
	if !filepath.IsAbs(rootdir) {
		panic(fmt.Sprintf("cannot set root dir: path %q is not absolute", rootdir))
	}
	BaseDir = rootdir

	WorkshopStateLockFile = filepath.Join(BaseDir, "state.lock")
	WorkshopSSHDir = filepath.Join(BaseDir, "ssh")
	WorkshopTlsDir = filepath.Join(BaseDir, "tls")
	WorkshopdRunDir = filepath.Join(BaseDir, "/run/workshopd")
	WorkshopdLocksDir = filepath.Join(WorkshopdRunDir, "locks")
}

func SetCacheDir(cachedir string) {
	if !filepath.IsAbs(cachedir) {
		panic(fmt.Sprintf("cannot set cache dir: path %q is not absolute", cachedir))
	}
	CacheDir = cachedir

	BaseDownloads = filepath.Join(CacheDir, "base")
	SdkDownloads = filepath.Join(CacheDir, "sdk")
}

func CreateDirs() error {
	if err := os.MkdirAll(BaseDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(CacheDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(BaseDownloads, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(SdkDownloads, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(WorkshopdRunDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(WorkshopdLocksDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(WorkshopTlsDir, 0755); err != nil {
		return err
	}
	return nil
}
