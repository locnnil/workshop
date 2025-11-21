//go:build integration

package osutil_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/osutil"
)

type integrationSuite struct{}

var _ = check.Suite(&integrationSuite{})

func (s *integrationSuite) TestUserAndEnv(c *check.C) {
	user, env, err := osutil.UserAndEnv("ubuntu")
	c.Assert(err, check.IsNil)

	c.Check(env["USER"], check.Equals, user.Username)
	c.Check(env["HOME"], check.Equals, user.HomeDir)
	c.Check(filepath.Join(dirs.XdgRuntimeDirBase, user.Uid), check.Equals, env["XDG_RUNTIME_DIR"])

	cmd := exec.Command(
		"systemctl",
		"--user",
		"set-environment",
		fmt.Sprintf("--machine=%s@.host", user.Uid),
		"FAKE_VARIABLE_FOR_TESTING=fakeValueForTest",
	)
	cmd.Env = append(cmd.Env, "XDG_RUNTIME_DIR="+env["XDG_RUNTIME_DIR"])
	c.Assert(cmd.Run(), check.IsNil)

	_, env, err = osutil.UserAndEnv("ubuntu")
	c.Assert(err, check.IsNil)

	c.Check(env["FAKE_VARIABLE_FOR_TESTING"], check.Equals, "fakeValueForTest")
}

func (s *integrationSuite) TestTimezone(c *check.C) {
	timezone, err := osutil.Timezone()
	c.Assert(err, check.IsNil)

	localtime, err := os.Stat("/etc/localtime")
	c.Assert(err, check.IsNil)

	zoneinfo, err := os.Stat(filepath.Join("/usr/share/zoneinfo", timezone))
	c.Assert(err, check.IsNil)

	c.Check(os.SameFile(localtime, zoneinfo), check.Equals, true)
}
