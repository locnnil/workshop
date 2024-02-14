package statecontext_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/canonical/workshop/internal/overlord/operation"
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

func (s *CommonStateFuncs) setupTask() *state.Task {
	s.state.Lock()
	defer s.state.Unlock()

	chg := s.state.NewChange("test", "...")
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

func (s *CommonStateFuncs) TestStopTaskOperation(c *check.C) {
	task := s.setupTask()

	s.state.Lock()
	// mark task to stop the associated operation
	task.Set("stop-operation", true)
	err := operation.StartOperation(s.state, "ws", s.project.ProjectId, operation.Operation{ChangeId: "1", Operation: operation.OperationRefresh, WaitOnError: true})
	c.Assert(err, check.IsNil)
	task.Change().Abort()
	s.state.Unlock()

	handler := statecontext.OnDo(func(task *state.Task, tomb *tomb.Tomb) error {
		return nil
	})
	err = handler(task, nil)
	c.Assert(err, check.IsNil)

	s.state.Lock()
	op := operation.OperationInProgress(s.state, "ws", s.project.ProjectId)
	c.Assert(op, check.IsNil)
	s.state.Unlock()
}

func (s *CommonStateFuncs) TestContextCancelled(c *check.C) {
	task := s.setupTask()

	s.state.Lock()
	err := operation.StartOperation(s.state, "ws", s.project.ProjectId, operation.Operation{ChangeId: "1", Operation: operation.OperationRefresh, WaitOnError: true})
	c.Assert(err, check.IsNil)
	task.Change().Abort()
	s.state.Unlock()

	handler := statecontext.OnDo(func(task *state.Task, tomb *tomb.Tomb) error {
		return fmt.Errorf("execution error %w", context.Canceled)
	})
	err = handler(task, nil)
	c.Assert(err, check.IsNil)

	s.state.Lock()
	op := operation.OperationInProgress(s.state, "ws", s.project.ProjectId)
	c.Assert(op, check.IsNil)
	s.state.Unlock()
}

func (s *CommonStateFuncs) TestExecutionErrorOnDo(c *check.C) {
	task := s.setupTask()

	s.state.Lock()
	err := operation.StartOperation(s.state, "ws", s.project.ProjectId, operation.Operation{ChangeId: "1", Operation: operation.OperationRefresh, WaitOnError: false})
	c.Assert(err, check.IsNil)
	s.state.Unlock()

	handler := statecontext.OnDo(func(task *state.Task, tomb *tomb.Tomb) error {
		return errors.New("task failed")
	})

	err = handler(task, nil)
	c.Assert(err, check.ErrorMatches, "task failed")
	s.state.Lock()
	defer s.state.Unlock()

	op := operation.OperationInProgress(s.state, "ws", s.project.ProjectId)
	c.Assert(op, check.IsNil)
}

func (s *CommonStateFuncs) TestStartTaskOnUndo(c *check.C) {
	task := s.setupTask()

	s.state.Lock()
	task.Set("start-operation", true)
	err := operation.StartOperation(s.state, "ws", s.project.ProjectId, operation.Operation{ChangeId: "1", Operation: operation.OperationRefresh, WaitOnError: false})
	c.Assert(err, check.IsNil)
	s.state.Unlock()

	handler := statecontext.OnUndo(func(task *state.Task, tomb *tomb.Tomb) error {
		return nil
	})

	err = handler(task, nil)
	c.Assert(err, check.IsNil)
	s.state.Lock()
	defer s.state.Unlock()

	op := operation.OperationInProgress(s.state, "ws", s.project.ProjectId)
	c.Assert(op, check.IsNil)
}

func (s *CommonStateFuncs) TestRefreshInProgressError(c *check.C) {
	handler := statecontext.OnDo(func(task *state.Task, tomb *tomb.Tomb) error {
		return errors.New("task failed")
	})
	s.state.Lock()
	err := operation.StartOperation(s.state, "ws", s.project.ProjectId, operation.Operation{ChangeId: "1", Operation: operation.OperationRefresh, WaitOnError: true})
	c.Assert(err, check.IsNil)
	s.state.Unlock()

	task := s.setupTask()
	err = handler(task, nil)
	expected := state.Wait{Reason: "wait on error: task failed", WaitedStatus: state.DoingStatus}
	c.Assert(err, check.ErrorMatches, expected.Error())

	s.state.Lock()
	defer s.state.Unlock()

	op := operation.OperationInProgress(s.state, "ws", s.project.ProjectId)
	c.Assert(op, check.NotNil)
	c.Assert(op.ChangeId, check.Equals, "1")
	c.Assert(op.Operation, check.Equals, "refresh")
}
