// Copyright (c) 2014-2020 Canonical Ltd
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

package osutil_test

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/osutil/sys"
	"github.com/canonical/workshop/internal/testutil"
)

type userSuite struct {
	testutil.BaseTest
}

func TestOsUtil(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&userSuite{})

func (s *userSuite) SetUpTest(c *check.C) {
}

func (s *userSuite) TearDownTest(c *check.C) {
}

func (s *userSuite) TestRealUser(c *check.C) {
	oldUser := os.Getenv("SUDO_USER")
	defer func() { os.Setenv("SUDO_USER", oldUser) }()

	for _, t := range []struct {
		SudoUsername    string
		CurrentUsername string
		CurrentUid      int
	}{
		// simulate regular "root", no SUDO_USER set
		{"", os.Getenv("USER"), 0},
		// simulate a normal sudo invocation
		{"guy", "guy", 0},
		// simulate running "sudo -u some-user -i" as root
		// (LP: #1638656)
		{"root", os.Getenv("USER"), 1000},
	} {
		restore := osutil.FakeUserCurrent(func() (*user.User, error) {
			return &user.User{
				Username: t.CurrentUsername,
				Uid:      strconv.Itoa(t.CurrentUid),
			}, nil
		})
		defer restore()

		os.Setenv("SUDO_USER", t.SudoUsername)
		cur, err := osutil.RealUser()
		c.Assert(err, check.IsNil)
		c.Check(cur.Username, check.Equals, t.CurrentUsername)
	}
}

func (s *userSuite) TestUidGid(c *check.C) {
	for k, t := range map[string]struct {
		User *user.User
		Uid  sys.UserID
		Gid  sys.GroupID
		Err  string
	}{
		"happy":   {&user.User{Uid: "10", Gid: "10"}, 10, 10, ""},
		"bad uid": {&user.User{Uid: "x", Gid: "10"}, sys.FlagID, sys.FlagID, `cannot parse user id "x"`},
		"bad gid": {&user.User{Uid: "10", Gid: "x"}, sys.FlagID, sys.FlagID, `cannot parse group id "x"`},
	} {
		uid, gid, err := osutil.UidGid(t.User)
		c.Check(uid, check.Equals, t.Uid, check.Commentf(k))
		c.Check(gid, check.Equals, t.Gid, check.Commentf(k))
		if t.Err == "" {
			c.Check(err, check.IsNil, check.Commentf(k))
		} else {
			c.Check(err, check.ErrorMatches, ".*"+t.Err+".*", check.Commentf(k))
		}
	}
}

func (s *userSuite) TestNormalizeUidGid(c *check.C) {
	test := func(uid, gid *int, username, group string, expectedUid, expectedGid *int, errMatch string) {
		uid, gid, err := osutil.NormalizeUidGid(uid, gid, username, group)
		if err != nil {
			c.Check(err, check.ErrorMatches, errMatch)
		} else {
			c.Check(errMatch, check.Equals, "")
		}
		c.Check(uid, check.DeepEquals, expectedUid)
		c.Check(gid, check.DeepEquals, expectedGid)
	}
	ptr := func(n int) *int {
		return &n
	}

	var userErr error
	restoreUser := osutil.FakeUserLookup(func(name string) (*user.User, error) {
		c.Check(name, check.Equals, "USER")
		return &user.User{Uid: "10", Gid: "20"}, userErr
	})
	defer restoreUser()

	var groupErr error
	restoreGroup := osutil.FakeUserLookupGroup(func(name string) (*user.Group, error) {
		c.Check(name, check.Equals, "GROUP")
		return &user.Group{Gid: "30"}, groupErr
	})
	defer restoreGroup()

	test(nil, nil, "", "", nil, nil, "")
	test(nil, nil, "", "GROUP", nil, nil, "must specify user, not just group")
	test(nil, nil, "USER", "", ptr(10), ptr(20), "")
	test(nil, nil, "USER", "GROUP", ptr(10), ptr(30), "")

	test(nil, ptr(2), "", "", nil, nil, "must specify user, not just group")
	test(nil, ptr(2), "", "GROUP", nil, nil, `group "GROUP" GID \(30\) does not match group-id \(2\)`)
	test(nil, ptr(2), "USER", "", ptr(10), ptr(2), "")
	test(nil, ptr(2), "USER", "GROUP", nil, nil, `group "GROUP" GID \(30\) does not match group-id \(2\)`)

	test(ptr(1), nil, "", "", nil, nil, "must specify group, not just UID")
	test(ptr(1), nil, "", "GROUP", ptr(1), ptr(30), "")
	test(ptr(1), nil, "USER", "", nil, nil, `user "USER" UID \(10\) does not match user-id \(1\)`)
	test(ptr(1), nil, "USER", "GROUP", nil, nil, `user "USER" UID \(10\) does not match user-id \(1\)`)

	test(ptr(1), ptr(2), "", "", ptr(1), ptr(2), "")
	test(ptr(1), ptr(2), "", "GROUP", nil, nil, `group "GROUP" GID \(30\) does not match group-id \(2\)`)
	test(ptr(1), ptr(2), "USER", "", nil, nil, `user "USER" UID \(10\) does not match user-id \(1\)`)
	test(ptr(1), ptr(2), "USER", "GROUP", nil, nil, `user "USER" UID \(10\) does not match user-id \(1\)`)

	userErr = fmt.Errorf("USER ERROR!")
	test(nil, nil, "USER", "", nil, nil, "USER ERROR!")
	groupErr = fmt.Errorf("GROUP ERROR!")
	test(ptr(1), nil, "", "GROUP", nil, nil, "GROUP ERROR!")
}

func (s *userSuite) TestUserAndEnv(c *check.C) {
	// Ensure we're testing this function in an environment using systemd as the
	// init system:
	// https://superuser.com/questions/1017959/how-to-know-if-i-am-using-systemd-on-linux
	cmd := exec.Command("ps", "--no-headers", "-o", "comm", "1")
	out, _, err := osutil.RunCmd(cmd)
	c.Assert(err, check.IsNil)
	if !strings.Contains(string(out), "systemd") {
		c.Skip("This test requires systemd")
	}

	envIn := map[string]string{
		"WORKSHOP_TEST_ENV_KEY":   "VALUE",
		"WORKSHOP_TEST_ENV_EMPTY": "",
	}

	for k, v := range envIn {
		// Set environment
		cmd := exec.Command("systemctl", "--user", "set-environment", k+"="+v)
		c.Check(cmd.Run(), check.IsNil)
	}

	cur, err := osutil.UserCurrent()
	c.Check(err, check.IsNil)

	// Check variables
	usr, envOut, err := osutil.UserAndEnv(cur.Username)
	c.Check(err, check.IsNil)
	c.Check(usr, check.DeepEquals, cur)
	for k, v := range envIn {
		c.Check(envOut[k], check.Equals, v)

		cmd := exec.Command("systemctl", "--user", "unset-environment", k)
		c.Check(cmd.Run(), check.IsNil)
	}
}
