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
