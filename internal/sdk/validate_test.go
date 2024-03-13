package sdk_test

import (
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"gopkg.in/check.v1"
)

type ValidateSuite struct {
	testutil.BaseTest
	projectId string
}

var _ = check.Suite(&ValidateSuite{})

func (s *ValidateSuite) SetUpTest(c *check.C) {
	s.BaseTest.SetUpTest(c)
	s.BaseTest.AddCleanup(sdk.MockSanitizePlugsSlots(func(snapInfo *sdk.Info) {}))
	s.projectId = "prj-4242"
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

func (s *ValidateSuite) TestIllegalSdkName(c *check.C) {
	info, err := sdk.ReadSdkInfo([]byte(`name: foo.something
`), s.projectId, "ws")
	c.Assert(err, check.IsNil)

	err = sdk.Validate(info)
	c.Check(err, check.ErrorMatches, `invalid sdk name: "foo.something"`)
}

func (s *ValidateSuite) TestIllegalSdkBase(c *check.C) {
	info, err := sdk.ReadSdkInfo([]byte(`name: foo.something
base: ubuntu@20.04
`), s.projectId, "ws")
	c.Assert(err, check.IsNil)

	err = sdk.Validate(info)
	c.Check(err, check.ErrorMatches, `invalid sdk name: "foo.something"`)
}
