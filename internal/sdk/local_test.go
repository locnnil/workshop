package sdk_test

import (
	"os"
	"os/user"
	"path/filepath"
	"time"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/osutil/sys"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
)

type localSdk struct {
	target string
	user   *user.User
	uid    sys.UserID
	gid    sys.GroupID
}

var _ = check.Suite(&localSdk{})

func (s *localSdk) SetUpTest(c *check.C) {
	s.target = c.MkDir()

	var err error
	s.user, err = user.Current()
	c.Assert(err, check.IsNil)

	s.uid, s.gid, err = osutil.UidGid(s.user)
	c.Assert(err, check.IsNil)
}

func (s *localSdk) createSource(c *check.C, contents string) string {
	source := filepath.Join(c.MkDir(), "source")
	c.Assert(os.Mkdir(source, 0755), check.IsNil)
	c.Assert(os.WriteFile(filepath.Join(source, "contents"), []byte(contents), 0644), check.IsNil)
	return source
}

func (s *localSdk) createRevision(c *check.C, revision, contents string) {
	c.Assert(os.Mkdir(filepath.Join(s.target, revision), 0755), check.IsNil)
	c.Assert(os.WriteFile(filepath.Join(s.target, revision, "contents"), []byte(contents), 0644), check.IsNil)
}

func (s *localSdk) TestCommitSuccess(c *check.C) {
	one := s.createSource(c, "1")
	revision, err := sdk.CommitRevision(s.user, one, s.target, sdk.Revision{})
	c.Assert(err, check.IsNil)
	c.Check(revision, check.Equals, sdk.R(-1))

	checkRev1 := func() {
		c.Check(filepath.Join(s.target, "x1"), testutil.DirEquals, []string{"-rw-r--r-- contents"})
		c.Check(filepath.Join(s.target, "x1", "contents"), testutil.FileEquals, "1")
	}
	c.Check(s.target, testutil.DirEquals, []string{"drwxr-xr-x x1"})
	checkRev1()

	two := s.createSource(c, "2")
	revision, err = sdk.CommitRevision(s.user, two, s.target, revision)
	c.Assert(err, check.IsNil)
	c.Check(revision, check.Equals, sdk.R(-2))

	checkRev2 := func() {
		c.Check(filepath.Join(s.target, "x2"), testutil.DirEquals, []string{"-rw-r--r-- contents"})
		c.Check(filepath.Join(s.target, "x2", "contents"), testutil.FileEquals, "2")
	}
	c.Check(s.target, testutil.DirEquals, []string{"drwxr-xr-x x1", "drwxr-xr-x x2"})
	checkRev1()
	checkRev2()

	oneAgain := s.createSource(c, "1")
	revision, err = sdk.CommitRevision(s.user, oneAgain, s.target, revision)
	c.Assert(err, check.IsNil)
	c.Check(revision, check.Equals, sdk.R(-1))

	c.Check(s.target, testutil.DirEquals, []string{"drwxr-xr-x x1", "drwxr-xr-x x2"})
	checkRev1()
	checkRev2()
}

func (s *localSdk) TestCommitIncreasesRevision(c *check.C) {
	s.createRevision(c, "x11", "11")
	s.createRevision(c, "x111", "111")
	s.createRevision(c, "x1111", "1111")

	source := s.createSource(c, "1112")
	revision, err := sdk.CommitRevision(s.user, source, s.target, sdk.R(-111))
	c.Assert(err, check.IsNil)
	c.Check(revision, check.Equals, sdk.R(-1112))
}

func (s *localSdk) TestCommitIgnoresUnusualRevisions(c *check.C) {
	s.createRevision(c, "10", "10")
	s.createRevision(c, "-11", "11")
	s.createRevision(c, "x+12", "12")
	s.createRevision(c, "x013", "13")

	source := s.createSource(c, "1")
	revision, err := sdk.CommitRevision(s.user, source, s.target, sdk.Revision{})
	c.Assert(err, check.IsNil)
	c.Check(revision, check.Equals, sdk.R(-1))
}

func (s *localSdk) TestCommitRemovesOldRevisions(c *check.C) {
	s.createRevision(c, "x42", "42")
	s.createRevision(c, "x43", "43")
	s.createRevision(c, "x44", "44")

	t3 := time.Now()
	t2 := t3.Add(-time.Minute)
	t1 := t2.Add(-time.Minute)
	c.Assert(os.Chtimes(filepath.Join(s.target, "x42"), time.Time{}, t2), check.IsNil)
	c.Assert(os.Chtimes(filepath.Join(s.target, "x43"), time.Time{}, t1), check.IsNil)
	c.Assert(os.Chtimes(filepath.Join(s.target, "x44"), time.Time{}, t3), check.IsNil)

	source := s.createSource(c, "45")
	revision, err := sdk.CommitRevision(s.user, source, s.target, sdk.R(-44))
	c.Assert(err, check.IsNil)
	c.Check(revision, check.Equals, sdk.R(-45))
	c.Check(s.target, testutil.DirEquals, []string{"drwxr-xr-x x42", "drwxr-xr-x x44", "drwxr-xr-x x45"})
}

func (s *localSdk) TestCommitKeepsInstalled(c *check.C) {
	s.createRevision(c, "x42", "42")
	s.createRevision(c, "x43", "43")
	s.createRevision(c, "x44", "44")

	t3 := time.Now()
	t2 := t3.Add(-time.Minute)
	t1 := t2.Add(-time.Minute)
	c.Assert(os.Chtimes(filepath.Join(s.target, "x42"), time.Time{}, t2), check.IsNil)
	c.Assert(os.Chtimes(filepath.Join(s.target, "x43"), time.Time{}, t1), check.IsNil)
	c.Assert(os.Chtimes(filepath.Join(s.target, "x44"), time.Time{}, t3), check.IsNil)

	source := s.createSource(c, "45")
	revision, err := sdk.CommitRevision(s.user, source, s.target, sdk.R(-43))
	c.Assert(err, check.IsNil)
	c.Check(revision, check.Equals, sdk.R(-45))
	c.Check(s.target, testutil.DirEquals, []string{"drwxr-xr-x x43", "drwxr-xr-x x44", "drwxr-xr-x x45"})
}

func (s *localSdk) TestCommitExistingUpdatesTimestamp(c *check.C) {
	s.createRevision(c, "x41", "41")
	s.createRevision(c, "x42", "42")
	s.createRevision(c, "x43", "43")
	s.createRevision(c, "x44", "44")

	old := time.Now().Add(-time.Minute)
	c.Assert(os.Chtimes(filepath.Join(s.target, "x43"), time.Time{}, old), check.IsNil)
	info, err := os.Stat(filepath.Join(s.target, "x43"))
	c.Assert(err, check.IsNil)
	c.Check(info.ModTime().Compare(old), check.Equals, 0)

	source := s.createSource(c, "43")
	revision, err := sdk.CommitRevision(s.user, source, s.target, sdk.R(-44))
	c.Assert(err, check.IsNil)
	c.Check(revision, check.Equals, sdk.R(-43))
	c.Check(s.target, testutil.DirEquals, []string{"drwxr-xr-x x41", "drwxr-xr-x x42", "drwxr-xr-x x43", "drwxr-xr-x x44"})

	info, err = os.Stat(filepath.Join(s.target, "x43"))
	c.Assert(err, check.IsNil)
	c.Check(info.ModTime().Compare(old), check.Equals, 1)
}
