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
	"time"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/ifacetest"
	"github.com/canonical/workshop/internal/overlord"
	"github.com/canonical/workshop/internal/overlord/ifacestate"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	"github.com/canonical/workshop/internal/workshop/fakebackend"
)

var _ = check.Suite(&apiSuite{})

type apiSuite struct {
	d          *Daemon
	b          *fakebackend.FakeWorkshopBackend
	secBackend *ifacetest.TestSecurityBackend
	store      *sdk.FakeStore

	workshopDir string
	username    string
	userhome    string
	installTime time.Time
	project     *workshop.Project
	ctx         context.Context

	vars map[string]string

	restoreMuxVars    func()
	restoreProjectId  func()
	restoreUser       func()
	restoreTime       func()
	restoreSanitize   func()
	restoreSecBackend func()
}

func TestApi(t *testing.T) { check.TestingT(t) }

func (s *apiSuite) SetUpTest(c *check.C) {
	s.restoreMuxVars = FakeMuxVars(s.muxVars)
	s.workshopDir = c.MkDir()

	s.username = "testuser"
	s.userhome = c.MkDir()
	cur, err := user.Current()
	c.Assert(err, check.IsNil)
	s.restoreUser = workshop.FakeUserLookup(func(name string) (*user.User, error) {
		c.Check(name, check.Equals, s.username)
		return &user.User{Name: s.username, HomeDir: s.userhome, Uid: cur.Uid, Gid: cur.Gid}, nil
	})

	s.project = &workshop.Project{
		Path:      s.workshopDir,
		ProjectId: "b8639dea",
	}

	s.store = &sdk.FakeStore{}

	s.b, err = fakebackend.New(c.MkDir())
	c.Check(err, check.IsNil)

	s.installTime = time.Date(2023, 04, 25, 1, 2, 3, 0, time.UTC)
	s.restoreTime = testutil.FakeFunc(func() time.Time { return s.installTime }, &workshop.InstallTimeNow)

	// will be called when project is created
	s.restoreProjectId = testutil.FakeFunc(func() (string, error) { return s.project.ProjectId, nil }, &workshop.NewProjectId)

	ctx := context.WithValue(context.TODO(), workshop.ContextProjectId, s.project.ProjectId)
	s.ctx = context.WithValue(ctx, workshop.ContextUser, s.username)

	_, _, err = s.b.CreateOrLoadProject(s.ctx, s.project.Path)
	c.Assert(err, check.IsNil)

	s.restoreSanitize = sdk.MockSanitizePlugsSlots(func(sdkInfo *sdk.Info) {})
	s.secBackend = &ifacetest.TestSecurityBackend{BackendName: "api-suite"}
	s.restoreSecBackend = ifacestate.MockSecurityBackends([]interfaces.SecurityBackend{s.secBackend})
}

func (s *apiSuite) TearDownTest(c *check.C) {
	s.d = nil
	s.workshopDir = ""
	s.restoreMuxVars()
	s.restoreProjectId()
	s.restoreUser()
	s.restoreTime()
	s.restoreSanitize()
	s.restoreSecBackend()
}

func (s *apiSuite) muxVars(*http.Request) map[string]string {
	return s.vars
}

func (s *apiSuite) daemon(c *check.C) *Daemon {
	if s.d != nil {
		panic("called daemon() twice")
	}
	dirs.SetRootDir(c.MkDir())
	c.Assert(dirs.CreateDirs(), check.IsNil)
	undo := overlord.MockWorkshopBackend(s.b)
	defer undo()

	d, err := New(&Options{Dir: s.workshopDir})
	c.Assert(err, check.IsNil)

	c.Assert(d.overlord.StartUp(), check.IsNil)
	d.addRoutes()
	s.d = d

	sdk.ReplaceStore(s.d.state, s.store)
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
