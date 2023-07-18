package hookstate_test

import (
	"fmt"
	"testing"

	"github.com/canonical/workspace/internal/overlord/hookstate"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
	"gopkg.in/check.v1"
)

type S struct {
	state   *state.State
	project *workspacebackend.Project
}

var _ = check.Suite(&S{})

func TestHookstateRequest(t *testing.T) { check.TestingT(t) }

func (s *S) SetUpTest(c *check.C) {
	s.state = state.New(nil)
	s.project = &workspacebackend.Project{Path: "/home/testuser", ProjectId: "42ws42ws"}
}

func (s *S) TestCreateHook(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	var sdk = workspacebackend.Sdk{Name: "go", Channel: "latest/stable"}

	envs := []map[string]string{
		{},
		{"SDK_STATE_DIR": "/var/lib/workspace/state/sdk/go"},
		{"SDK_STATE_DIR": "/var/lib/workspace/state/sdk/go"},
	}

	for num, i := range []hookstate.WorkspaceHookType{
		hookstate.SetupBase,
		hookstate.SaveState,
		hookstate.RestoreState,
	} {
		task := hookstate.SetupHook(s.state, "ws", s.project.ProjectId, &sdk, i)

		var hookSetup hookstate.HookSetup
		err := task.Get("hook-setup", &hookSetup)
		c.Assert(err, check.IsNil)
		c.Assert(hookSetup.Type(), check.Equals, i.String())
		c.Assert(hookSetup.Sdk, check.DeepEquals, sdk)
		c.Assert(hookSetup.Environment, check.DeepEquals, envs[num])
		c.Check(task.Summary(), check.Equals, fmt.Sprintf("Run hook %q for \"go\" SDK", hookSetup.Type()))
	}
}
