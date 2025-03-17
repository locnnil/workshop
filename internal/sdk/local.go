package sdk

import (
	"cmp"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"golang.org/x/exp/slices"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
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

	revision, exists := nextRevision(source, target, revisions)
	if exists {
		return revision, nil
	}

	uid, gid, err := osutil.UidGid(u)
	if err != nil {
		return Revision{}, err
	}
	if err := osutil.MkdirAllChown(target, os.ModePerm, uid, gid); err != nil {
		return Revision{}, err
	}
	if err := osutil.CopyDirOnBehalf(source, filepath.Join(target, revision.String()), uid, gid); err != nil {
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
