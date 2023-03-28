package projectstate

import (
	"math/rand"
	"testing"

	util "github.com/canonical/workspace/internal"
	"github.com/canonical/workspace/internal/mocks"
	"github.com/canonical/workspace/internal/workspacebackend"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"golang.org/x/exp/slices"
)

type ProjectTestSuite struct {
	suite.Suite
	Fs      afero.Fs
	Backend *mocks.MockWorkspaceBackend
}

func TestRunProjectTests(t *testing.T) {
	suite.Run(t, &ProjectTestSuite{})
}

func (s *ProjectTestSuite) SetupTest() {
	s.Fs = afero.NewMemMapFs()
	s.Backend = mocks.NewMockWorkspaceBackend(s.T())
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
	ws, err := project.EnumWorkspaceFiles()

	assert.Len(t, ws, 2)
	assert.Equal(t, ws[0].Name, "project1")
	assert.Equal(t, ws[1].Name, "project2")
	assert.NotContains(t, ws, "")
	assert.NotContains(t, ws, "project2dir")
	assert.NotContains(t, ws, "project3")

	assert.NoError(t, err)
}

func (s *ProjectTestSuite) TestNewProject() {
	t := s.T()

	project, err := NewProject(s.Backend, s.Fs, "/")
	assert.Equal(t, "52fdfc07", project.ProjectId())
	assert.NoError(t, err)
	assert.Equal(t, "/", project.ProjectDirectory())

	project, err = NewProject(s.Backend, s.Fs, "/doesnotexist")
	assert.Nil(t, project)
	assert.ErrorIs(t, err, afero.ErrFileNotFound)
}

func (s *ProjectTestSuite) TestLoadProject() {
	t := s.T()
	fs := afero.NewOsFs()
	fs.MkdirAll("/tmp/experiments", 0755)
	defer fs.RemoveAll("/tmp/experiments")

	/* No relative paths are allowed */
	project, err := LoadProject(nil, fs, "../tmp/experiments")
	assert.Nil(t, project)
	assert.Error(t, util.ErrNoRelativePathsAllowed, err)

	/* Could not read the project directory */
	project, err = LoadProject(nil, fs, "/invalid&")
	assert.Nil(t, project)
	assert.Error(t, err)

	/* Project exists, no workspace instances */
	h := s.Backend.On("GetWorkspacesByConfig", mock.Anything).Once().Return([]*workspacebackend.WorkspaceProps{}, nil)
	afero.WriteFile(s.Fs, "/.workspace.lock", []byte("PROJECTID"), 0644)
	project, err = LoadProject(s.Backend, s.Fs, "/")
	assert.NotNil(t, project)
	assert.Equal(t, "/", project.ProjectDirectory())
	assert.NoError(t, err)

	/* Project exists, some workspace instances running */
	instances := []*workspacebackend.WorkspaceProps{
		{
			Name: "instance1",
			Devices: map[string]map[string]string{"workspace.project": {
				"type": "disk", "source": "/", "path": "/project"}},
		},
	}
	instances[0].SetState(util.Ready, util.None)
	h.Unset()
	s.Backend.On("GetWorkspacesByConfig", mock.Anything).Once().Return(instances, nil)
	project, err = LoadProject(s.Backend, s.Fs, "/")
	assert.NotNil(t, project)
	assert.Equal(t, "/", project.ProjectDirectory())
	assert.NoError(t, err)
}

func (s *ProjectTestSuite) TestEnumWorkspacesNoFilesNoInstances() {
	s.Backend.On("GetWorkspacesByConfig", mock.Anything).Once().Return([]*workspacebackend.WorkspaceProps{}, nil)
	project := Project{fs: s.Fs, backend: s.Backend, path: "/"}

	result, err := project.RetrieveWorkspaces()

	assert.Empty(s.T(), result)
	assert.NoError(s.T(), err)
}

func (s *ProjectTestSuite) TestEnumFilesErrorReadingProjectDirectory() {
	project := Project{fs: s.Fs, backend: s.Backend, path: "/"}
	s.Fs.RemoveAll("/")

	result, err := project.EnumWorkspaceFiles()

	assert.Nil(s.T(), result)
	assert.Error(s.T(), err)
}

func (s *ProjectTestSuite) TestEnumWorkspacesFilesOnly() {
	s.Backend.On("GetWorkspacesByConfig", mock.Anything).Once().Return([]*workspacebackend.WorkspaceProps{}, nil)
	project := Project{fs: s.Fs, backend: s.Backend, path: "/"}
	afero.WriteFile(s.Fs, ".workspace.project1.yaml", []byte(""), 0644)
	afero.WriteFile(s.Fs, ".workspace.lock", []byte(""), 0644)

	result, err := project.RetrieveWorkspaces()
	/* Make sure files without instances are not returned */
	assert.NoError(s.T(), err)
	assert.Len(s.T(), result, 1)
	assert.Equal(s.T(), result[0].State(), util.Inactive)
	assert.Equal(s.T(), util.None, result[0].Reason())
}

func (s *ProjectTestSuite) TestEnumWorkspacesInstancesOnly() {
	instances := []*workspacebackend.WorkspaceProps{
		{
			Name: "instance1",
		},
	}
	instances[0].SetState(util.Ready, util.None)

	s.Backend.On("GetWorkspacesByConfig", mock.Anything).Once().Return(instances, nil)
	project := Project{fs: s.Fs, backend: s.Backend, path: "/"}

	result, err := project.RetrieveWorkspaces()
	assert.NoError(s.T(), err)
	assert.Len(s.T(), result, 1)
	/* the workspace does not have a corresponding file, hence, an error state */
	assert.Equal(s.T(), util.Error, result[0].State())
	assert.Equal(s.T(), util.MissingFile, result[0].Reason())
	assert.Equal(s.T(), "instance1", result[0].Name)
}

func (s *ProjectTestSuite) TestEnumWorkspacesSomeOrphanedInstances() {
	instances := []*workspacebackend.WorkspaceProps{
		{
			Name: "instance1",
		},
		{
			Name: "project1",
		},
	}
	instances[0].SetState(util.Ready, util.None)
	instances[1].SetState(util.Ready, util.None)

	s.Backend.On("GetWorkspacesByConfig", mock.Anything).Once().Return(instances, nil)
	project := Project{fs: s.Fs, backend: s.Backend, path: "/"}
	afero.WriteFile(s.Fs, ".workspace.project1.yaml", []byte(""), 0644)

	result, err := project.RetrieveWorkspaces()
	// Make sure the order is always predictable
	slices.SortFunc(result, func(i, j *workspacebackend.WorkspaceProps) bool { return i.Name > j.Name })
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), util.Error, result[1].State())
	assert.Equal(s.T(), util.MissingFile, result[1].Reason())
	assert.Equal(s.T(), "instance1", result[1].Name)

	assert.Equal(s.T(), util.Ready, result[0].State())
	assert.Equal(s.T(), "project1", result[0].Name)
	assert.Len(s.T(), result, 2)
}

func (s *ProjectTestSuite) TestReadProject() {
	project := Project{fs: s.Fs}

	err := project.ReadProject("/project")
	assert.Error(s.T(), err)

	afero.WriteFile(s.Fs, "/.workspace.lock", []byte("23451S"), 0644)

	err = project.ReadProject("/")
	assert.NoError(s.T(), err)
}
