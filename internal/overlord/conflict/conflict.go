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

package conflict

import (
	"errors"
	"fmt"
	"slices"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
)

type ChangeActionMismatch string

// ChangeConflictError represents an error because of snap conflicts between changes.
type ChangeConflictError struct {
	ProjectId    string
	Workshop     string
	ChangeKind   string
	ChangeStatus string
	// a Message is optional, otherwise one is composed from the other information
	Message string
	// ChangeID can optionally be set to the ID of the change with which the operation conflicts
	ChangeID string
}

type ChangeSetup struct {
	Mode string `json:"mode"`
}

type Mode int

const (
	ChangeTransactional Mode = iota
	ChangeWaitOnError
	ChangeContinue
	ChangeAbort
)

// ErrorNoWaitingChange signals that an abort or continue had no change to
// resume: no change is in progress for the workshop at all. Match it with
// [errors.Is].
var ErrorNoWaitingChange = errors.New("no waiting change in progress")

func (s Mode) String() string {
	return [...]string{"transactional", "wait-on-error", "continue", "abort"}[s]
}

func (s Mode) Resume() bool {
	return s == ChangeContinue || s == ChangeAbort
}

func ParseMode(s string) (Mode, error) {
	changeMap := map[string]Mode{
		ChangeTransactional.String(): ChangeTransactional,
		ChangeWaitOnError.String():   ChangeWaitOnError,
		ChangeContinue.String():      ChangeContinue,
		ChangeAbort.String():         ChangeAbort,
	}
	if val, ok := changeMap[s]; ok {
		return val, nil
	}
	return -1, errors.New(`change mode must be any of: "transactional", "wait-on-error", "continue", "abort"`)
}

type RefreshOption int

const (
	RefreshUpdate RefreshOption = iota
	RefreshRestore
)

func (s RefreshOption) String() string {
	return [...]string{"update", "restore"}[s]
}

func ParseRefreshSetting(s string) (RefreshOption, error) {
	refreshMap := map[string]RefreshOption{
		RefreshUpdate.String():  RefreshUpdate,
		RefreshRestore.String(): RefreshRestore,
	}
	if val, ok := refreshMap[s]; ok {
		return val, nil
	}
	return -1, errors.New(`refresh behaviour must be any of: "update", "restore"`)
}

func (e ChangeActionMismatch) Error() string {
	return string(e)
}

func (e *ChangeConflictError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.ChangeKind != "" {
		return fmt.Sprintf("workshop %q has %q change in progress", e.Workshop, e.ChangeKind)
	}
	return fmt.Sprintf("workshop %q has changes in progress", e.Workshop)
}

func checkWorkshop(task *state.Task, projectId, workshop string) (bool, error) {
	chg := task.Change()

	if task.Kind() == "disconnect" {
		// disconnect can affect more then one workshop
		var plugRef sdk.PlugRef
		var slotRef sdk.SlotRef
		if err := task.Get("plug", &plugRef); err != nil {
			return false, err
		}
		if err := task.Get("slot", &slotRef); err != nil {
			return false, err
		}

		if projectId == plugRef.ProjectId && workshop == plugRef.Workshop {
			return true, nil
		}

		if projectId == slotRef.ProjectId && workshop == slotRef.Workshop {
			return true, nil
		}
		return false, nil
	}

	if !chg.Has("project-id") || !task.Has("workshop") {
		return false, nil
	}

	var taskWorkshop, chgProject string
	if err := task.Get("workshop", &taskWorkshop); err != nil {
		return false, fmt.Errorf("internal error: cannot obtain workshop name from task: %s", task.Summary())
	}

	if err := chg.Get("project-id", &chgProject); err != nil {
		return false, fmt.Errorf("internal error: cannot obtain project from task: %s", task.Summary())
	}

	if projectId == chgProject && workshop == taskWorkshop {
		return true, nil
	}
	return false, nil
}

// Iterates over the list of running tasks and returns either nil or
// a change running for the provided projectID / workshop pair.
// Ignores certain kinds of changes based on the ignoreKinds argument.
// Ignore discard-background changes.
func findRunningChange(st *state.State, projectId, workshop string, ignoreKinds []string) (*state.Change, error) {
	for _, task := range st.Tasks() {
		chg := task.Change()
		if chg.IsReady() || slices.Contains(ignoreKinds, chg.Kind()) {
			continue
		}

		var discardBackground bool
		err := chg.Get("discard-background", &discardBackground)
		if err != nil && !errors.Is(err, state.ErrNoState) {
			return nil, err
		}
		if discardBackground {
			continue
		}

		ok, err := checkWorkshop(task, projectId, workshop)
		if err != nil {
			return nil, err
		}
		if ok {
			return chg, nil
		}
	}
	return nil, nil
}

// Iterates over the list of running tasks and returns a ChangeConflictError if
// there is a change running for the provided projectID / workshop pair.
// Ignores certain kinds of changes based on the ignoreKinds argument.
// Ignore discarded changes.
func CheckChangeConflict(st *state.State, projectId, workshop string, ignoreKinds []string) error {
	chg, err := findRunningChange(st, projectId, workshop, ignoreKinds)
	if err != nil {
		return err
	}
	if chg == nil {
		return nil
	}
	return &ChangeConflictError{
		ProjectId:    projectId,
		Workshop:     workshop,
		ChangeKind:   chg.Kind(),
		ChangeStatus: chg.Status().String(),
		ChangeID:     chg.ID(),
	}
}

func BackgroundDiscard(chg *state.Change, workshop string) {
	for _, tsk := range chg.Tasks() {
		if tsk.Status() == state.WaitStatus {
			tsk.SetStatus(state.DoStatus)
			tsk.Logf("Discarding %q for %q workshop...", chg.Kind(), workshop)
		}
	}

	// "discard-background" changes:
	//  1. skip all undo handlers when aborting
	//  2. are ignored by CheckChangeConflict
	chg.Set("discard-background", true)

	chg.Abort()
}

// Attempt to resume the change associated with the Resume/Launch operation
// for the given workshop. Depending on the mode the change will either be
// turned into Doing (Continue mode) or Abort (Abort mode).
func ResumeAfterWait(
	st *state.State, workshop string, projectId string, mode Mode, action string,
) (*state.Change, error) {
	if mode != ChangeAbort && mode != ChangeContinue {
		return nil, fmt.Errorf("cannot resume: only abort or continue can be used to resume the operation")
	}

	chg, err := findRunningChange(st, projectId, workshop, []string{"exec"})
	if err != nil {
		return nil, err
	}
	if chg == nil {
		return nil, fmt.Errorf("cannot %s: %w", mode, ErrorNoWaitingChange)
	}

	// The change exists but cannot be resumed: it is either a different kind
	// than requested, or the matching kind still running rather than paused
	// waiting on error. Either way a change is in progress for the workshop.
	if chg.Kind() != action || chg.Status() != state.WaitStatus {
		return nil, &ChangeConflictError{
			ProjectId:    projectId,
			Workshop:     workshop,
			ChangeKind:   chg.Kind(),
			ChangeStatus: chg.Status().String(),
			ChangeID:     chg.ID(),
		}
	}

	if mode == ChangeContinue {
		attempt, err := ChangeAttempt(chg)
		if err != nil {
			return nil, err
		}
		chg.Set("attempt", attempt+1)
	}

	for _, tsk := range chg.Tasks() {
		if tsk.Status() == state.WaitStatus {
			switch mode {
			case ChangeContinue:
				waited := tsk.WaitedStatus()
				tsk.SetStatus(waited)
				tsk.Logf("Continuing %q for workshop %q...", chg.Kind(), workshop)
			case ChangeAbort:
				tsk.SetStatus(state.DoStatus)
				tsk.Logf("Aborting %q for workshop %q...", chg.Kind(), workshop)
			}
		}
	}

	if mode == ChangeAbort {
		chg.Abort()
	}

	return chg, nil
}

func ChangeAttempt(change *state.Change) (int, error) {
	var attempt int
	if err := change.Get("attempt", &attempt); errors.Is(err, state.ErrNoState) {
		attempt = 1
	} else if err != nil {
		return 0, fmt.Errorf("internal error: change attempt counter invalid (change ID: %s)", change.ID())
	}
	return attempt, nil
}

// Iterates over the list of running tasks and returns a change ID if
// there is a change running for the provided projectID, workshop and kind.
// Ignore discard-background changes.
func FindChangeByKind(st *state.State, projectId, workshop, kind string) (string, error) {
	for _, task := range st.Tasks() {
		chg := task.Change()
		if chg.IsReady() || chg.Kind() != kind {
			continue
		}

		var discardBackground bool
		err := chg.Get("discard-background", &discardBackground)
		if err != nil && !errors.Is(err, state.ErrNoState) {
			return "", err
		}
		if discardBackground {
			continue
		}

		ok, err := checkWorkshop(task, projectId, workshop)
		if err != nil {
			return "", err
		}
		if ok {
			return chg.ID(), nil
		}
	}
	return "", errors.New("change not found")
}
