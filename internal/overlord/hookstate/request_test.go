package hookstate_test

import (
	"fmt"
	"time"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshop"
)

type hooksRequestSuite struct {
	state   *state.State
	project *workshop.Project
}

var _ = check.Suite(&hooksRequestSuite{})

func (s *hooksRequestSuite) SetUpTest(c *check.C) {
	s.state = state.New(nil)
	s.project = &workshop.Project{ProjectId: "42424242", Path: c.MkDir()}
}

func (s *hooksRequestSuite) TestCreateHook(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	for _, hook := range []hookstate.WorkshopHookType{
		hookstate.SetupBase,
		hookstate.SetupProject,
		hookstate.SaveState,
		hookstate.RestoreState,
	} {
		task := hookstate.Hook(s.state, s.project.ProjectId, "test", "go", 0, hook)

		var hookSetup hookstate.HookSetup
		err := task.Get("hook-setup", &hookSetup)
		c.Assert(err, check.IsNil)
		c.Assert(hookSetup.Type(), check.Equals, hook.String())
		c.Assert(hookSetup.Workshop, check.DeepEquals, "test")
		c.Assert(hookSetup.Sdk, check.DeepEquals, "go")
		c.Check(task.Summary(), check.Equals, fmt.Sprintf("Run hook %q for \"go\" SDK", hookSetup.Type()))
	}
}

func (s *hooksRequestSuite) TestCreateHookWithTimeout(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	task := hookstate.Hook(s.state, s.project.ProjectId, "test", "go", 5*time.Second, hookstate.CheckHealth)
	var hookSetup hookstate.HookSetup
	err := task.Get("hook-setup", &hookSetup)
	c.Assert(err, check.IsNil)
	c.Assert(hookSetup.Type(), check.Equals, "check-health")
	c.Assert(hookSetup.Workshop, check.DeepEquals, "test")
	c.Assert(hookSetup.Sdk, check.DeepEquals, "go")
	c.Assert(hookSetup.Timeout, check.Equals, 5*time.Second)
	c.Check(task.Summary(), check.Equals, fmt.Sprintf("Run hook %q for \"go\" SDK", hookSetup.Type()))
}
