package main

import (
	"os"
	"path/filepath"

	. "gopkg.in/check.v1"
)

func (m *Main) TestHomeDirectoryPathContraction(c *C) {
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
