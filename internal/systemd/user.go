package systemd

import (
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/osutil"
)

// Returns the environment for the user as set by systemd.
// This is the equivalent of running 'systemctl --user show-environment'
func UserEnvironment(user *user.User) (map[string]string, error) {
	cmd := exec.Command("sudo", "-E", "-u", user.Username, "systemctl", "--user", "show-environment")
	// XDG_RUNTIME_DIR may not be set if a command invoked by sudo or
	// systemd-run; set it here to the default location. It is required for the
	// systemctl to work with --user. See:
	// https://unix.stackexchange.com/questions/346841/why-does-sudo-i-not-set-xdg-runtime-dir-for-the-target-user
	defaultXdg := filepath.Join(dirs.XdgRuntimeDirBase, user.Uid)
	cmd.Env = append(cmd.Env, "XDG_RUNTIME_DIR="+defaultXdg)
	out, errOut, err := osutil.RunCmd(cmd)
	if err != nil {
		return nil, fmt.Errorf(string(errOut))
	}

	rawEnv := strings.FieldsFunc(string(out), func(r rune) bool { return r == '\n' })
	return osutil.ParseEnvironment(rawEnv)
}
