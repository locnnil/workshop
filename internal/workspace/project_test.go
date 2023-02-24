package workspace

import (
	"math/rand"
	"net/http"
	"path/filepath"
	"testing"

	util "github.com/canonical/workspace/internal"
	"github.com/canonical/workspace/internal/mocks"
	"github.com/canonical/workspace/internal/server"
	"github.com/lxc/lxd/shared/api"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ProjectTestSuite struct {
	suite.Suite
	Fs  afero.Fs
	Srv *mocks.MockWorkspaceServer
}

func TestRunProjectTests(t *testing.T) {
	suite.Run(t, &ProjectTestSuite{})
}

func (s *ProjectTestSuite) SetupTest() {
	s.Fs = afero.NewMemMapFs()
	s.Srv = mocks.NewMockWorkspaceServer(s.T())
	s.Fs.MkdirAll(util.DataDir, 0755)
	s.Fs.MkdirAll(util.SdksDir, 0755)
	rand.Seed(1)
}

func (s *ProjectTestSuite) TestEnumWorkspacesInACWD() {
	t := s.T()
	fs := afero.NewMemMapFs()
	project, _ := NewProject(nil, fs, "/")
	afero.WriteFile(fs, ".workspace.project1.yaml", []byte(""), 0644)
	afero.WriteFile(fs, ".workspace.project2.yaml", []byte(""), 0644)
	afero.WriteFile(fs, "workspace.project3.yaml", []byte(""), 0644)

	fs.Mkdir(".workspace.project2dir.yaml", 0755)
	afero.WriteFile(fs, ".workspace.yaml", []byte(""), 0644)

	ws, err := project.enumWorkspaceFiles()
	assert.Contains(t, ws, "project1")
	assert.Equal(t, ws["project1"].Name, "project1")
	assert.Contains(t, ws, "project2")
	assert.Equal(t, ws["project2"].Name, "project2")
	assert.NotContains(t, ws, "")
	assert.NotContains(t, ws, "project2dir")
	assert.NotContains(t, ws, "project3")

	assert.NoError(t, err)
}

func (s *ProjectTestSuite) TestEnumWorkspacesInAGivenProject() {
	t := s.T()
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

func (s *ProjectTestSuite) TestEnumInstancesNoFilesNoInstances() {
	project, _ := NewProject(s.Srv, s.Fs, "/")
	s.Srv.On("GetWorkspaces", mock.Anything).Return(map[string]server.WorkspaceProps{}, nil)

	result, err := project.EnumWorkspaces()

	assert.Empty(s.T(), result)
	assert.NoError(s.T(), err)
}

func (s *ProjectTestSuite) TestEnumInstancesErrorFromServer() {
	project, _ := NewProject(s.Srv, s.Fs, "/")
	s.Srv.On("GetWorkspaces", mock.Anything).Return(nil, api.StatusErrorf(http.StatusNotFound, ""))

	result, err := project.EnumWorkspaces()

	assert.Nil(s.T(), result)
	assert.Error(s.T(), err)
}

func (s *ProjectTestSuite) TestEnumInstancesErrorReadingProjectDirectory() {
	project, _ := NewProject(s.Srv, s.Fs, "/")
	s.Fs.RemoveAll("/")

	result, err := project.EnumWorkspaces()

	assert.Nil(s.T(), result)
	assert.Error(s.T(), err)
}

func (s *ProjectTestSuite) TestEnumInstancesFilesOnly() {
	project, _ := NewProject(s.Srv, s.Fs, "/")
	afero.WriteFile(s.Fs, ".workspace.project1.yaml", []byte(""), 0644)
	s.Srv.On("GetWorkspaces", mock.Anything).Return(map[string]server.WorkspaceProps{}, nil)

	result, err := project.EnumWorkspaces()
	assert.NoError(s.T(), err)
	assert.Contains(s.T(), result, "project1")
	assert.Equal(s.T(), util.Inactive, result["project1"].State)
	assert.Equal(s.T(), "project1", result["project1"].Name)
}

func (s *ProjectTestSuite) TestEnumInstancesInstancesOnly() {
	project, _ := NewProject(s.Srv, s.Fs, "/")
	instances := map[string]server.WorkspaceProps{
		"instance1": {
			Name:  "instance1",
			State: util.Ready,
		},
	}
	s.Srv.On("GetWorkspaces", mock.Anything).Return(instances, nil)

	result, err := project.EnumWorkspaces()
	assert.NoError(s.T(), err)
	assert.Contains(s.T(), result, "instance1")
	assert.Equal(s.T(), util.Orphaned, result["instance1"].State)
	assert.Equal(s.T(), "instance1", result["instance1"].Name)
}

func (s *ProjectTestSuite) TestEnumInstancesSomeOrphanedInstances() {
	project, _ := NewProject(s.Srv, s.Fs, "/")
	afero.WriteFile(s.Fs, ".workspace.project1.yaml", []byte(""), 0644)
	instances := map[string]server.WorkspaceProps{
		"instance1": {
			Name:  "instance1",
			State: util.Ready,
		},
		"project1": {
			Name:  "instance1",
			State: util.Ready,
		},
	}
	s.Srv.On("GetWorkspaces", mock.Anything).Return(instances, nil)

	result, err := project.EnumWorkspaces()
	assert.NoError(s.T(), err)
	assert.Contains(s.T(), result, "instance1")
	assert.Equal(s.T(), util.Orphaned, result["instance1"].State)
	assert.Equal(s.T(), "instance1", result["instance1"].Name)

	assert.Contains(s.T(), result, "project1")
	assert.Equal(s.T(), util.Ready, result["project1"].State)
	assert.Equal(s.T(), "project1", result["project1"].Name)
	assert.Len(s.T(), result, 2)
}
