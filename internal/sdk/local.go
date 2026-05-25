// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package sdk

import (
	"cmp"
	"crypto/sha3"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/osutil/sys"
	"github.com/canonical/workshop/internal/revert"
)

// CommitRevision copies the source SDK into a subdirectory of the target, reusing an existing
// subdirectory if possible. The subdirectory name is a local revision number, e.g. x123 for -123.
// If no local revision matches the source, the new revision is one less than the lowest existing
// revision, e.g. -124. After creating a new revision, all but the three most recently installed
// revisions will be removed. The currently installed revision can be passed as an argument,
// to avoid relying too heavily on directory timestamps.
func CommitRevision(u *user.User, source, target string, installed Revision) (Revision, string, error) {
	digest, err := copySdkDir(u, source, target)
	if err != nil {
		return Revision{}, "", err
	}

	revision, err := linkRevision(target, digest, installed)
	if err != nil {
		return Revision{}, "", err
	}

	return revision, digest, nil
}

func copySdkDir(u *user.User, source, target string) (string, error) {
	uid, gid, err := osutil.UidGid(u)
	if err != nil {
		return "", err
	}
	if err := osutil.MkdirAllChown(target, os.ModePerm, uid, gid); err != nil {
		return "", err
	}

	rev := revert.New()
	defer rev.Fail()

	dirfd, err := os.Open(target)
	if err != nil {
		return "", err
	}
	defer dirfd.Close()

	temp, err := os.MkdirTemp(target, "copy-*")
	if err != nil {
		return "", err
	}
	rev.Add(func() { _ = os.RemoveAll(temp) })

	if err := osutil.CopyAllChown(source, temp, uid, gid); err != nil {
		return "", err
	}

	if err := fixLayout(source, temp, uid, gid); err != nil {
		return "", err
	}

	// FIXME: should drop privileges for this. There's a (small) chance that
	// unauthorised users can brute force the source directory contents.
	sum, err := osutil.HashDirEntries(sha3.New384(), temp)
	if err != nil {
		return "", err
	}
	digest := hex.EncodeToString(sum)

	if err := os.Rename(temp, filepath.Join(target, digest)); errors.Is(err, os.ErrExist) {
		return digest, nil
	} else if err != nil {
		return "", err
	}
	rev.Success()

	if err := dirfd.Sync(); err != nil {
		return "", err
	}

	return digest, nil
}

// fixLayout moves sdk.yaml to meta/sdk.yaml and hooks/ to sdk/hooks/.
func fixLayout(source, target string, uid sys.UserID, gid sys.GroupID) error {
	d, err := os.Open(target)
	if err != nil {
		return err
	}
	defer d.Close()

	info, err := d.Stat()
	if err != nil {
		return err
	}

	if err := moveSdkYaml(source, target, info.Mode().Perm(), uid, gid); err != nil {
		return err
	}

	if err := moveHooks(source, target, info.Mode().Perm(), uid, gid); err != nil {
		return err
	}

	return d.Sync()
}

func moveSdkYaml(source, target string, perm os.FileMode, uid sys.UserID, gid sys.GroupID) error {
	short := filepath.Join(target, "sdk.yaml")
	meta := filepath.Join(target, "meta")
	long := filepath.Join(meta, "sdk.yaml")

	if !osutil.FileExists(short) {
		return nil
	}
	if osutil.FileExists(long) {
		return fmt.Errorf("local SDK %q contains both sdk.yaml and meta/sdk.yaml", source)
	}

	if err := mkdirChmodChown(meta, perm, uid, gid); err != nil {
		return err
	}

	d, err := os.Open(meta)
	if err != nil {
		return err
	}
	defer d.Close()

	if err := os.Rename(short, long); err != nil {
		return err
	}

	return d.Sync()
}

func moveHooks(source, target string, perm os.FileMode, uid sys.UserID, gid sys.GroupID) error {
	short := filepath.Join(target, "hooks")
	sdk := filepath.Join(target, "sdk")
	long := filepath.Join(sdk, "hooks")

	exists, isDir, err := osutil.ExistsIsDir(short)
	if err != nil {
		return err
	}
	if !exists || !isDir {
		return nil
	}
	if osutil.FileExists(long) {
		return fmt.Errorf("local SDK %q contains both hooks and sdk/hooks", source)
	}

	if err := mkdirChmodChown(sdk, perm, uid, gid); err != nil {
		return err
	}

	d, err := os.Open(sdk)
	if err != nil {
		return err
	}
	defer d.Close()

	if err := os.Rename(short, long); err != nil {
		return err
	}

	return d.Sync()
}

func mkdirChmodChown(path string, perm os.FileMode, uid sys.UserID, gid sys.GroupID) error {
	if err := os.Mkdir(path, os.ModePerm); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	if err := os.Chmod(path, perm); err != nil {
		return err
	}
	if err := sys.ChownPath(path, uid, gid); err != nil {
		return &os.PathError{Op: "chown", Path: path, Err: err}
	}
	return nil
}

func linkRevision(target, digest string, installed Revision) (Revision, error) {
	dirfd, err := os.Open(target)
	if err != nil {
		return Revision{}, err
	}
	defer dirfd.Close()

	revisions, latest, err := listRevisions(dirfd)
	if err != nil {
		return Revision{}, err
	}

	for _, revision := range revisions {
		path := filepath.Join(target, revision.String())

		link, err := os.Readlink(path)
		if err != nil {
			return Revision{}, err
		}

		if link == digest {
			// Extend revision lifetime.
			if err := sys.Lchtimes(path, time.Time{}, time.Now()); err != nil {
				logger.Noticef("Cannot update revision timestamp: %v", err)
			}
			return revision, nil
		}
	}

	revision := Revision{N: latest.N - 1}
	if err := os.Symlink(digest, filepath.Join(target, revision.String())); err != nil {
		return Revision{}, err
	}

	if err := dirfd.Sync(); err != nil {
		return Revision{}, err
	}

	removeOldRevisions(target, revisions, installed)

	return revision, nil
}

// List all symlinks in the given directory which look like local revisions
// (x1, x2, ...). Also return the latest existing revision (among all files,
// not just symlinks), or the zero revision if there aren't any.
func listRevisions(dirfd *os.File) ([]Revision, Revision, error) {
	recs, err := dirfd.ReadDir(-1)
	if err != nil {
		return nil, Revision{}, err
	}

	revisions := make([]Revision, 0, len(recs))
	latest := Revision{}

	for _, rec := range recs {
		name := rec.Name()
		revision, err := ParseRevision(name)
		if err != nil || !revision.Local() || revision.String() != name {
			continue
		}

		if revision.N < latest.N {
			latest.N = revision.N
		}

		if rec.Type()&os.ModeSymlink != 0 {
			revisions = append(revisions, revision)
		}
	}

	return revisions, latest, nil
}

func removeOldRevisions(target string, revisions []Revision, installed Revision) {
	remove := len(revisions) - 2
	if remove <= 0 {
		return
	}
	revisions = slices.DeleteFunc(revisions, func(rev Revision) bool { return rev == installed })

	times := make([]revTime, 0, len(revisions))
	for _, revision := range revisions {
		var modTime time.Time
		info, err := os.Lstat(filepath.Join(target, revision.String()))
		if err != nil {
			// Broken revision, use zero modTime so it is removed first.
			logger.Noticef("Cannot read revision timestamp: %v", err)
		} else {
			modTime = info.ModTime()
		}
		times = append(times, revTime{revision, modTime})
	}

	slices.SortFunc(times, func(a, b revTime) int { return a.modTime.Compare(b.modTime) })
	times = times[:remove]
	for _, time := range times {
		if err := removeRevision(target, time.revision.String()); err != nil {
			logger.Noticef("Cannot remove old revision: %v", err)
		}
	}
}

func removeRevision(target string, revision string) (err error) {
	path := filepath.Join(target, revision)

	digest, err := os.Readlink(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// Ensure we only delete actual SDKs and ignore malicious symlinks.
	if strings.ContainsRune(digest, os.PathSeparator) {
		return &os.PathError{Op: "remove", Path: path, Err: fmt.Errorf("invalid hash %q", digest)}
	}

	temp, err := os.MkdirTemp(target, "remove-*")
	if err != nil {
		return err
	}
	defer func() {
		if err1 := os.RemoveAll(temp); err1 != nil {
			err = cmp.Or(err, err1)
		}
	}()

	if err := os.Rename(filepath.Join(target, digest), filepath.Join(temp, digest)); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

type revTime struct {
	revision Revision
	modTime  time.Time
}
