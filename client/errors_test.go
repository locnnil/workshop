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

package client_test

import (
	"errors"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/client"
)

// errorSuite checks structured API error conversion behaviour.
type errorSuite struct{}

var _ = check.Suite(&errorSuite{})

// TestChangeConflictErrorAsFullValue checks that a complete API error value
// maps to [client.ChangeConflictError].
func (errorSuite) TestChangeConflictErrorAsFullValue(c *check.C) {
	err := &client.Error{
		Kind: client.ErrorKindChangeConflict,
		Value: map[string]any{
			"change-id":   "29",
			"change-kind": "refresh",
			"project-id":  "project-1",
			"workshop":    "dev",
		},
	}

	var conflictErr client.ChangeConflictError
	c.Assert(errors.As(err, &conflictErr), check.Equals, true)
	c.Check(conflictErr, check.DeepEquals, client.ChangeConflictError{
		ChangeID:   "29",
		ChangeKind: "refresh",
		ProjectID:  "project-1",
		Workshop:   "dev",
	})
}

// TestChangeConflictErrorAsNonMapValue checks that unexpected API error value
// shapes do not map to [client.ChangeConflictError].
func (errorSuite) TestChangeConflictErrorAsNonMapValue(c *check.C) {
	err := &client.Error{
		Kind:  client.ErrorKindChangeConflict,
		Value: "not a map",
	}

	var conflictErr client.ChangeConflictError
	c.Check(errors.As(err, &conflictErr), check.Equals, false)
}

// TestChangeConflictErrorAsPartialValue checks that incomplete API error values
// still map to a partial [client.ChangeConflictError].
func (errorSuite) TestChangeConflictErrorAsPartialValue(c *check.C) {
	err := &client.Error{
		Kind: client.ErrorKindChangeConflict,
		Value: map[string]any{
			"change-id": "29",
			"workshop":  "dev",
		},
	}

	var conflictErr client.ChangeConflictError
	c.Assert(errors.As(err, &conflictErr), check.Equals, true)
	c.Check(conflictErr, check.DeepEquals, client.ChangeConflictError{
		ChangeID: "29",
		Workshop: "dev",
	})
}

// TestChangeConflictErrorAsWrongKind checks that unrelated API error kinds do
// not map to [client.ChangeConflictError].
func (errorSuite) TestChangeConflictErrorAsWrongKind(c *check.C) {
	err := &client.Error{
		Kind: client.ErrorKindNoUpdatesAvailable,
		Value: map[string]any{
			"change-id": "29",
		},
	}

	var conflictErr client.ChangeConflictError
	c.Check(errors.As(err, &conflictErr), check.Equals, false)
}

// TestWaitingChangeErrorAsMissingReason checks that a value object without a
// reason still maps, leaving the reason empty.
func (errorSuite) TestWaitingChangeErrorAsMissingReason(c *check.C) {
	err := &client.Error{
		Kind:  client.ErrorKindNoWaitingChange,
		Value: map[string]any{},
	}

	var waitingErr client.WaitingChangeError
	c.Assert(errors.As(err, &waitingErr), check.Equals, true)
	c.Check(waitingErr.Reason, check.Equals, client.WaitingChangeReason(""))
}

// TestWaitingChangeErrorAsNonMapValue checks that unexpected API error value
// shapes do not map to [client.WaitingChangeError].
func (errorSuite) TestWaitingChangeErrorAsNonMapValue(c *check.C) {
	err := &client.Error{
		Kind:  client.ErrorKindNoWaitingChange,
		Value: "not a map",
	}

	var waitingErr client.WaitingChangeError
	c.Check(errors.As(err, &waitingErr), check.Equals, false)
}

// TestWaitingChangeErrorAsReason checks that a change-not-waiting API error
// maps to [client.WaitingChangeError] carrying its reason.
func (errorSuite) TestWaitingChangeErrorAsReason(c *check.C) {
	err := &client.Error{
		Kind: client.ErrorKindNoWaitingChange,
		Value: map[string]any{
			"reason": string(client.WaitingChangeNoChange),
		},
	}

	var waitingErr client.WaitingChangeError
	c.Assert(errors.As(err, &waitingErr), check.Equals, true)
	c.Check(waitingErr, check.DeepEquals, client.WaitingChangeError{
		Reason: client.WaitingChangeNoChange,
	})
}

// TestWaitingChangeErrorAsWrongKind checks that unrelated API error kinds do
// not map to [client.WaitingChangeError].
func (errorSuite) TestWaitingChangeErrorAsWrongKind(c *check.C) {
	err := &client.Error{
		Kind:  client.ErrorKindChangeConflict,
		Value: map[string]any{"reason": string(client.WaitingChangeNoChange)},
	}

	var waitingErr client.WaitingChangeError
	c.Check(errors.As(err, &waitingErr), check.Equals, false)
}
