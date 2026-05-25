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

package revert_test

import (
	"testing"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/revert"
)

type revertSuite struct {
	result []string
}

var _ = check.Suite(&revertSuite{})

func TestRevert(t *testing.T) { check.TestingT(t) }

func (r *revertSuite) SetUpTest(c *check.C) {
	r.result = []string{}
}

func (r *revertSuite) TestRevertFail(c *check.C) {
	defer func() {
		c.Assert(r.result, check.DeepEquals, []string{"2nd step", "1st step"})
	}()

	revert := revert.New()
	defer revert.Fail()

	// Revert functions are run in reverse order on return.
	revert.Add(func() { r.result = append(r.result, "1st step") })
	revert.Add(func() { r.result = append(r.result, "2nd step") })
}

func (r *revertSuite) TestRevertSuccess(c *check.C) {
	defer func() {
		c.Assert(r.result, check.DeepEquals, []string{})
	}()

	revert := revert.New()
	defer revert.Fail()

	revert.Add(func() { r.result = append(r.result, "ignored") })

	revert.Success()
}
