package statecontext_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/canonical/workshop/internal/overlord/conflict"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/statecontext"
	"github.com/canonical/workshop/internal/workshopbackend"
	"gopkg.in/check.v1"
	"gopkg.in/tomb.v2"
)

type CommonStateFuncs struct {
	state   *state.State
	project *workshopbackend.Project
}

var _ = check.Suite(&CommonStateFuncs{})

func Test(t *testing.T) { check.TestingT(t) }

func (s *CommonStateFuncs) setupTask() *state.Task {
	s.state.Lock()
	defer s.state.Unlock()

	chg := s.state.NewChange("refresh", "...")
	chg.Set("user", "testuser")

	t := s.state.NewTask("task", "...")
	t.Set("workshop", "ws")
	t.Set("project", *s.project)
	chg.AddTask(t)
	return t
}

func (s *CommonStateFuncs) SetUpTest(c *check.C) {
	s.state = state.New(nil)
	s.project = &workshopbackend.Project{Path: c.MkDir(), ProjectId: "42ws42ws"}
}

func (s *CommonStateFuncs) TestContextCancelled(c *check.C) {
	task := s.setupTask()

	s.state.Lock()
	chg := task.Change()
	chg.Set("refresh-setup", conflict.RefreshSetup{Mode: conflict.RefreshWaitOnError.String()})
	chg.Set("project-id", s.project.ProjectId)
	task.Change().Abort()
	s.state.Unlock()

	handler := statecontext.OnDo(func(task *state.Task, tomb *tomb.Tomb) error {
		return fmt.Errorf("execution error %w", context.Canceled)
	})
	err := handler(task, nil)
	c.Assert(err, check.IsNil)
}

func (s *CommonStateFuncs) TestRefreshWaitOnError(c *check.C) {
	handler := statecontext.OnDo(func(task *state.Task, tomb *tomb.Tomb) error {
		return errors.New("task failed")
	})

	task := s.setupTask()
	s.state.Lock()
	chg := task.Change()
	chg.Set("refresh-setup", conflict.RefreshSetup{Mode: conflict.RefreshWaitOnError.String()})
	chg.Set("project-id", s.project.ProjectId)
	s.state.Unlock()

	err := handler(task, nil)
	expected := state.Wait{Reason: "wait on error: task failed", WaitedStatus: state.DoingStatus}
	c.Assert(err, check.ErrorMatches, expected.Error())
	s.state.Lock()
	c.Assert(task.Log(), check.HasLen, 2)
	c.Assert(task.Log()[0], check.Matches, ".*Setting the task to wait until the refresh is either aborted or continued...")
	c.Assert(task.Log()[1], check.Matches, ".*task failed")
	s.state.Unlock()

}

func (s *CommonStateFuncs) TestExecutionOnDoRetry(c *check.C) {
	task := s.setupTask()

	handler := statecontext.OnDo(func(task *state.Task, tomb *tomb.Tomb) error {
		return &state.Retry{Reason: "not enough time"}
	})

	err := handler(task, nil)
	c.Assert(err, check.ErrorMatches, "task should be retried")
	c.Assert(err.(*state.Retry).Reason, check.Equals, "not enough time")
}
