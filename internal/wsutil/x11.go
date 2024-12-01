package wsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/systemd"
	"github.com/canonical/workshop/internal/workshop"
)

// Copies the user's $XAUTHORITY file to the Workshop run directory.
// This is used by interfaces that require an X11 socket.
func CopyXauthority(user string) error {
	usr, err := workshop.LookupUsername(user)
	if err != nil {
		return err
	}

	env, err := systemd.UserEnvironment(usr)
	if err != nil {
		return err
	}

	xauth, _ := env["XAUTHORITY"]
	if xauth == "" {
		return err
	}

	destDir := filepath.Join(dirs.WorkshopdRunDir, usr.Uid)
	if !osutil.IsDir(destDir) {
		os.MkdirAll(destDir, 0744)
	}

	// Make sure the target file doesn't exist as a directory.
	// We do this because if the file doesn't exist and the desktop interface is
	// mounted, workshopd will create an empty directory as part of the mount
	// interface.
	// This then breaks any 'normal' file operations
	//
	// There's no advantage to comparing the files, destroy and copy every
	// time
	destFile := filepath.Join(destDir, ".Xauthority")
	os.RemoveAll(destFile)

	// Make sure the user owns the file.
	// Important to make sure that the user cannot steal another user's
	// Xauthority file
	// See migrateXauthority function inside of snapd for more info
	f, err := os.Stat(xauth)
	if err != nil {
		return err
	}
	sys := f.Sys()
	if sys == nil {
		return fmt.Errorf("cannot validate owner of file %s", f.Name())
	}
	if fmt.Sprintf("%d", sys.(*syscall.Stat_t).Uid) != usr.Uid {
		return fmt.Errorf("Xauthority file isn't owned by the current user %s", usr.Uid)
	}

	err = osutil.CopyFile(xauth, destFile, osutil.CopyFlagOverwrite)
	if err != nil {
		return err
	}

	return nil
}
