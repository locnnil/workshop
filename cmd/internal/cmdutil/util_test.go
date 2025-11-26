package cmdutil

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/check.v1"
)

type cmdUtil struct {
}

func Test(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&cmdUtil{})

func (m *cmdUtil) TestHomeDirectoryPathContraction(c *check.C) {
	home, _ := os.UserHomeDir()
	r := ContractHome(filepath.Join(home, "test"))
	c.Assert(r, check.Equals, "~/test")
	r = ContractHome(filepath.Join(home, "///test"))
	c.Assert(r, check.Equals, "~/test")
	r = ContractHome(home)
	c.Assert(r, check.Equals, "~")
	r = ContractHome("/sys")
	c.Assert(r, check.Equals, "/sys")
}
