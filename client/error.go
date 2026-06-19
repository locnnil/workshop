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

package client

import "fmt"

// ChangeConflictError describes an operation blocked by another change.
type ChangeConflictError struct {
	// ChangeID is the ID of the blocking change.
	ChangeID string

	// ChangeKind is the kind of the blocking change, such as "refresh".
	ChangeKind string

	// ProjectID is the ID of the project containing the blocked workshop.
	ProjectID string

	// Workshop is the name of the blocked workshop.
	Workshop string
}

// WaitingChangeError describes an abort or continue request that could not be
// applied because no change is paused waiting on error for the workshop.
type WaitingChangeError struct {
	// Reason classifies why no waiting change was available. It is one of the
	// WaitingChange* reason constants.
	Reason WaitingChangeReason
}

// WaitingChangeReason classifies why an abort or continue had no change paused
// waiting on error to act on. Its values are carried in the change-not-waiting
// API error.
type WaitingChangeReason string

const (
	// WaitingChangeNoChange indicates no change is in progress for the
	// workshop, so there is nothing for an abort or continue to act on.
	WaitingChangeNoChange WaitingChangeReason = "no-change"

	// WaitingChangeRunning indicates the change to be resumed exists but is
	// still running rather than paused waiting on error.
	WaitingChangeRunning WaitingChangeReason = "running"
)

// As maps generic API errors into richer client-side error types.
func (e *Error) As(target any) bool {
	switch e.Kind {
	case ErrorKindChangeConflict:
		conflict, ok := target.(*ChangeConflictError)
		if !ok {
			return false
		}
		return toChangeConflictError(*e, conflict)
	case ErrorKindNoWaitingChange:
		waiting, ok := target.(*WaitingChangeError)
		if !ok {
			return false
		}
		return toWaitingChangeError(*e, waiting)
	default:
		return false
	}
}

// Error returns a human-readable description of the blocking change.
func (e ChangeConflictError) Error() string {
	if e.ChangeKind != "" {
		return fmt.Sprintf(
			"workshop %q has %q change in progress",
			e.Workshop,
			e.ChangeKind,
		)
	}
	return fmt.Sprintf("workshop %q has changes in progress", e.Workshop)
}

// Error returns a human-readable fallback description. Callers that can render
// their own message should branch on [WaitingChangeError.Reason] instead.
func (e WaitingChangeError) Error() string {
	return "no waiting change in progress"
}

// toChangeConflictError extracts change-conflict details from a generic API
// error. It returns true when the error value has the expected object shape,
// even if some individual fields are missing or not strings; in that case the
// corresponding [ChangeConflictError] fields remain empty. It returns false
// when the error value is not an object and therefore cannot represent a
// change conflict payload.
func toChangeConflictError(err Error, conflict *ChangeConflictError) bool {
	value, ok := err.Value.(map[string]any)
	if !ok {
		return false
	}

	changeID, _ := value["change-id"].(string)
	changeKind, _ := value["change-kind"].(string)
	projectID, _ := value["project-id"].(string)
	workshop, _ := value["workshop"].(string)

	*conflict = ChangeConflictError{
		ChangeID:   changeID,
		ChangeKind: changeKind,
		ProjectID:  projectID,
		Workshop:   workshop,
	}
	return true
}

// toWaitingChangeError extracts change-not-waiting details from a generic API
// error. It returns true when the error value has the expected object shape,
// even if the reason is missing or not a string; in that case the
// [WaitingChangeError.Reason] field remains empty. It returns false when the
// error value is not an object and therefore cannot represent the payload.
func toWaitingChangeError(err Error, waiting *WaitingChangeError) bool {
	value, ok := err.Value.(map[string]any)
	if !ok {
		return false
	}

	reason, _ := value["reason"].(string)

	*waiting = WaitingChangeError{
		Reason: WaitingChangeReason(reason),
	}
	return true
}
