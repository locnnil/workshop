package workshop_test

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	"github.com/canonical/workshop/internal/workshop/fakebackend"
)

type workshopFsSuite struct {
	fs workshop.WorkshopFs
}

var _ = check.Suite(&workshopFsSuite{})

func (f *workshopFsSuite) SetUpTest(c *check.C) {
	var err error
	f.fs, err = fakebackend.NewWorkshopFs(c.MkDir())
	c.Assert(err, check.IsNil)
}

func (f *workshopFsSuite) TestAtomicWrite(c *check.C) {
	c.Assert(workshop.AtomicWrite(f.fs, "/file", strings.NewReader("content"), 0400), check.IsNil)

	c.Check(f.fs, testutil.DirEquals, []string{"-r-------- file"})
	content, err := afero.ReadFile(f.fs, "/file")
	c.Assert(err, check.IsNil)
	c.Check(string(content), check.Equals, "content")

	c.Assert(workshop.AtomicWrite(f.fs, "/file", strings.NewReader("new"), 0500), check.IsNil)
	c.Check(f.fs, testutil.DirEquals, []string{"-r-x------ file"})
	content, err = afero.ReadFile(f.fs, "/file")
	c.Assert(err, check.IsNil)
	c.Check(string(content), check.Equals, "new")
}

func (f *workshopFsSuite) TestAtomicWriteNameOnly(c *check.C) {
	err := workshop.AtomicWrite(f.fs, "file", strings.NewReader("content"), 0644)
	c.Check(err, check.ErrorMatches, `parent directory not found for "file"`)
	c.Check(f.fs, testutil.DirEquals, []string{})
}

func (f *workshopFsSuite) TestAtomicWriteNoDirectory(c *check.C) {
	err := workshop.AtomicWrite(f.fs, "/var/tmp/file", strings.NewReader("content"), 0644)
	c.Check(err, testutil.ErrorIs, os.ErrNotExist)
	c.Check(f.fs, testutil.DirEquals, []string{})
}

func (f *workshopFsSuite) TestAtomicWriteSourceError(c *check.C) {
	expected := errors.New("fake error")
	err := workshop.AtomicWrite(f.fs, "/file", &ErrorSource{expected}, 0644)
	c.Check(err, testutil.ErrorIs, expected)
	c.Check(f.fs, testutil.DirEquals, []string{})
}

type ErrorSource struct {
	err error
}

func (s *ErrorSource) WriteTo(w io.Writer) (int64, error) {
	return 0, s.err
}

func (f *workshopFsSuite) TestAtomicWriteRenameFailed(c *check.C) {
	c.Assert(f.fs.Mkdir("/file", os.ModePerm), check.IsNil)
	err := workshop.AtomicWrite(f.fs, "/file", strings.NewReader("content"), 0644)
	c.Check(err, testutil.ErrorIs, os.ErrExist)

	c.Check(f.fs, testutil.DirEquals, []string{"drwxrwxr-x file"})
	dir := afero.NewBasePathFs(f.fs, "/file")
	c.Check(dir, testutil.DirEquals, []string{})
}
