package workshop

import (
	"path/filepath"
)

func UserDataRootDir(homedir string, env map[string]string) string {
	path := filepath.Join(homedir, ".local", "share")
	dataDir := env["XDG_DATA_HOME"]
	if dataDir != "" {
		path = dataDir
	}

	return filepath.Join(path, "workshop")
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
