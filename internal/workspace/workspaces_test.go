package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestEnumWorkspacesInACWD(t *testing.T) {
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, ".workspace.project1.yaml", []byte(""), 0700)
	afero.WriteFile(fs, ".workspace.project2.yaml", []byte(""), 0700)
	afero.WriteFile(fs, "workspace.project3.yaml", []byte(""), 0700)

	fs.Mkdir(".workspace.project2dir.yaml", 0700)
	afero.WriteFile(fs, ".workspace.yaml", []byte(""), 0700)

	ws, err := EnumWorkspaces(fs, "/")
	assert.Contains(t, ws, "project1")
	assert.Contains(t, ws, "project2")
	assert.NotContains(t, ws, "")
	assert.NotContains(t, ws, "project2dir")
	assert.NotContains(t, ws, "project3")

	assert.NoError(t, err)
}

func TestEnumWorkspacesInAGivenProject(t *testing.T) {
	fs := afero.NewMemMapFs()
	project := "/work/experiments"
	fs.MkdirAll(project, os.ModeDir)
	afero.WriteFile(fs, filepath.Join(project, ".workspace.project1.yaml"), []byte(""), 0700)

	/* Test an absolute path use case */
	ws, err := EnumWorkspaces(fs, project)
	assert.Contains(t, ws, "project1")
	assert.NoError(t, err)

	/* No relative paths are allowed */
	ws, err = EnumWorkspaces(fs, "../experiments")
	assert.Nil(t, ws)
	assert.Error(t, err)

	/* Could not read the project directory */
	ws, err = EnumWorkspaces(fs, "/invalid&")
	assert.Nil(t, ws)
	assert.Error(t, err)
}
