package statecontext_test

import (
	"testing"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/statecontext"
	"github.com/canonical/workshop/internal/workspacebackend"
	"gopkg.in/check.v1"
)

type OperationSuite struct {
	state   *state.State
	project *workspacebackend.Project
}

var _ = check.Suite(&OperationSuite{})

func Test(t *testing.T) { check.TestingT(t) }

func (s *OperationSuite) SetUpTest(c *check.C) {
	s.state = state.New(nil)
	s.project = &workspacebackend.Project{Path: c.MkDir(), ProjectId: "42ws42ws"}
}

func (s *OperationSuite) TestResumeRefreshNothingInProgress(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	_, err := statecontext.ResumeRefresh(s.state, "ws", s.project.ProjectId, statecontext.RefreshContinue)
	c.Check(err, check.ErrorMatches, ".* no refresh in progress")
}

func (s *OperationSuite) TestResumeRefreshContinue(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	statecontext.StartOperation(s.state, "ws", s.project.ProjectId, statecontext.Operation{ChangeId: "1", Operation: statecontext.OperationRefresh, WaitOnError: true})
	change := s.state.NewChange("refresh", "...")
	task := s.state.NewTask("no-op", "...")
	task.SetToWait(state.DoingStatus)
	change.AddTask(task)

	_, err := statecontext.ResumeRefresh(s.state, "ws", s.project.ProjectId, statecontext.RefreshContinue)
	c.Check(err, check.IsNil)
	c.Assert(task.Status(), check.Equals, state.DoingStatus)
}

func (s *OperationSuite) TestResumeRefreshAbort(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	statecontext.StartOperation(s.state, "ws", s.project.ProjectId, statecontext.Operation{ChangeId: "1", Operation: statecontext.OperationRefresh, WaitOnError: true})
	change := s.state.NewChange("refresh", "...")
	task := s.state.NewTask("no-op", "...")
	change.AddTask(task)
	task.SetToWait(state.DoingStatus)

	_, err := statecontext.ResumeRefresh(s.state, "ws", s.project.ProjectId, statecontext.RefreshAbort)
	c.Check(err, check.IsNil)
	c.Assert(task.Status(), check.Equals, state.ErrorStatus)
}
