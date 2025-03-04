package workshop

import (
	"os/user"
	"path/filepath"

	"github.com/canonical/workshop/internal/systemd"
)

func userAndEnv(name string) (*user.User, map[string]string, error) {
	usr, err := LookupUsername(name)
	if err != nil {
		return nil, nil, err
	}

	env, err := systemd.UserEnvironment(usr)
	if err != nil {
		return nil, nil, err
	}

	return usr, env, err
}

func UserDataRootDir(name string) (string, error) {
	usr, env, err := userAndEnv(name)
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

func ProjectUserData(rootDir, pid string) string {
	return filepath.Join(rootDir, "id", pid)
}

func UserData(rootDir, pid, w string) string {
	return filepath.Join(ProjectUserData(rootDir, pid), w)
}

func SdkMountDir(rootDir, pid, w, sdk string) string {
	return filepath.Join(UserData(rootDir, pid, w), "mount", sdk)
}

func SdkMountHostSource(rootDir, pid, w, sdk, plug string) string {
	return filepath.Join(SdkMountDir(rootDir, pid, w, sdk), plug)
}

func SketchSdkDir(rootDir, pid, w string) string {
	return filepath.Join(UserData(rootDir, pid, w), "sdk", "sketch")
}

func SketchSdkCurrent(rootDir, pid, w string) string {
	return filepath.Join(SketchSdkDir(rootDir, pid, w), "current")
}

func SketchSdkStash(rootDir, pid, w string) string {
	return filepath.Join(SketchSdkDir(rootDir, pid, w), "stash")
}
