// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2014-2024 Canonical Ltd
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

package osutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/canonical/workshop/internal/osutil/sys"
	"github.com/canonical/workshop/internal/revert"
)

// CopyFlag is used to tweak the behaviour of CopyFile
type CopyFlag uint8

const (
	// CopyFlagDefault is the default behaviour
	CopyFlagDefault CopyFlag = 0
	// CopyFlagSync does a sync after copying the files
	CopyFlagSync CopyFlag = 1 << iota
	// CopyFlagOverwrite overwrites the target if it exists
	CopyFlagOverwrite
	// CopyFlagPreserveAll preserves mode,owner,time attributes
	CopyFlagPreserveAll
)

var (
	openfile = doOpenFile
	copyfile = doCopyFile
)

type fileish interface {
	Close() error
	Sync() error
	Fd() uintptr
	Stat() (os.FileInfo, error)
	Read([]byte) (int, error)
	Write([]byte) (int, error)
}

func doOpenFile(name string, flag int, perm os.FileMode) (fileish, error) {
	return os.OpenFile(name, flag, perm)
}

// CopyFile copies src to dst
func CopyFile(src, dst string, flags CopyFlag) (err error) {
	if flags&CopyFlagPreserveAll != 0 {
		// Our native copy code does not preserve all attributes
		// (yet). If the user needs this functionality we just
		// fallback to use the system's "cp" binary to do the copy.
		if err := runCpPreserveAll(src, dst, "copy all"); err != nil {
			return err
		}
		if flags&CopyFlagSync != 0 {
			return runSync()
		}
		return nil
	}

	fin, err := openfile(src, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("unable to open %s: %w", src, err)
	}
	defer func() {
		if cerr := fin.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("when closing %s: %w", src, cerr)
		}
	}()

	fi, err := fin.Stat()
	if err != nil {
		return fmt.Errorf("unable to stat %s: %w", src, err)
	}

	outflags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if flags&CopyFlagOverwrite == 0 {
		outflags |= os.O_EXCL
	}

	fout, err := openfile(dst, outflags, fi.Mode())
	if err != nil {
		return fmt.Errorf("unable to create %s: %w", dst, err)
	}
	defer func() {
		if cerr := fout.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("when closing %s: %w", dst, cerr)
		}
	}()

	if err := copyfile(fin, fout, fi); err != nil {
		return fmt.Errorf("unable to copy %s to %s: %w", src, dst, err)
	}

	if flags&CopyFlagSync != 0 {
		if err = fout.Sync(); err != nil {
			return fmt.Errorf("unable to sync %s: %w", dst, err)
		}
	}

	return nil
}

func runCmd(cmd *exec.Cmd, errdesc string) error {
	if output, err := cmd.CombinedOutput(); err != nil {
		output = bytes.TrimSpace(output)
		if exitCode, err := ExitCode(err); err == nil {
			return &CopySpecialFileError{
				desc:     errdesc,
				exitCode: exitCode,
				output:   output,
			}
		}
		return &CopySpecialFileError{
			desc:   errdesc,
			err:    err,
			output: output,
		}
	}

	return nil
}

func runSync(args ...string) error {
	return runCmd(exec.Command("sync", args...), "sync")
}

func runCpPreserveAll(path, dest, errdesc string) error {
	return runCmd(exec.Command("cp", "-av", path, dest), errdesc)
}

// CopySpecialFile is used to copy all the things that are not files
// (like device nodes, named pipes etc)
func CopySpecialFile(path, dest string) error {
	if err := runCpPreserveAll(path, dest, "copy device node"); err != nil {
		return err
	}
	return runSync(filepath.Dir(dest))
}

// CopySpecialFileError is returned if a special file copy fails
type CopySpecialFileError struct {
	desc     string
	exitCode int
	output   []byte
	err      error
}

func (e CopySpecialFileError) Error() string {
	if e.err == nil {
		return fmt.Sprintf("failed to %s: %q (%v)", e.desc, e.output, e.exitCode)
	}

	return fmt.Sprintf("failed to %s: %q (%v)", e.desc, e.output, e.err)
}

// CopyDirOnBehalf copies a directory into a new location, owned by the provided user.
// It ignores non-regular files, resolves symlinks, preserves permissions and syncs to disk.
// FIXME: the chown behaviour is not currently tested. The plan is to simplify this function
// and run it as the actual user, once we have decided on a uniform approach for this.
func CopyDirOnBehalf(src, dst string, uid sys.UserID, gid sys.GroupID) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := checkOwner(uid, info, src); err != nil {
		return err
	}

	dst, err = filepath.Abs(dst)
	if err != nil {
		return err
	}

	// Fail early and also check dst != "/".
	if FileExists(dst) {
		return &os.PathError{Op: "mkdir", Path: dst, Err: syscall.EEXIST}
	}
	parent := filepath.Dir(dst)

	rev := revert.New()
	defer rev.Fail()

	temp, err := os.MkdirTemp(parent, "copy-*")
	if err != nil {
		return err
	}
	rev.Add(func() { _ = os.RemoveAll(temp) })

	if err := os.Chmod(temp, info.Mode().Perm()); err != nil {
		return err
	}

	if err := chownCopyAndSync(uid, gid, src, temp, []os.FileInfo{info}); err != nil {
		return err
	}

	if err = os.Rename(temp, dst); err != nil {
		return err
	}

	d, err := os.OpenFile(parent, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return err
	}

	rev.Success()
	return nil
}

func checkOwner(uid sys.UserID, info os.FileInfo, path string) error {
	fuid, _, err := sys.FileOwner(info)
	if err != nil {
		return &os.PathError{Op: "stat", Path: path, Err: err}
	}
	if fuid != uid {
		return &os.PathError{Op: "open", Path: path, Err: syscall.EPERM}
	}
	return nil
}

func chownCopyAndSync(uid sys.UserID, gid sys.GroupID, src, dst string, prev []os.FileInfo) error {
	d, err := os.OpenFile(dst, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer d.Close()

	if err := sys.Chown(d, uid, gid); err != nil {
		return &os.PathError{Op: "chown", Path: dst, Err: err}
	}

	names, infos, err := regularFilesAndDirs(src)
	if err != nil {
		return err
	}

	for i, info := range infos {
		from := filepath.Join(src, names[i])
		to := filepath.Join(dst, names[i])

		if err := checkOwner(uid, info, from); err != nil {
			return err
		}

		if info.IsDir() {
			if err := copyDirOnBehalf(uid, gid, from, to, info, prev); err != nil {
				return err
			}
		} else if err := copyFileOnBehalf(uid, gid, from, to, info); err != nil {
			return err
		}
	}

	return d.Sync()
}

func copyDirOnBehalf(uid sys.UserID, gid sys.GroupID, src, dst string, info os.FileInfo, prev []os.FileInfo) error {
	if err := os.Mkdir(dst, info.Mode().Perm()); err != nil {
		return err
	}
	// Need to Chmod, despite passing mode to Mkdir, because of system umask.
	if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
		return err
	}

	if slices.ContainsFunc(prev, func(fi os.FileInfo) bool { return os.SameFile(fi, info) }) {
		return fmt.Errorf("directory %q contains a symlink cycle", dst)
	}
	prev = append(prev, info)
	if err := chownCopyAndSync(uid, gid, src, dst, prev); err != nil {
		return err
	}

	return nil
}

func copyFileOnBehalf(uid sys.UserID, gid sys.GroupID, src, dst string, info os.FileInfo) error {
	r, err := os.Open(src)
	if err != nil {
		return err
	}
	defer r.Close()

	w, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer w.Close()
	// Need to Chmod, despite passing mode to Mkdir, because of system umask.
	if err := w.Chmod(info.Mode().Perm()); err != nil {
		return err
	}

	if err := sys.Chown(w, uid, gid); err != nil {
		return &os.PathError{Op: "chown", Path: dst, Err: err}
	}

	if err := copyfile(r, w, info); err != nil {
		return fmt.Errorf("copy %q to %q: %w", src, dst, err)
	}

	return w.Sync()
}
