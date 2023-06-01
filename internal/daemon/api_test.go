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

package daemon

import (
	"net/http"
	"os/user"
	"testing"

	"github.com/canonical/workspace/internal/testutil"
	"gopkg.in/check.v1"
)

var _ = check.Suite(&apiSuite{})

type apiSuite struct {
	d *Daemon

	workspaceDir string
	username     string

	vars map[string]string

	restoreMuxVars    func()
	restoreUserLookup func()
}

func TestApi(t *testing.T) { check.TestingT(t) }

func (s *apiSuite) SetUpTest(c *check.C) {
	s.restoreMuxVars = FakeMuxVars(s.muxVars)
	s.workspaceDir = c.MkDir()
	s.username = "testuser"

	s.restoreUserLookup = testutil.FakeFunc(func(uid string) (*user.User, error) {
		return &user.User{Username: s.username}, nil
	}, &LookupUsername)
}

func (s *apiSuite) TearDownTest(c *check.C) {
	s.d = nil
	s.workspaceDir = ""
	s.restoreMuxVars()
	s.restoreUserLookup()
}

func (s *apiSuite) muxVars(*http.Request) map[string]string {
	return s.vars
}

func (s *apiSuite) daemon(c *check.C) *Daemon {
	if s.d != nil {
		panic("called daemon() twice")
	}
	d, err := New(&Options{Dir: s.workspaceDir})
	c.Assert(err, check.IsNil)
	d.addRoutes()
	s.d = d
	return d
}

func apiCmd(path string) *Command {
	for _, cmd := range api {
		if cmd.Path == path {
			return cmd
		}
	}
	panic("no command with path " + path)
}
