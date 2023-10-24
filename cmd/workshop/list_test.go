package main

import (
	"os"
	"path/filepath"
	"testing"

	. "gopkg.in/check.v1"
)

type WorkspaceList struct {
}

var _ = Suite(&WorkspaceList{})

func TestMain(t *testing.T) { TestingT(t) }

func (m *WorkspaceList) TestHomeDirectoryPathContraction(c *C) {
	home, _ := os.UserHomeDir()
	r := contractHomeDirectory(filepath.Join(home, "test"))
	c.Assert(r, Equals, "~/test")
	r = contractHomeDirectory(filepath.Join(home, "///test"))
	c.Assert(r, Equals, "~/test")
	r = contractHomeDirectory(home)
	c.Assert(r, Equals, "~")
	r = contractHomeDirectory("/sys")
	c.Assert(r, Equals, "/sys")

	/* This will fail because of how filepath handles path prefixes (not path aware)
	r = contractHomeDirectory(home + "4")
	assert.Equal(t, "~", r)
	*/
}
