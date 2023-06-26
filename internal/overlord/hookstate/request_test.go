package hookstate_test

import (
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

	var sdk = workspacebackend.Sdk{Name: "sdk", Channel: "latest/stable"}
	task := hookstate.SetupHook(s.state, &sdk, "1", workspacebackend.SetupBase)

	var hookSetup hookstate.HookSetup
	err := task.Get("hook-setup", &hookSetup)
	c.Assert(err, check.IsNil)
	c.Assert(hookSetup.Type(), check.Equals, workspacebackend.SetupBase.String())
}
