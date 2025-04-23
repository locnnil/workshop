package workshop

import (
	"path/filepath"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/sdk"
)

func UserDataRootDir(homedir string, env map[string]string) string {
	// Use $HOME in preference of the user's home directory, if set.
	// https://specifications.freedesktop.org/basedir-spec/latest/
	if env["HOME"] != "" {
		homedir = env["HOME"]
	}

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

func LocalSdkDir(userDataDir, pid, w, name string) string {
	return filepath.Join(UserData(userDataDir, pid, w), "sdk", name)
}

func LocalSdkRevision(userDataDir, pid, w, name string, revision sdk.Revision) string {
	return filepath.Join(LocalSdkDir(userDataDir, pid, w, name), revision.String())
}

func SketchSdkDir(userDataDir, pid, w string) string {
	return LocalSdkDir(userDataDir, pid, w, sdk.Sketch)
}

func SketchSdkCurrent(userDataDir, pid, w string) string {
	return filepath.Join(SketchSdkDir(userDataDir, pid, w), "current")
}

func SketchSdkStash(userDataDir, pid, w string) string {
	return filepath.Join(SketchSdkDir(userDataDir, pid, w), "stash")
}

func ProjectCacheDir(pid string) string {
	return filepath.Join(dirs.CacheDir, "id", pid)
}

func CacheDir(pid, w string) string {
	return filepath.Join(ProjectCacheDir(pid), w)
}

func AptCacheDir(pid, w string) string {
	return filepath.Join(CacheDir(pid, w), "apt")
}
