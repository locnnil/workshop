package systemd

import (
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/osutil"
)

var userLookup = user.Lookup

func UserEnvironment(usr string) (map[string]string, error) {
	user, err := userLookup(usr)
	if err != nil {
		return nil, err
	}

	uid, _, err := osutil.UidGid(user)
	if err != nil {
		return nil, err
	}

	// Systemd is responsible for generating many environment
	// variables. Because of this we must parse the environment as it's set by
	// systemd.
	cmd := exec.Command("sudo", "-E", "-u", user.Username, "systemctl", "--user", "show-environment")
	// XDG_RUNTIME_DIR may not be set if a command invoked by sudo or
	// systemd-run; set it here to the default location. It is required for the
	// systemctl to work with --user. See:
	// https://unix.stackexchange.com/questions/346841/why-does-sudo-i-not-set-xdg-runtime-dir-for-the-target-user
	defaultXdg := filepath.Join(dirs.XdgRuntimeDirBase, strconv.FormatUint(uint64(uid), 10))
	cmd.Env = append(cmd.Env, "XDG_RUNTIME_DIR="+defaultXdg)
	out, errOut, err := osutil.RunCmd(cmd)
	if err != nil {
		return nil, fmt.Errorf(string(errOut))
	}

	rawEnv := strings.FieldsFunc(string(out), func(r rune) bool { return r == '\n' })
	return osutil.ParseEnvironment(rawEnv)
}
