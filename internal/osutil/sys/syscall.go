// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (c) 2017 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package sys

import (
	"cmp"
	"errors"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// FlagID can be passed to chown-ish functions to mean "no change",
// and can be returned from getuid-ish functions to mean "not found".
const FlagID = 1<<32 - 1

// UserID is the type of the system's user identifiers (in C, uid_t).
//
// We give it its own explicit type so you don't have to remember that
// it's a uint32 (which lead to the bug this package fixes in the
// first place)
type UserID uint32

// GroupID is the type of the system's group identifiers (in C, gid_t).
type GroupID uint32

// uid_t is an unsigned 32-bit integer in linux right now.
// so syscall.Gete?[ug]id are wrong, and break in 32 bits
// (see https://github.com/golang/go/issues/22739)
func Getuid() UserID {
	return UserID(getid(_SYS_GETUID))
}

func Geteuid() UserID {
	return UserID(getid(_SYS_GETEUID))
}

func Getgid() GroupID {
	return GroupID(getid(_SYS_GETGID))
}

func Getegid() GroupID {
	return GroupID(getid(_SYS_GETEGID))
}

func getid(id uintptr) uint32 {
	// these are documented as not failing, but see golang#22924
	r0, _, errno := syscall.RawSyscall(id, 0, 0, 0)
	if errno != 0 {
		return uint32(-errno)
	}
	return uint32(r0)
}

func Chown(f *os.File, uid UserID, gid GroupID) error {
	return Fchown(int(f.Fd()), uid, gid)
}

func Fchown(fd int, uid UserID, gid GroupID) error {
	return syscall.Fchown(fd, int(uid), int(gid))
}

func ChownPath(path string, uid UserID, gid GroupID) error {
	return syscall.Chown(path, int(uid), int(gid))
}

func LchownPath(path string, uid UserID, gid GroupID) error {
	return syscall.Lchown(path, int(uid), int(gid))
}

func FileOwner(info os.FileInfo) (UserID, GroupID, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, errors.New("user and group unavailable")
	}
	return UserID(stat.Uid), GroupID(stat.Gid), nil
}

// AccessTime returns the file access time if available.
func AccessTime(info os.FileInfo) (time.Time, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, errors.New("file access time unavailable")
	}
	return time.Unix(stat.Atim.Unix()), nil
}

// Lchtimes is like os.Chtimes but doesn't follow symlinks.
func Lchtimes(path string, atime time.Time, mtime time.Time) error {
	time1, err1 := timeToTimespec(atime)
	time2, err2 := timeToTimespec(mtime)
	if err := cmp.Or(err1, err2); err != nil {
		return err
	}
	times := []unix.Timespec{time1, time2}

	return unix.UtimesNanoAt(unix.AT_FDCWD, path, times, unix.AT_SYMLINK_NOFOLLOW)
}

func timeToTimespec(t time.Time) (unix.Timespec, error) {
	if t.IsZero() {
		return unix.Timespec{Sec: unix.UTIME_OMIT, Nsec: unix.UTIME_OMIT}, nil
	}
	return unix.TimeToTimespec(t)
}
