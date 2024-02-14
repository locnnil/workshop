/*
 * Copyright (C) 2019 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package healthstate_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/overlord"
	"github.com/canonical/workshop/internal/overlord/healthstate"
	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
)

func TestHealthState(t *testing.T) { check.TestingT(t) }

type healthSuite struct {
	testutil.BaseTest
	se      *overlord.StateEngine
	state   *state.State
	runner  *state.TaskRunner
	backend *workshopbackend.FakeWorkshopBackend
	hookMgr *hookstate.HookManager
	project *workshopbackend.Project
	ctx     context.Context
}

var _ = check.Suite(&healthSuite{})

func (s *healthSuite) SetUpTest(c *check.C) {
	s.BaseTest.SetUpTest(c)
	dirs.SetRootDir(c.MkDir())

	s.state = state.New(nil)
	s.runner = state.NewTaskRunner(s.state)

	s.backend = workshopbackend.NewFakeWorkshopBackend()
	ctx := context.WithValue(context.Background(), workshopbackend.ContextUser, "testuser")
	var err error
	s.project, _, err = s.backend.CreateOrLoadProject(ctx, c.MkDir())
	c.Assert(err, check.IsNil)
	s.ctx = context.WithValue(ctx, workshopbackend.ContextProjectId, s.project.ProjectId)

	s.hookMgr = hookstate.New(s.state, s.runner, s.backend)

	s.se = overlord.NewStateEngine(s.state)
	s.se.AddManager(s.hookMgr)
	s.se.AddManager(s.runner)

	s.se.AddManager(s.hookMgr)
	s.se.AddManager(s.runner)

	healthstate.Init(s.hookMgr)

	c.Assert(s.se.StartUp(), check.IsNil)
}

func (s *healthSuite) TearDownTest(c *check.C) {
	s.se.Stop()
	s.BaseTest.TearDownTest(c)
}

func setWorkshopProject(w string, p *workshopbackend.Project, tasks ...*state.Task) {
	for _, i := range tasks {
		i.Set("workshop", w)
		i.Set("project", *p)
	}
}

func (s *healthSuite) launchWorkshop(c *check.C, newsdk string, createHealthCheck bool) {
	err := os.WriteFile(filepath.Join(s.project.Path, ".workshop.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04
sdks:
  one:
    channel: latest/stable
`), 0644)
	c.Check(err, check.IsNil)
	err = s.backend.LaunchWorkshop(s.ctx, "ws", "ubuntu@20.04")
	c.Check(err, check.IsNil)
	ws, err := s.backend.WorkshopFs(s.ctx, "ws")
	c.Check(err, check.IsNil)
	err = ws.MkdirAll(sdk.SdkHooksDir(newsdk), 0744)
	c.Check(err, check.IsNil)

	if createHealthCheck {
		_, err = ws.Create(sdk.SdkHookPath(newsdk, hookstate.CheckHealth.String()))
		c.Check(err, check.IsNil)
	}
}

func (*healthSuite) TestStatusHappy(c *check.C) {
	for i, str := range healthstate.KnownStatuses {
		status, err := healthstate.SetHealthLookup(str)
		c.Check(err, check.IsNil, check.Commentf("%v", str))
		c.Check(status, check.Equals, healthstate.HealthCheckResult(i), check.Commentf("%v", str))
	}
}

func (*healthSuite) TestStatusUnhappy(c *check.C) {
	status, err := healthstate.SetHealthLookup("rabbits")
	c.Check(status, check.Equals, healthstate.HealthCheckResult(-1))
	c.Check(err, check.ErrorMatches, `invalid status "rabbits".*`)
	c.Check(status.String(), check.Equals, "invalid (-1)")
}

func (s *healthSuite) TestExecCheckHealthNotProvided(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	t1 := hookstate.Hook(s.state, "ws", "one", hookstate.CheckHealth)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)

	s.launchWorkshop(c, "one", false)

	s.state.Unlock()
	s.se.Ensure()
	s.se.Wait()
	s.state.Lock()

	c.Assert(s.backend.ExecCalls, check.HasLen, 0)
	c.Assert(t1.Status(), check.Equals, state.DoneStatus)
}

func (s *healthSuite) TestExecCheckHealthSetHealthNotCalled(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	t1 := hookstate.Hook(s.state, "ws", "one", hookstate.CheckHealth)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)

	s.launchWorkshop(c, "one", true)
	restore := healthstate.FakeRetryTimeout(0 * time.Second)
	defer restore()

	s.state.Unlock()
	for i := 0; i < 11; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	c.Assert(s.backend.ExecCalls, check.HasLen, 11)
	c.Assert(t1.Status(), check.Equals, state.ErrorStatus)
	c.Assert(t1.Log()[0], check.Matches, `.*SDK "one" is not healthy after multiple checks`)
}
