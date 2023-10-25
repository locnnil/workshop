package hookstate_test

import (
	"fmt"

	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshopbackend"
	"gopkg.in/check.v1"
)

type S struct {
	state   *state.State
	project *workshopbackend.Project
}

var _ = check.Suite(&S{})

func (s *S) SetUpTest(c *check.C) {
	s.state = state.New(nil)
	s.project = &workshopbackend.Project{Path: "/home/testuser", ProjectId: "42ws42ws"}
}

func (s *S) TestCreateHook(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	var sdk = workshopbackend.SdkRecord{Name: "go", Channel: "latest/stable"}

	envs := []map[string]string{
		{},
		{"SDK_STATE_DIR": "/var/lib/workshop/state/sdk/go"},
		{"SDK_STATE_DIR": "/var/lib/workshop/state/sdk/go"},
	}

	for num, i := range []hookstate.WorkspaceHookType{
		hookstate.SetupBase,
		hookstate.SaveState,
		hookstate.RestoreState,
	} {
		task := hookstate.SetupHook(s.state, &sdk, i)

		var hookSetup hookstate.HookSetup
		err := task.Get("hook-setup", &hookSetup)
		c.Assert(err, check.IsNil)
		c.Assert(hookSetup.Type(), check.Equals, i.String())
		c.Assert(hookSetup.Sdk, check.DeepEquals, sdk)
		c.Assert(hookSetup.Environment, check.DeepEquals, envs[num])
		c.Check(task.Summary(), check.Equals, fmt.Sprintf("Run hook %q for \"go\" SDK", hookSetup.Type()))
	}
}
