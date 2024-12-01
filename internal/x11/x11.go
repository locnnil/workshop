package x11

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"syscall"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/osutil"
)

var userLookup = user.Lookup

// Copies the user's $XAUTHORITY file to the Workshopd run directory.
func MigrateXauthority(user *user.User, xauth string) (err error) {
	if xauth == "" {
		return errors.New("xauth cannot be empty")
	}

	destDir := filepath.Join(dirs.WorkshopdRunDir, user.Uid)
	if !osutil.IsDir(destDir) {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return err
		}
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
	if err = os.RemoveAll(destFile); err != nil {
		return err
	}

	// We are performing a Stat() here to ensure that the user can't steal
	// another user's Xauthority file. Note that while Stat() uses fstat() on the
	// file descriptor created during Open(), the file might have change
	// ownership between the Open() and the Stat(). That's ok because we aren't
	// trying to block access that the user already has: if the user has the
	// privileges to chown another user's Xauthority file, we won't block that
	// since the user can just steal it without having to use workshop. This code
	// is just to ensure that a user who doesn't have those privileges can't
	// steal the file via 'workshop connect'
	f, err := os.Stat(xauth)
	if err != nil {
		return err
	}
	sys := f.Sys()
	if sys == nil {
		return fmt.Errorf("cannot validate owner of file %s", f.Name())
	}
	// cheap comparison as the current uid is only available as a string
	// but it is better to convert the uid from the stat result to a
	// string than a string into a number.
	if fmt.Sprintf("%d", sys.(*syscall.Stat_t).Uid) != user.Uid {
		return fmt.Errorf("Xauthority file isn't owned by the current user %s", user.Uid)
	}

	err = osutil.CopyFile(xauth, destFile, osutil.CopyFlagOverwrite)
	if err != nil {
		return err
	}

	return nil
}
