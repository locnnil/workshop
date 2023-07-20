package workspacestate_test

import (
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/overlord/workspacestate"
	"github.com/canonical/workspace/internal/testutil"
	"github.com/canonical/workspace/internal/workspacebackend"
	"gopkg.in/check.v1"
)

type ManagerSuite struct {
	state   *state.State
	backend workspacebackend.WorkspaceBackend
}

var _ = check.Suite(&ManagerSuite{})

func (s *ManagerSuite) SetUpTest(c *check.C) {
	s.state = state.New(nil)
	s.backend = workspacebackend.NewFakeWorkspaceBackend()
}

func (s *ManagerSuite) TestAddHandlers(c *check.C) {
	runner := state.NewTaskRunner(s.state)

	workspacestate.NewWorkspaceManager(runner, s.backend)

	c.Assert(runner.KnownTaskKinds(), testutil.DeepUnsortedMatches, []string{
		"create-workspace",
		"start-workspace",
		"stop-workspace",
		"delete-workspace",
		"mount-project",
		"delete-workspace-copy",
		"make-workspace-copy",
		"create-state-storage",
		"remove-state-storage",
	})
}
