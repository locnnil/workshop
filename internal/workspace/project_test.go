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
	afero.WriteFile(s.Fs, ".workspace.project1.yaml", []byte(""), 0644)
	afero.WriteFile(s.Fs, ".workspace.project2.yaml", []byte(""), 0644)
	afero.WriteFile(s.Fs, "workspace.project3.yaml", []byte(""), 0644)
	s.Fs.Mkdir(".workspace.project2dir.yaml", 0755)
	afero.WriteFile(s.Fs, ".workspace.yaml", []byte(""), 0644)
	afero.WriteFile(s.Fs, ".workspace.lock", []byte(""), 0644)

	project := &Project{fs: s.Fs}
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

	project := &Project{fs: s.Fs, Path: "/tmp/experiments"}

	afero.WriteFile(fs, filepath.Join(project.GetProjectDirectory(), ".workspace.project1.yaml"), []byte(""), 0755)

	/* No relative paths are allowed */
	project, err := NewProject(nil, fs, "../tmp/experiments")
	assert.Nil(t, project)
	assert.Error(t, ErrNoRelativePathsAllowed, err)

	/* Could not read the project directory */
	project, err = NewProject(nil, fs, "/invalid&")
	assert.Nil(t, project)
	assert.Error(t, err)
}

func (s *ProjectTestSuite) TestEnumInstancesNoFilesNoInstances() {
	s.Srv.On("GetWorkspaces", mock.Anything).Twice().Return(map[string]*server.WorkspaceProps{}, nil)
	project, _ := NewProject(s.Srv, s.Fs, "/")

	result, err := project.EnumWorkspaces()

	assert.Empty(s.T(), result)
	assert.NoError(s.T(), err)
}

func (s *ProjectTestSuite) TestEnumInstancesErrorFromServer() {
	s.Srv.
		On("GetWorkspaces", mock.Anything).Return(nil, api.StatusErrorf(http.StatusNotFound, ""))

	project, err := NewProject(s.Srv, s.Fs, "/")

	assert.Nil(s.T(), project)
	assert.Error(s.T(), err)
}

func (s *ProjectTestSuite) TestEnumInstancesErrorReadingProjectDirectory() {
	s.Srv.On("GetWorkspaces", mock.Anything).Return(map[string]*server.WorkspaceProps{}, nil)
	project, _ := NewProject(s.Srv, s.Fs, "/")
	s.Fs.RemoveAll("/")

	result, err := project.enumWorkspaceFiles()

	assert.Nil(s.T(), result)
	assert.Error(s.T(), err)
}

func (s *ProjectTestSuite) TestEnumInstancesFilesOnly() {
	s.Srv.On("GetWorkspaces", mock.Anything).Twice().Return(map[string]*server.WorkspaceProps{}, nil)
	project, _ := NewProject(s.Srv, s.Fs, "/")
	afero.WriteFile(s.Fs, ".workspace.project1.yaml", []byte(""), 0644)

	result, err := project.EnumWorkspaces()
	assert.NoError(s.T(), err)
	assert.Contains(s.T(), result, "project1")
	assert.Equal(s.T(), util.Inactive, result["project1"].State)
	assert.Equal(s.T(), "project1", result["project1"].Name)
}

func (s *ProjectTestSuite) TestEnumInstancesInstancesOnly() {
	instances := map[string]*server.WorkspaceProps{
		"instance1": {
			Name:  "instance1",
			State: util.Ready,
		},
	}
	s.Srv.On("GetWorkspaces", mock.Anything).Twice().Return(instances, nil)
	project, _ := NewProject(s.Srv, s.Fs, "/")

	result, err := project.EnumWorkspaces()
	assert.NoError(s.T(), err)
	assert.Contains(s.T(), result, "instance1")
	assert.Equal(s.T(), util.Error, result["instance1"].State)
	assert.Equal(s.T(), "instance1", result["instance1"].Name)
}

func (s *ProjectTestSuite) TestEnumInstancesSomeOrphanedInstances() {
	instances := map[string]*server.WorkspaceProps{
		"instance1": {
			Name:  "instance1",
			State: util.Ready,
		},
		"project1": {
			Name:  "instance1",
			State: util.Ready,
		},
	}
	s.Srv.On("GetWorkspaces", mock.Anything).Twice().Return(instances, nil)
	project, _ := NewProject(s.Srv, s.Fs, "/")
	afero.WriteFile(s.Fs, ".workspace.project1.yaml", []byte(""), 0644)

	result, err := project.EnumWorkspaces()
	assert.NoError(s.T(), err)
	assert.Contains(s.T(), result, "instance1")
	assert.Equal(s.T(), util.Error, result["instance1"].State)
	assert.Equal(s.T(), "instance1", result["instance1"].Name)

	assert.Contains(s.T(), result, "project1")
	assert.Equal(s.T(), util.Ready, result["project1"].State)
	assert.Equal(s.T(), "project1", result["project1"].Name)
	assert.Len(s.T(), result, 2)
}

func (s *ProjectTestSuite) TestReadProject() {
	project := Project{fs: s.Fs}

	err := project.ReadProject("/project")
	assert.Error(s.T(), err)

	afero.WriteFile(s.Fs, "/.workspace.lock", []byte(""), 0644)

	err = project.ReadProject("/")
	assert.NoError(s.T(), err)
}
