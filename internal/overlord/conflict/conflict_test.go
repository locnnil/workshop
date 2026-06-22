// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package conflict_test

import (
	"errors"
	"testing"

	"gopkg.in/check.v1"

	conflict "github.com/canonical/workshop/internal/overlord/conflict"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
)

type conflictSuite struct {
	state   *state.State
	project workshop.Project
}

var _ = check.Suite(&conflictSuite{})

func Test(t *testing.T) { check.TestingT(t) }

func (s *conflictSuite) newChange(kind string) *state.Change {
	change := s.state.NewChange(kind, "test")
	task := s.state.NewTask("test", "test")
	task.Set("workshop", "ws")
	change.AddTask(task)
	change.Set("project-id", s.project.ProjectId)
	return change
}

func (s *conflictSuite) newChangeDisconnect(kind string) *state.Change {
	change := s.state.NewChange(kind, "test")
	task := s.state.NewTask("disconnect", "test")
	task.Set("slot", sdk.SlotRef{ProjectId: s.project.ProjectId, Workshop: "ws"})
	task.Set("plug", sdk.SlotRef{ProjectId: s.project.ProjectId, Workshop: "another-ws"})
	change.AddTask(task)
	change.Set("project-id", s.project.ProjectId)
	return change
}

func (s *conflictSuite) SetUpTest(c *check.C) {
	s.state = state.New(nil)
	s.project = workshop.Project{Path: c.MkDir(), ProjectId: "42ws42ws"}
}

func (s *conflictSuite) TestCheckChangeConflictNotFound(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	c.Assert(conflict.CheckChangeConflict(s.state, s.project.ProjectId, "ws", nil), check.IsNil)
}

func (s *conflictSuite) TestCheckChangeConflictFound(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	change := s.newChange("launch")

	err := conflict.CheckChangeConflict(s.state, s.project.ProjectId, "ws", nil)
	c.Assert(err, check.NotNil)
	conflictErr, ok := err.(*conflict.ChangeConflictError)
	c.Assert(ok, check.Equals, true)
	c.Assert(conflictErr.ChangeID, check.Equals, change.ID())
	c.Assert(conflictErr.ChangeKind, check.Equals, "launch")
	c.Assert(conflictErr.ChangeStatus, check.Equals, "Do")
	c.Assert(conflictErr.Workshop, check.Equals, "ws")
	c.Assert(conflictErr.ProjectId, check.Equals, s.project.ProjectId)
}

func (s *conflictSuite) TestCheckChangeDisconnectConflictFound(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	change := s.newChangeDisconnect("disconnect")

	err := conflict.CheckChangeConflict(s.state, s.project.ProjectId, "ws", nil)
	c.Assert(err, check.NotNil)
	conflictErr, ok := err.(*conflict.ChangeConflictError)
	c.Assert(ok, check.Equals, true)
	c.Assert(conflictErr.ChangeID, check.Equals, change.ID())
	c.Assert(conflictErr.ChangeKind, check.Equals, "disconnect")
	c.Assert(conflictErr.ChangeStatus, check.Equals, "Do")
	c.Assert(conflictErr.Workshop, check.Equals, "ws")
	c.Assert(conflictErr.ProjectId, check.Equals, s.project.ProjectId)

	err = conflict.CheckChangeConflict(s.state, s.project.ProjectId, "another-ws", nil)
	c.Assert(err, check.NotNil)
	conflictErr, ok = err.(*conflict.ChangeConflictError)
	c.Assert(ok, check.Equals, true)
	c.Assert(conflictErr.ChangeID, check.Equals, change.ID())
	c.Assert(conflictErr.ChangeKind, check.Equals, "disconnect")
	c.Assert(conflictErr.ChangeStatus, check.Equals, "Do")
	c.Assert(conflictErr.Workshop, check.Equals, "another-ws")
	c.Assert(conflictErr.ProjectId, check.Equals, s.project.ProjectId)
}

func (s *conflictSuite) TestCheckChangeNoConflictWithReadyChange(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	change := s.newChange("launch")
	change.SetStatus(state.DoneStatus)

	c.Assert(conflict.CheckChangeConflict(s.state, s.project.ProjectId, "ws", nil), check.IsNil)
}

func (s *conflictSuite) TestCheckChangeConflictIgnoreKinds(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	launch := s.newChange("launch")
	launch.SetStatus(state.DoneStatus)
	s.newChange("exec")
	s.newChange("connect")

	c.Assert(conflict.CheckChangeConflict(s.state, s.project.ProjectId, "ws", nil), check.NotNil)

	c.Assert(conflict.CheckChangeConflict(s.state, s.project.ProjectId, "ws", []string{"connect", "exec"}), check.IsNil)
}

func (s *conflictSuite) TestFindChangeByKindNotFound(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	id, err := conflict.FindChangeByKind(s.state, s.project.ProjectId, "ws", "exec")
	c.Check(err, check.ErrorMatches, "change not found")
	c.Check(id, check.Equals, "")
}

func (s *conflictSuite) TestFindChangeByKindFound(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	change := s.newChange("exec")

	id, err := conflict.FindChangeByKind(s.state, s.project.ProjectId, "ws", "exec")
	c.Assert(err, check.IsNil)
	c.Check(id, check.Equals, change.ID())
}

func (s *conflictSuite) TestFindChangeByKindIgnoreReadyChanges(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	done := s.newChange("launch")
	done.SetStatus(state.DoneStatus)
	doing := s.newChange("launch")
	done = s.newChange("launch")
	done.SetStatus(state.DoneStatus)

	id, err := conflict.FindChangeByKind(s.state, s.project.ProjectId, "ws", "launch")
	c.Assert(err, check.IsNil)
	c.Check(id, check.Equals, doing.ID())
}

func (s *conflictSuite) TestFindChangeByKindIgnoreOtherKinds(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	s.newChange("launch")
	change := s.newChange("exec")
	s.newChange("launch")

	id, err := conflict.FindChangeByKind(s.state, s.project.ProjectId, "ws", "exec")
	c.Assert(err, check.IsNil)
	c.Check(id, check.Equals, change.ID())
}

func (s *conflictSuite) TestResumeAfterWaitNothingInProgress(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	_, err := conflict.ResumeAfterWait(s.state, "ws", s.project.ProjectId, conflict.ChangeContinue, "refresh")
	c.Check(errors.Is(err, conflict.ErrorNoWaitingChange), check.Equals, true)
	c.Check(err, check.ErrorMatches, "cannot continue: no waiting change in progress")
}

func (s *conflictSuite) TestResumeAfterWaitIncorrectMode(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	_, err := conflict.ResumeAfterWait(s.state, "ws", s.project.ProjectId, conflict.ChangeTransactional, "refresh")
	c.Check(err, check.ErrorMatches, "cannot resume: only abort or continue can be used to resume the operation")
}

func (s *conflictSuite) TestResumeAfterWaitWrongChangeKindInProgress(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	change := s.newChange("launch")

	_, err := conflict.ResumeAfterWait(s.state, "ws", s.project.ProjectId, conflict.ChangeContinue, "refresh")
	var conflictErr *conflict.ChangeConflictError
	c.Assert(errors.As(err, &conflictErr), check.Equals, true)
	c.Check(conflictErr, check.DeepEquals, &conflict.ChangeConflictError{
		ProjectId:    s.project.ProjectId,
		Workshop:     "ws",
		ChangeKind:   "launch",
		ChangeStatus: change.Status().String(),
		ChangeID:     change.ID(),
	})
}

func (s *conflictSuite) TestResumeAfterWaitNoWaitingOnError(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	change := s.newChange("refresh")

	_, err := conflict.ResumeAfterWait(s.state, "ws", s.project.ProjectId, conflict.ChangeContinue, "refresh")
	var conflictErr *conflict.ChangeConflictError
	c.Assert(errors.As(err, &conflictErr), check.Equals, true)
	c.Check(conflictErr, check.DeepEquals, &conflict.ChangeConflictError{
		ProjectId:    s.project.ProjectId,
		Workshop:     "ws",
		ChangeKind:   "refresh",
		ChangeStatus: change.Status().String(),
		ChangeID:     change.ID(),
	})
}

func (s *conflictSuite) TestResumeChangeContinue(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	change := s.newChange("refresh")
	task := change.Tasks()[0]
	task.SetToWait(state.HoldStatus)

	attempt, err := conflict.ChangeAttempt(change)
	c.Assert(err, check.IsNil)
	c.Check(attempt, check.Equals, 1)

	_, err = conflict.ResumeAfterWait(s.state, "ws", s.project.ProjectId, conflict.ChangeContinue, "refresh")
	c.Assert(err, check.IsNil)
	c.Assert(task.Status(), check.Equals, state.HoldStatus)

	attempt, err = conflict.ChangeAttempt(change)
	c.Assert(err, check.IsNil)
	c.Check(attempt, check.Equals, 2)
}

func (s *conflictSuite) TestResumeChangeAbort(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	change := s.newChange("refresh")
	task := change.Tasks()[0]
	task.SetToWait(state.DoingStatus)
	task2 := s.state.NewTask("do", "to do")
	// emulate tasks to be done down the lane
	// if abort is called for the change, the task will
	// be reverted to hold.
	task2.SetStatus(state.DoStatus)
	change.AddTask(task2)
	change.SetStatus(state.WaitStatus)

	_, err := conflict.ResumeAfterWait(s.state, "ws", s.project.ProjectId, conflict.ChangeAbort, "refresh")
	c.Assert(err, check.IsNil)
	c.Assert(task.Status(), check.Equals, state.HoldStatus)
	c.Assert(task2.Status(), check.Equals, state.HoldStatus)
}
