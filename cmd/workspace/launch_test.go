package main

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestEnumWorkspace(t *testing.T) {
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, ".workspace.project1.yaml", []byte(""), 0700)
	afero.WriteFile(fs, ".workspace.project2.yaml", []byte(""), 0700)
	afero.WriteFile(fs, "workspace.project3.yaml", []byte(""), 0700)

	fs.Mkdir(".workspace.project2dir.yaml", 0700)
	afero.WriteFile(fs, ".workspace.yaml", []byte(""), 0700)

	ws, err := enumWorkspaces(fs)
	assert.Contains(t, ws, "project1")
	assert.Contains(t, ws, "project2")
	assert.NotContains(t, ws, "")
	assert.NotContains(t, ws, "project2dir")
	assert.NotContains(t, ws, "project3")

	assert.NoError(t, err)
}
