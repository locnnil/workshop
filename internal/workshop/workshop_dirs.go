package workshop

import (
	"path/filepath"

	"github.com/canonical/workshop/internal/osutil"
)

func UserDataRootDir(name string) (string, error) {
	usr, env, err := osutil.UserAndEnv(name)
	if err != nil {
		return "", err
	}

	path := filepath.Join(usr.HomeDir, ".local", "share")
	dataDir := env["XDG_DATA_HOME"]
	if dataDir != "" {
		path = dataDir
	}

	return filepath.Join(path, "workshop"), nil
}

func ProjectUserData(userDataDir, pid string) string {
	return filepath.Join(userDataDir, "id", pid)
}

func UserData(userDataDir, pid, w string) string {
	return filepath.Join(ProjectUserData(userDataDir, pid), w)
}

func SdkMountDir(userDataDir, pid, w, sdk string) string {
	return filepath.Join(UserData(userDataDir, pid, w), "mount", sdk)
}

func SdkMountHostSource(userDataDir, pid, w, sdk, plug string) string {
	return filepath.Join(SdkMountDir(userDataDir, pid, w, sdk), plug)
}

func SketchSdkDir(userDataDir, pid, w string) string {
	return filepath.Join(UserData(userDataDir, pid, w), "sdk", "sketch")
}

func SketchSdkCurrent(userDataDir, pid, w string) string {
	return filepath.Join(SketchSdkDir(userDataDir, pid, w), "current")
}

func SketchSdkStash(userDataDir, pid, w string) string {
	return filepath.Join(SketchSdkDir(userDataDir, pid, w), "stash")
}
