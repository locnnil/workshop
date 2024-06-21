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
	"context"
	"net/http"
	"os/user"
	"testing"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
)

var _ = check.Suite(&apiSuite{})

type apiSuite struct {
	d *Daemon
	b workshop.Backend

	workshopDir string
	username    string
	project     *workshop.Project
	ctx         context.Context

	vars map[string]string

	restoreMuxVars    func()
	restoreUserLookup func()
	restoreProjectId  func()
}

func TestApi(t *testing.T) { check.TestingT(t) }

func (s *apiSuite) SetUpTest(c *check.C) {
	s.restoreMuxVars = FakeMuxVars(s.muxVars)
	s.workshopDir = c.MkDir()
	s.username = "testuser"
	s.project = &workshop.Project{
		Path:      s.workshopDir,
		ProjectId: "b8639dea",
	}
	s.b = workshop.NewFakeWorkshopBackend()

	s.restoreUserLookup = testutil.FakeFunc(func(uid string) (*user.User, error) {
		return &user.User{Username: s.username}, nil
	}, &workshop.LookupUsername)

	// will be called when project is created
	s.restoreProjectId = testutil.FakeFunc(func() (string, error) { return s.project.ProjectId, nil }, &workshop.NewProjectId)

	ctx := context.WithValue(context.TODO(), workshop.ContextProjectId, s.project.ProjectId)
	s.ctx = context.WithValue(ctx, workshop.ContextUser, "testuser")

	_, _, err := s.b.CreateOrLoadProject(s.ctx, s.project.Path)
	c.Assert(err, check.IsNil)
}

func (s *apiSuite) TearDownTest(c *check.C) {
	s.d = nil
	s.workshopDir = ""
	s.restoreMuxVars()
	s.restoreUserLookup()
	s.restoreProjectId()
}

func (s *apiSuite) muxVars(*http.Request) map[string]string {
	return s.vars
}

func (s *apiSuite) daemon(c *check.C) *Daemon {
	if s.d != nil {
		panic("called daemon() twice")
	}
	d, err := New(&Options{Dir: s.workshopDir}, s.b)
	c.Assert(err, check.IsNil)
	c.Assert(d.overlord.StartUp(), check.IsNil)
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
