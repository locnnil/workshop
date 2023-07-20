package statecontext_test

import (
	"context"
	"errors"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/overlord/statecontext"
	"github.com/canonical/workspace/internal/testutil"
	"github.com/canonical/workspace/internal/workspacebackend"
	"gopkg.in/check.v1"
	"gopkg.in/tomb.v2"
)

type CommonStateFuncs struct {
	state   *state.State
	project *workspacebackend.Project
}

var _ = check.Suite(&CommonStateFuncs{})

func (s *CommonStateFuncs) setupTask() *state.Task {
	s.state.Lock()
	defer s.state.Unlock()

	chg := s.state.NewChange("test", "...")
	chg.Set("user", "testuser")

	t := s.state.NewTask("task", "...")
	t.Set("workspace", "ws")
	t.Set("project", &s.project)
	chg.AddTask(t)
	return t
}

func (s *CommonStateFuncs) SetUpTest(c *check.C) {
	s.state = state.New(nil)
	s.project = &workspacebackend.Project{Path: c.MkDir(), ProjectId: "42ws42ws"}
}

func (s *CommonStateFuncs) TestContextCancelled(c *check.C) {
	handler := statecontext.WaitOnErrorDecorator(func(task *state.Task, tomb *tomb.Tomb) error {
		return context.Canceled
	})
	task := s.setupTask()
	err := handler(task, nil)
	c.Assert(err, check.IsNil)
}

func (s *CommonStateFuncs) TestExecutionError(c *check.C) {
	handler := statecontext.WaitOnErrorDecorator(func(task *state.Task, tomb *tomb.Tomb) error {
		return errors.New("task failed")
	})
	task := s.setupTask()
	err := handler(task, nil)
	c.Assert(err, check.ErrorMatches, "task failed")
	var ops statecontext.Operations
	s.state.Lock()
	defer s.state.Unlock()

	err = s.state.Get(statecontext.OpsInProgressKey, &ops)
	c.Assert(err, testutil.ErrorIs, state.ErrNoState)
}

func (s *CommonStateFuncs) TestRefreshInProgressError(c *check.C) {
	handler := statecontext.WaitOnErrorDecorator(func(task *state.Task, tomb *tomb.Tomb) error {
		return errors.New("task failed")
	})
	s.state.Lock()
	err := statecontext.StartRefresh(s.state, "ws", s.project.ProjectId, "1", true)
	c.Assert(err, check.IsNil)
	s.state.Unlock()

	task := s.setupTask()
	err = handler(task, nil)
	expected := state.Wait{Reason: "wait on error: task failed", WaitedStatus: state.DoingStatus}
	c.Assert(err, check.ErrorMatches, expected.Error())

	s.state.Lock()
	defer s.state.Unlock()

	op, in := statecontext.RefreshInProgress(s.state, "ws", s.project.ProjectId)
	c.Assert(in, check.Equals, true)
	c.Assert(op.ChangeId, check.Equals, "1")
	c.Assert(op.Operation, check.Equals, "refresh")
}
