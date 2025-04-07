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
