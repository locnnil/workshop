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

package x11_test

import (
	"os"
	"os/user"
	"path/filepath"
	"syscall"
	"testing"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/x11"
)

func Test(t *testing.T) { check.TestingT(t) }

type X11TestSuit struct {
	restore func()
}

var _ = check.Suite(&X11TestSuit{})

func FakeWorkshopdRunDir(dir string) func() {
	old := dirs.WorkshopdRunDir
	dirs.WorkshopdRunDir = dir
	return func() {
		dirs.WorkshopdRunDir = old
	}
}

func (x *X11TestSuit) SetUpTest(c *check.C) {
	x.restore = FakeWorkshopdRunDir(c.MkDir())
}

func (x *X11TestSuit) TearDownTest(c *check.C) {
	x.restore()
}

func (x *X11TestSuit) TestMigrateXAuthoritySuccess(c *check.C) {
	user, err := user.Current()
	c.Assert(err, check.IsNil)

	xf, err := os.Create(filepath.Join(dirs.WorkshopdRunDir, ".workshop-Xauthority"))
	c.Assert(err, check.IsNil)
	defer xf.Close()

	err = x11.MigrateXauthority(user, filepath.Join(dirs.WorkshopdRunDir, ".workshop-Xauthority"))
	c.Assert(err, check.IsNil)

	c.Assert(filepath.Join(dirs.WorkshopdRunDir, user.Uid, "Xauthority", ".Xauthority"), testutil.FilePresent)
}

func (x *X11TestSuit) TestMigrateXAuthorityOwnershipFail(c *check.C) {
	user, err := user.Lookup("root")
	c.Assert(err, check.IsNil)

	xf, err := os.Create(filepath.Join(dirs.WorkshopdRunDir, ".workshop-Xauthority"))
	c.Assert(err, check.IsNil)
	defer xf.Close()

	err = x11.MigrateXauthority(user, filepath.Join(dirs.WorkshopdRunDir, ".workshop-Xauthority"))
	c.Assert(err, check.NotNil)
	c.Assert(err, testutil.ErrorIs, syscall.EACCES)
}
