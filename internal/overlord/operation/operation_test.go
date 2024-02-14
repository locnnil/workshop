package operation_test

import (
	"testing"

	"github.com/canonical/workshop/internal/overlord/operation"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshopbackend"
	"gopkg.in/check.v1"
)

type OperationSuite struct {
	state   *state.State
	project *workshopbackend.Project
}

var _ = check.Suite(&OperationSuite{})

func Test(t *testing.T) { check.TestingT(t) }

func (s *OperationSuite) SetUpTest(c *check.C) {
	s.state = state.New(nil)
	s.project = &workshopbackend.Project{Path: c.MkDir(), ProjectId: "42ws42ws"}
}

func (s *OperationSuite) TestResumeRefreshNothingInProgress(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	_, err := operation.ResumeRefresh(s.state, "ws", s.project.ProjectId, operation.RefreshContinue)
	c.Check(err, check.ErrorMatches, ".* no refresh in progress")
}

func (s *OperationSuite) TestResumeRefreshAnotherOperationInProgress(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	operation.StartOperation(s.state, "ws", s.project.ProjectId, operation.Operation{ChangeId: "1", Operation: operation.OperationStart, WaitOnError: true})
	change := s.state.NewChange("start", "...")
	task := s.state.NewTask("no-op", "...")
	task.SetToWait(state.DoingStatus)
	change.AddTask(task)

	_, err := operation.ResumeRefresh(s.state, "ws", s.project.ProjectId, operation.RefreshContinue)
	c.Check(err, check.ErrorMatches, ".*cannot resume, no refresh in progress.*")
}

func (s *OperationSuite) TestResumeRefreshContinue(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	operation.StartOperation(s.state, "ws", s.project.ProjectId, operation.Operation{ChangeId: "1", Operation: operation.OperationRefresh, WaitOnError: true})
	change := s.state.NewChange("refresh", "...")
	task := s.state.NewTask("no-op", "...")
	task.SetToWait(state.DoingStatus)
	change.AddTask(task)

	_, err := operation.ResumeRefresh(s.state, "ws", s.project.ProjectId, operation.RefreshContinue)
	c.Check(err, check.IsNil)
	c.Assert(task.Status(), check.Equals, state.DoingStatus)
}

func (s *OperationSuite) TestResumeRefreshAbort(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	operation.StartOperation(s.state, "ws", s.project.ProjectId, operation.Operation{ChangeId: "1", Operation: operation.OperationRefresh, WaitOnError: true})
	change := s.state.NewChange("refresh", "...")
	task := s.state.NewTask("no-op", "...")
	change.AddTask(task)
	task.SetToWait(state.DoingStatus)

	_, err := operation.ResumeRefresh(s.state, "ws", s.project.ProjectId, operation.RefreshAbort)
	c.Check(err, check.IsNil)
	c.Assert(task.Status(), check.Equals, state.ErrorStatus)
}
