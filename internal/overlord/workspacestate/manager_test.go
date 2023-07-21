package workspacestate_test

import (
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/overlord/statecontext"
	"github.com/canonical/workspace/internal/overlord/workspacestate"
	"github.com/canonical/workspace/internal/testutil"
	"github.com/canonical/workspace/internal/workspacebackend"
	"gopkg.in/check.v1"
)

type ManagerSuite struct {
	state   *state.State
	backend workspacebackend.WorkspaceBackend
	runner  *state.TaskRunner
	manager *workspacestate.WorkspaceManager
}

var _ = check.Suite(&ManagerSuite{})

func (s *ManagerSuite) SetUpTest(c *check.C) {
	s.state = state.New(nil)
	s.backend = workspacebackend.NewFakeWorkspaceBackend()
	s.runner = state.NewTaskRunner(s.state)
	s.manager = workspacestate.NewWorkspaceManager(s.state, s.runner, s.backend)
}

func (s *ManagerSuite) TestAddHandlers(c *check.C) {
	workspacestate.NewWorkspaceManager(s.state, s.runner, s.backend)

	c.Assert(s.runner.KnownTaskKinds(), testutil.DeepUnsortedMatches, []string{
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

func (s *ManagerSuite) setupWorkspace(running bool) *workspacebackend.Workspace {
	wrkspc := workspacebackend.NewWorkspace(s.backend, "ws", "42424242")
	wrkspc.SetRunning(running)
	return wrkspc
}

func (s *ManagerSuite) TestWorkspaceStateChanges(c *check.C) {
	type stateSetup struct {
		status            state.Status
		running           bool
		refreshInProgress bool
		hasErrors         bool
		expectedState     workspacebackend.WorkspaceState
		expectedErrors    []workspacebackend.WorkspaceErrorType
	}
	cases := []stateSetup{
		// running, no operation in progress, no errors
		{state.DefaultStatus, true, false, false, workspacebackend.WorkspaceReady, nil},
		// running, no operation in prorgess, has errors
		{state.DefaultStatus, true, false, true, workspacebackend.WorkspaceError, []workspacebackend.WorkspaceErrorType{workspacebackend.MissingFile}},
		// not running, no operation in prorgess, no errors
		{state.DefaultStatus, false, false, false, workspacebackend.WorkspaceStopped, nil},
		// not running, no operation in prorgess, has errors
		{state.DefaultStatus, false, false, true, workspacebackend.WorkspaceError, []workspacebackend.WorkspaceErrorType{workspacebackend.MissingFile}},
		// running, has operation in prorgess, waits on error
		{state.WaitStatus, true, true, false, workspacebackend.WorkspacePending, []workspacebackend.WorkspaceErrorType{workspacebackend.WaitOnError}},
		// running, has operation in prorgess, no errors
		{state.DoingStatus, true, true, false, workspacebackend.WorkspacePending, nil},
		// running, has operation in prorgess, has errors
		{state.DoingStatus, true, true, true, workspacebackend.WorkspaceError, []workspacebackend.WorkspaceErrorType{workspacebackend.MissingFile}},
		// not running, has operation in prorgess, no errors
		{state.DoingStatus, false, true, false, workspacebackend.WorkspacePending, nil},
		// not running, has operation in prorgess, has errors
		{state.DoingStatus, false, true, true, workspacebackend.WorkspaceError, []workspacebackend.WorkspaceErrorType{workspacebackend.MissingFile}},
	}

	s.state.Lock()
	defer s.state.Unlock()

	for i, setup := range cases {
		// setup
		wrkspc := s.setupWorkspace(setup.running)
		if setup.hasErrors {
			// add any error to emulate error state
			wrkspc.AddError(workspacebackend.MissingFile)
		}
		if setup.refreshInProgress {
			chg := s.state.NewChange("test", "...")
			chg.SetStatus(setup.status)
			err := statecontext.StartRefresh(s.state, "ws", "42424242", chg.ID(), true)
			c.Assert(err, check.IsNil)
		}

		// validate
		st := workspacestate.WorkspaceState(s.manager, wrkspc)
		c.Assert(st, check.Equals, setup.expectedState, check.Commentf("case num: %v", i))
		c.Assert(wrkspc.Errors(), testutil.DeepUnsortedMatches, setup.expectedErrors, check.Commentf("case num: %v", i))
	}
}
