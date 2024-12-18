package hookstate_test

import (
	"fmt"
	"time"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/state"
)

type S struct {
	state *state.State
}

var _ = check.Suite(&S{})

func (s *S) SetUpTest(c *check.C) {
	s.state = state.New(nil)
}

func (s *S) TestCreateHook(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	envs := []map[string]string{
		{},
		{"SDK_STATE_DIR": "/var/lib/workshop/state/sdk/go"},
		{"SDK_STATE_DIR": "/var/lib/workshop/state/sdk/go"},
	}

	for num, i := range []hookstate.WorkshopHookType{
		hookstate.SetupBase,
		hookstate.SaveState,
		hookstate.RestoreState,
	} {
		task := hookstate.Hook(s.state, "test", "go", i)

		var hookSetup hookstate.HookSetup
		err := task.Get("hook-setup", &hookSetup)
		c.Assert(err, check.IsNil)
		c.Assert(hookSetup.Type(), check.Equals, i.String())
		c.Assert(hookSetup.Workshop, check.DeepEquals, "test")
		c.Assert(hookSetup.Sdk, check.DeepEquals, "go")
		c.Assert(hookSetup.Environment, check.DeepEquals, envs[num])
		c.Check(task.Summary(), check.Equals, fmt.Sprintf("Run hook %q for \"go\" SDK", hookSetup.Type()))
	}
}

func (s *S) TestCreateHookWithTimeout(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	task := hookstate.HookWithTimeout(s.state, "test", "go", hookstate.CheckHealth, 5*time.Second)
	var hookSetup hookstate.HookSetup
	err := task.Get("hook-setup", &hookSetup)
	c.Assert(err, check.IsNil)
	c.Assert(hookSetup.Type(), check.Equals, "check-health")
	c.Assert(hookSetup.Workshop, check.DeepEquals, "test")
	c.Assert(hookSetup.Sdk, check.DeepEquals, "go")
	c.Assert(hookSetup.Environment, check.HasLen, 0)
	c.Assert(hookSetup.Timeout, check.Equals, 5*time.Second)
	c.Check(task.Summary(), check.Equals, fmt.Sprintf("Run hook %q for \"go\" SDK", hookSetup.Type()))
}
