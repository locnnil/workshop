package main

import (
	"testing"

	"github.com/canonical/workspace/internal/overlord/projectstate"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestGetProjectDirectory(t *testing.T) {
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

		assert.Equal(t, i.expected, path)
		assert.ErrorIs(t, i.err, err)

	}
}
