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
	"github.com/canonical/workshop/internal/osutil/sys"
)

var userLookup = user.Lookup

// Copies the user's $XAUTHORITY file to the Workshopd run directory.
func MigrateXauthority(user *user.User, xauth string) (err error) {
	if xauth == "" {
		return errors.New("xauth cannot be empty")
	}

	destDir := filepath.Join(dirs.WorkshopdRunDir, user.Uid, "Xauthority")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// We are performing a Stat() here to ensure that the user can't steal
	// another user's Xauthority file. Note that while Stat() uses fstat() on the
	// file descriptor created during Open(), the file might have changed
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
	fsys := f.Sys()
	if fsys == nil {
		return fmt.Errorf("cannot validate owner of file %s", f.Name())
	}
	// cheap comparison as the current uid is only available as a string
	// but it is better to convert the uid from the stat result to a
	// string than a string into a number.
	if fmt.Sprintf("%d", fsys.(*syscall.Stat_t).Uid) != user.Uid {
		return fmt.Errorf("Xauthority file isn't owned by the current user %s", user.Uid)
	}

	destFile := filepath.Join(destDir, ".Xauthority")
	err = osutil.CopyFile(xauth, filepath.Join(destDir, ".Xauthority"), osutil.CopyFlagOverwrite)
	if err != nil {
		return err
	}

	uid, gid, err := osutil.UidGid(user)
	if err != nil {
		return err
	}

	if err = sys.ChownPath(destFile, uid, gid); err != nil {
		return err
	}

	return nil
}
