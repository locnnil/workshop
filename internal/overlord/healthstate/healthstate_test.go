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
	"github.com/canonical/workshop/internal/workshop"
	"github.com/canonical/workshop/internal/workshop/fakebackend"
)

func TestHealthState(t *testing.T) { check.TestingT(t) }

type healthSuite struct {
	testutil.BaseTest
	se      *overlord.StateEngine
	state   *state.State
	runner  *state.TaskRunner
	backend *fakebackend.FakeWorkshopBackend
	hookMgr *hookstate.HookManager
	project workshop.Project
	ctx     context.Context
}

var _ = check.Suite(&healthSuite{})

func (s *healthSuite) SetUpTest(c *check.C) {
	s.BaseTest.SetUpTest(c)
	var err error
	dirs.SetRootDir(c.MkDir())

	s.state = state.New(nil)
	s.runner = state.NewTaskRunner(s.state)

	s.backend, err = fakebackend.New(c.MkDir())
	c.Assert(err, check.IsNil)
	workshop.ReplaceBackend(s.state, s.backend)

	ctx := context.WithValue(context.Background(), workshop.ContextUser, "testuser")
	project, _, err := s.backend.CreateOrLoadProject(ctx, c.MkDir())
	c.Assert(err, check.IsNil)
	s.project = *project
	s.ctx = context.WithValue(ctx, workshop.ContextProjectId, s.project.ProjectId)

	s.hookMgr = hookstate.New(s.state, s.runner)

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

func setWorkshopProject(w string, p workshop.Project, tasks ...*state.Task) {
	for _, task := range tasks {
		task.Set("workshop", w)
		task.Set("project", p)
	}
}

func ensureTaskHealthIsSet(t *state.Task, expected *healthstate.HealthCheck, c *check.C) {
	var health healthstate.HealthCheck
	err := t.Get("health", &health)
	c.Assert(err, check.IsNil)
	c.Assert(expected, check.DeepEquals, &health)
}

func (s *healthSuite) launchWorkshop(c *check.C, newsdk string, createHealthCheck bool) {
	wf := &workshop.File{Name: "ws", Base: "ubuntu@20.04", Sdks: []workshop.SdkRecord{{Name: "one", Channel: "latest/stable"}}}
	err := s.backend.LaunchWorkshop(s.ctx, wf)
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
	s.se.Ensure()
	s.se.Wait()
	s.state.Lock()

	c.Assert(s.backend.ExecCalls, check.HasLen, 1)
	c.Assert(t1.Status(), check.Equals, state.ErrorStatus)
	c.Assert(t1.Log()[0], check.Matches, `.*SDK "one" health status is unknown`)
}

func (s *healthSuite) TestExecCheckHealthSetHealthError(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	t1 := hookstate.Hook(s.state, "ws", "one", hookstate.CheckHealth)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)

	now := time.Now().UTC()
	result := healthstate.HealthCheck{
		Timestamp:   now,
		Sdk:         "one",
		CheckResult: healthstate.CheckError,
		Message:     "something went wrong",
		Code:        "error-error",
	}

	var hookContext *hookstate.Context
	s.backend.ExecCallback = func(ctx context.Context, name string, args *workshop.Execution) (workshop.ExecContext, error) {
		return workshop.ExecContext{
			WaitExecution: func(ctx context.Context) error {
				// emulate workshopctl set-health --code=<code> error <message>
				var err error
				hookContext, err = s.hookMgr.Context(args.ExecArgs.Environment["WORKSHOP_COOKIE"])
				c.Assert(err, check.IsNil)
				hookContext.Lock()
				hookContext.Set("health", result)
				hookContext.Unlock()
				return nil
			},
		}, nil
	}
	defer func() {
		s.backend.ExecCallback = fakebackend.DoExecDefault
	}()

	s.launchWorkshop(c, "one", true)
	s.state.Unlock()
	s.se.Ensure()
	s.se.Wait()
	s.state.Lock()

	c.Assert(t1.Status(), check.Equals, state.ErrorStatus)
	c.Assert(t1.Log()[0], check.Matches, `.*something went wrong`)
	s.state.Unlock()
	hookContext.Lock()
	var counter int
	err := hookContext.Get("retry-counter", &counter)
	hookContext.Unlock()
	s.state.Lock()
	c.Assert(err, check.IsNil)
	c.Assert(counter, check.Equals, 0)

	ensureTaskHealthIsSet(t1, &result, c)
}

func (s *healthSuite) TestExecCheckHealthSetHealthWaiting(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	t1 := hookstate.Hook(s.state, "ws", "one", hookstate.CheckHealth)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)

	restore := healthstate.FakeRetryTimeout(0 * time.Second)
	defer restore()

	now := time.Now().UTC()
	resultWait := healthstate.HealthCheck{
		Timestamp:   now,
		Sdk:         "one",
		CheckResult: healthstate.CheckWaiting,
		Message:     "not ready yet",
		Code:        "wait-for-me",
	}

	nowOkay := time.Now().UTC()
	resultOkay := healthstate.HealthCheck{
		Timestamp:   nowOkay,
		Sdk:         "one",
		CheckResult: healthstate.CheckOkay,
	}

	s.backend.ExecCallback = func(ctx context.Context, name string, args *workshop.Execution) (workshop.ExecContext, error) {
		return workshop.ExecContext{
			WaitExecution: func(ctx context.Context) error {
				hookCtx, err := s.hookMgr.Context(args.ExecArgs.Environment["WORKSHOP_COOKIE"])
				c.Assert(err, check.IsNil)
				hookCtx.Lock()
				var counter int
				hookCtx.Get("retry-counter", &counter)
				if counter == 0 {
					// emulate workshopctl set-health --code=<code> error <message>
					hookCtx.Set("health", resultWait)
				} else {
					// emulate workshopctl set-health okay
					hookCtx.Set("health", resultOkay)
				}
				hookCtx.Unlock()
				return nil
			},
		}, nil
	}
	defer func() {
		s.backend.ExecCallback = fakebackend.DoExecDefault
	}()

	s.launchWorkshop(c, "one", true)
	s.state.Unlock()
	for i := 0; i < 2; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	c.Assert(t1.Status(), check.Equals, state.DoneStatus)
	c.Assert(t1.Log(), check.HasLen, 0)
	ensureTaskHealthIsSet(t1, &resultOkay, c)
}

func (s *healthSuite) TestExecCheckHealthSetHealthExceededAttempts(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	t1 := hookstate.Hook(s.state, "ws", "one", hookstate.CheckHealth)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)

	restore := healthstate.FakeRetryTimeout(0 * time.Second)
	defer restore()

	restoreAttempts := healthstate.FakeRetryAttempts(2)
	defer restoreAttempts()

	now := time.Now().UTC()
	resultWait := healthstate.HealthCheck{
		Timestamp:   now,
		Sdk:         "one",
		CheckResult: healthstate.CheckWaiting,
		Message:     "not ready yet",
		Code:        "wait-for-me",
	}

	var hookContext *hookstate.Context
	s.backend.ExecCallback = func(ctx context.Context, name string, args *workshop.Execution) (workshop.ExecContext, error) {
		return workshop.ExecContext{
			WaitExecution: func(ctx context.Context) error {
				var err error
				hookContext, err = s.hookMgr.Context(args.ExecArgs.Environment["WORKSHOP_COOKIE"])
				c.Assert(err, check.IsNil)
				hookContext.Lock()
				// emulate workshopctl set-health --code=<code> error <message>
				hookContext.Set("health", resultWait)
				hookContext.Unlock()
				return nil
			},
		}, nil
	}
	defer func() {
		s.backend.ExecCallback = fakebackend.DoExecDefault
	}()

	s.launchWorkshop(c, "one", true)
	s.state.Unlock()
	for i := 0; i < 3; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	c.Assert(t1.Status(), check.Equals, state.ErrorStatus)
	c.Assert(t1.Log()[0], check.Matches, `.*SDK \"one\" is not healthy after multiple checks`)
	c.Assert(t1.Log(), check.HasLen, 1)

	s.state.Unlock()
	hookContext.Lock()
	var counter int
	err := hookContext.Get("retry-counter", &counter)
	hookContext.Unlock()
	s.state.Lock()
	c.Assert(err, check.IsNil)
	c.Assert(counter, check.Equals, 0)
}

func (s *healthSuite) TestExecCheckHealthTimeout(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	t1 := hookstate.Hook(s.state, "ws", "one", hookstate.CheckHealth)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)

	s.backend.ExecCallback = func(ctx context.Context, name string, args *workshop.Execution) (workshop.ExecContext, error) {
		return workshop.ExecContext{
			WaitExecution: func(ctx context.Context) error {
				return context.DeadlineExceeded
			},
		}, nil
	}
	defer func() {
		s.backend.ExecCallback = fakebackend.DoExecDefault
	}()

	s.launchWorkshop(c, "one", true)
	s.state.Unlock()
	s.se.Ensure()
	s.se.Wait()
	s.state.Lock()

	c.Assert(t1.Status(), check.Equals, state.ErrorStatus)
	c.Assert(t1.Log()[0], check.Matches, `.*SDK "one" health status check timed out`)
}
