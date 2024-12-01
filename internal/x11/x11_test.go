package x11_test

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/x11"
)

var userCurrent = user.Current
var userLookup = user.Lookup

func Test(t *testing.T) { TestingT(t) }

type X11TestSuit struct{}

var _ = Suite(&X11TestSuit{})

func (x *X11TestSuit) SetUpTest(c *C) {
}

func (x *X11TestSuit) TearDownTest(c *C) {
}

func restoreWorkshopdRunDir(runDir string) {
	dirs.WorkshopdRunDir = runDir
}

func (x *X11TestSuit) TestCopyXAuthority(c *C) {
	user, err := userCurrent()
	c.Assert(err, IsNil)

	defer restoreWorkshopdRunDir(dirs.WorkshopdRunDir)
	dirs.WorkshopdRunDir = c.MkDir()

	xf, err := os.Create(filepath.Join(dirs.WorkshopdRunDir, ".workshop-Xauthority"))
	defer xf.Close()
	c.Assert(err, IsNil)

	err = x11.MigrateXauthority(user, filepath.Join(dirs.WorkshopdRunDir, ".workshop-Xauthority"))
	c.Assert(err, IsNil)
	_, err = os.Stat(filepath.Join(dirs.WorkshopdRunDir, user.Uid, ".Xauthority"))
	c.Assert(err, IsNil)
}

func (x *X11TestSuit) TestCopyXAuthorityOwnershipFail(c *C) {
	user, err := userLookup("root")
	c.Assert(err, IsNil)

	defer restoreWorkshopdRunDir(dirs.WorkshopdRunDir)
	dirs.WorkshopdRunDir = c.MkDir()

	xf, err := os.Create(filepath.Join(dirs.WorkshopdRunDir, ".workshop-Xauthority"))
	defer xf.Close()
	c.Assert(err, IsNil)

	err = x11.MigrateXauthority(user, filepath.Join(dirs.WorkshopdRunDir, ".workshop-Xauthority"))
	c.Assert(err, NotNil)
	c.Assert(err.Error(), testutil.Contains, "Xauthority file isn't owned")
}
