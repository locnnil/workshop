package conflict_test

import (
	"testing"

	conflict "github.com/canonical/workshop/internal/overlord/conflict"
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

	_, err := conflict.ResumeRefresh(s.state, "ws", s.project.ProjectId, conflict.RefreshContinue)
	c.Check(err, check.ErrorMatches, ".* no refresh in progress")
}
