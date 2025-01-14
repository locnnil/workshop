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
	c.Assert(workshop.AtomicWrite(f.fs, "/file", strings.NewReader("content"), 0200), check.IsNil)

	info, err := f.fs.Stat("/file")
	c.Assert(err, check.IsNil)
	c.Check(info.Name(), check.Equals, "file")
	c.Check(info.Size(), check.Equals, int64(7))
	c.Check(info.Mode().Perm(), check.Equals, os.FileMode(0200))
	c.Check(info.IsDir(), check.Equals, false)

	infos, err := afero.ReadDir(f.fs, "/")
	c.Assert(err, check.IsNil)
	c.Assert(infos, check.HasLen, 1)
	c.Check(infos[0].Name(), check.Equals, "file")

	c.Assert(workshop.AtomicWrite(f.fs, "/file", strings.NewReader("new"), 0100), check.IsNil)

	info, err = f.fs.Stat("/file")
	c.Assert(err, check.IsNil)
	c.Check(info.Name(), check.Equals, "file")
	c.Check(info.Size(), check.Equals, int64(3))
	c.Check(info.Mode().Perm(), check.Equals, os.FileMode(0100))
	c.Check(info.IsDir(), check.Equals, false)

	infos, err = afero.ReadDir(f.fs, "/")
	c.Assert(err, check.IsNil)
	c.Assert(infos, check.HasLen, 1)
	c.Check(infos[0].Name(), check.Equals, "file")
}

func (f *workshopFsSuite) TestAtomicWriteNameOnly(c *check.C) {
	err := workshop.AtomicWrite(f.fs, "file", strings.NewReader("content"), 0644)
	c.Check(err, check.ErrorMatches, `parent directory not found for "file"`)
}

func (f *workshopFsSuite) TestAtomicWriteNoDirectory(c *check.C) {
	err := workshop.AtomicWrite(f.fs, "/var/tmp/file", strings.NewReader("content"), 0644)
	c.Check(err, testutil.ErrorIs, os.ErrNotExist)
}

func (f *workshopFsSuite) TestAtomicWriteSourceError(c *check.C) {
	expected := errors.New("fake error")
	err := workshop.AtomicWrite(f.fs, "/file", &ErrorSource{expected}, 0644)
	c.Check(err, testutil.ErrorIs, expected)

	infos, err := afero.ReadDir(f.fs, "/")
	c.Assert(err, check.IsNil)
	c.Assert(infos, check.HasLen, 0)
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

	infos, err := afero.ReadDir(f.fs, "/")
	c.Assert(err, check.IsNil)
	c.Assert(infos, check.HasLen, 1)
	c.Check(infos[0].Name(), check.Equals, "file")

	infos, err = afero.ReadDir(f.fs, "/file")
	c.Assert(err, check.IsNil)
	c.Assert(infos, check.HasLen, 0)
}
