package osutil

import (
	"syscall"
)

func Rename(olddir, newdir string) error {
	return syscall.Rename(olddir, newdir)
}
