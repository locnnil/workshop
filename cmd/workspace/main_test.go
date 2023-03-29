package main

import (
	"testing"

	. "gopkg.in/check.v1"

	"github.com/canonical/workspace/internal/overlord/projectstate"
	"github.com/spf13/afero"
)

type Main struct {
}

var _ = Suite(&Main{})

func Test(t *testing.T) { TestingT(t) }

func (m *Main) TestGetProjectDirectory(c *C) {
	cases := []struct {
		project  string
		lockFile bool
		cwd      string
		expected string
		err      error
	}{

		// nested directory
		{"/home/user/", true, "/home/user/nested", "/home/user", nil},

		// nested directory
		{"/home/user/", true, "/home/user/test/very/deeply", "/home/user", nil},

		// same level
		{"/home/user/same", true, "/home/user/same", "/home/user/same", nil},

		// different cwd
		{"/home/user/different", true, "/home", "/home", nil},

		// project is in root
		{"/", true, "/home/user/notroot", "/", nil},

		// .lock does not exist
		{"/home/user/nolock", false, "/home/user/test/nolock", "/home/user/test/nolock", nil},
	}
	for _, i := range cases {
		fs := afero.NewMemMapFs()
		fs.MkdirAll(i.project, 0755)
		fs.MkdirAll(i.cwd, 0755)
		if i.lockFile == true {
			fs.Create(projectstate.LockPath(i.project))
		}

		path, err := getProjectDirectory(fs, i.cwd)

		c.Check(path, Equals, i.expected)
		c.Check(err, Equals, i.err)
	}
}
