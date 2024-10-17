package osutil

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func Rename(olddir, newdir string) error {
	return syscall.Rename(olddir, newdir)
}

func Exchange(olddir, newdir string) error {
	return unix.Renameat2(-1, olddir, -1, newdir, unix.RENAME_EXCHANGE)
}
