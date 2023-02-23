package workspace

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestEnumWorkspacesInACWD(t *testing.T) {
	fs := afero.NewMemMapFs()
	project, _ := NewProject(nil, fs, "/")
	afero.WriteFile(fs, ".workspace.project1.yaml", []byte(""), 0644)
	afero.WriteFile(fs, ".workspace.project2.yaml", []byte(""), 0644)
	afero.WriteFile(fs, "workspace.project3.yaml", []byte(""), 0644)

	fs.Mkdir(".workspace.project2dir.yaml", 0755)
	afero.WriteFile(fs, ".workspace.yaml", []byte(""), 0644)

	ws, err := project.EnumWorkspaces()
	assert.Contains(t, ws, "project1")
	assert.Equal(t, ws["project1"].Name, "project1")
	assert.Contains(t, ws, "project2")
	assert.Equal(t, ws["project2"].Name, "project2")
	assert.NotContains(t, ws, "")
	assert.NotContains(t, ws, "project2dir")
	assert.NotContains(t, ws, "project3")

	assert.NoError(t, err)
}

func TestEnumWorkspacesInAGivenProject(t *testing.T) {
	fs := afero.NewOsFs()
	fs.MkdirAll("/tmp/experiments", 0755)
	defer fs.RemoveAll("/tmp/experiments")

	project, err := NewProject(nil, fs, "/tmp/experiments")
	assert.NoError(t, err)
	afero.WriteFile(fs, filepath.Join(project.GetProjectDirectory(), ".workspace.project1.yaml"), []byte(""), 0755)

	/* No relative paths are allowed */
	project, err = NewProject(nil, fs, "../tmp/experiments")
	assert.Nil(t, project)
	assert.Error(t, ErrNoRelativePathsAllowed, err)

	/* Could not read the project directory */
	project, err = NewProject(nil, fs, "/invalid&")
	assert.Nil(t, project)
	assert.Error(t, err)
}
