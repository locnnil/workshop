package sdk

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"golang.org/x/exp/slices"

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
func CommitRevision(u *user.User, source, target string, installed Revision) (Revision, error) {
	revisions, err := listRevisions(target)
	if err != nil {
		return Revision{}, err
	}

	temp, rev, err := copyRevision(u, source, target)
	if err != nil {
		return Revision{}, err
	}
	defer rev.Fail()

	revision, exists := nextRevision(temp, target, revisions)
	if exists {
		return revision, nil
	}

	if err := moveRevision(temp, target, revision); err != nil {
		return Revision{}, err
	}
	rev.Success()
	if err := commitRevision(target); err != nil {
		return Revision{}, err
	}

	remove := len(revisions) - 2
	revisions = slices.DeleteFunc(revisions, func(rev Revision) bool { return rev == installed })
	removeOldRevisions(target, revisions, remove)

	return revision, nil
}

func listRevisions(path string) ([]Revision, error) {
	recs, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	revisions := make([]Revision, 0, len(recs))
	for _, rec := range recs {
		name := rec.Name()
		revision, err := ParseRevision(name)
		if err == nil && revision.Local() && revision.String() == name {
			revisions = append(revisions, revision)
		}
	}
	return revisions, nil
}

func copyRevision(u *user.User, source, target string) (string, *revert.Reverter, error) {
	uid, gid, err := osutil.UidGid(u)
	if err != nil {
		return "", nil, err
	}
	if err := osutil.MkdirAllChown(target, os.ModePerm, uid, gid); err != nil {
		return "", nil, err
	}

	rev := revert.New()
	defer rev.Fail()

	temp, err := os.MkdirTemp(target, "copy-*")
	if err != nil {
		return "", nil, err
	}
	rev.Add(func() { _ = os.RemoveAll(temp) })

	if err := osutil.CopyAllChown(source, temp, uid, gid); err != nil {
		return "", nil, err
	}

	if err := fixLayout(source, temp, uid, gid); err != nil {
		return "", nil, err
	}

	clone := rev.Clone()
	rev.Success()
	return temp, clone, nil
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

func nextRevision(source, target string, revisions []Revision) (Revision, bool) {
	if len(revisions) == 0 {
		return R(-1), false
	}

	for _, revision := range revisions {
		dir := filepath.Join(target, revision.String())
		// FIXME: should drop privileges for this. There's a (small) chance that
		// unauthorised users can brute force the source directory contents.
		if osutil.DirsAreEqual(source, dir) {
			// Extend revision lifetime.
			if err := os.Chtimes(dir, time.Time{}, time.Now()); err != nil {
				logger.Noticef("Cannot update revision timestamp: %v", err)
			}
			return revision, true
		}
	}

	revision := slices.MinFunc(revisions, func(a, b Revision) int { return cmp.Compare(a.N, b.N) })
	return Revision{N: revision.N - 1}, false
}

func moveRevision(source, target string, revision Revision) error {
	return os.Rename(source, filepath.Join(target, revision.String()))
}

func commitRevision(target string) error {
	d, err := os.Open(target)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func removeOldRevisions(path string, revisions []Revision, remove int) {
	if remove <= 0 {
		return
	}

	times := make([]revTime, 0, len(revisions))
	for _, revision := range revisions {
		var modTime time.Time
		info, err := os.Stat(filepath.Join(path, revision.String()))
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
		if err := os.RemoveAll(filepath.Join(path, time.revision.String())); err != nil {
			logger.Noticef("Cannot remove old revision: %v", err)
		}
	}
}

type revTime struct {
	revision Revision
	modTime  time.Time
}
