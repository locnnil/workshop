package sdk_test

import (
	"github.com/canonical/workspace/internal/sdk"
	"github.com/canonical/workspace/internal/testutil"
	"gopkg.in/check.v1"
)

type ValidateSuite struct {
	testutil.BaseTest
}

var _ = check.Suite(&ValidateSuite{})

func (s *ValidateSuite) SetUpTest(c *check.C) {
	s.BaseTest.SetUpTest(c)
}

func (s *ValidateSuite) TearDownTest(c *check.C) {
	s.BaseTest.TearDownTest(c)
}

func (s *ValidateSuite) TestValidateSlotPlugInterfaceName(c *check.C) {
	valid := []string{
		"a",
		"aaa",
		"a-a",
		"aa-a",
		"a-aa",
		"a-b-c",
		"valid",
		"valid-123",
	}
	for _, name := range valid {
		err := sdk.ValidateSlotName(name)
		c.Assert(err, check.IsNil)
		err = sdk.ValidatePlugName(name)
		c.Assert(err, check.IsNil)
		err = sdk.ValidateInterfaceName(name)
		c.Assert(err, check.IsNil)
	}
	invalid := []string{
		"",
		"a a",
		"a--a",
		"-a",
		"a-",
		"0",
		"123",
		"123abc",
		"日本語",
	}
	for _, name := range invalid {
		err := sdk.ValidateSlotName(name)
		c.Assert(err, check.ErrorMatches, `invalid slot name: ".*"`)
		err = sdk.ValidatePlugName(name)
		c.Assert(err, check.ErrorMatches, `invalid plug name: ".*"`)
		err = sdk.ValidateInterfaceName(name)
		c.Assert(err, check.ErrorMatches, `invalid interface name: ".*"`)
	}
}
