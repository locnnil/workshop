package workspace

import (
	"math/rand"
	"path/filepath"
	"testing"

	util "github.com/canonical/workspace/internal"
	store "github.com/canonical/workspace/internal/fakestore"
	"github.com/canonical/workspace/internal/mocks"
	srv "github.com/canonical/workspace/internal/server"
	"github.com/lxc/lxd/shared/api"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type LaunchTestSuite struct {
	suite.Suite
	Fs      afero.Fs
	Srv     *mocks.MockWorkspaceServer
	Store   *mocks.MockStoreClient
	Project *Project
}

func (s *LaunchTestSuite) SetupTest() {
	s.Fs = afero.NewMemMapFs()
	s.Srv = mocks.NewMockWorkspaceServer(s.T())
	s.Store = mocks.NewMockStoreClient(s.T())
	s.Project, _ = NewProject(s.Srv, s.Fs, "/")
	s.Fs.MkdirAll(util.DataDir, 0755)
	s.Fs.MkdirAll(util.SdksDir, 0755)
	rand.Seed(1)
}

func (s *LaunchTestSuite) createTestFile(filename string, data []byte) string {
	dir := filepath.Dir(filename)
	s.Fs.MkdirAll(dir, 0644)
	afero.WriteFile(s.Fs, filename, []byte(data), 0644)
	file, _ := s.Fs.Stat(filename)
	return file.Name()
}

func (s *LaunchTestSuite) createTestWorkspace(name string, data []byte) srv.WorkspaceProps {
	return srv.WorkspaceProps{Name: name,
		FileName: s.createTestFile(filepath.Join(s.Project.GetProjectDirectory(), util.ToFileName(name)),
			data)}
}

var dataNoSDK = []byte(`name: translation
base: ubuntu@20.04`)

var dataWithSDK = []byte(`name: translation
base: ubuntu@20.04
sdks:
  huggingface:
    channel: latest/stable`)

func (s *LaunchTestSuite) TestNewWorkspaceEmptyNoProject() {
	var file = s.createTestWorkspace("translation", dataNoSDK)
	ws, err := NewWorkspace(s.Srv, s.Project, s.Fs, file)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "translation", ws.(*WorkspaceInstance).Name)
	assert.Equal(s.T(), "ubuntu@20.04", ws.(*WorkspaceInstance).Base)
	assert.Empty(s.T(), ws.(*WorkspaceInstance).SDKs)
	assert.NotNil(s.T(), ws)
}

func (s *LaunchTestSuite) TestNewWorkspaceEmptyWithProject() {
	var file = s.createTestWorkspace("translation", dataNoSDK)
	ws, err := NewWorkspace(s.Srv, s.Project, s.Fs, file)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "5577006791947779410", ws.(*WorkspaceInstance).project.ProjectId)
	assert.Equal(s.T(), "translation", ws.(*WorkspaceInstance).Name)
	assert.Equal(s.T(), "ubuntu@20.04", ws.(*WorkspaceInstance).Base)
	assert.Empty(s.T(), ws.(*WorkspaceInstance).SDKs)
	assert.NotNil(s.T(), ws)
}

func (s *LaunchTestSuite) TestNewWorkspaceWithSDKsWithProject() {
	var file = s.createTestWorkspace("translation", dataWithSDK)
	ws, err := NewWorkspace(s.Srv, s.Project, s.Fs, file)

	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "5577006791947779410", ws.(*WorkspaceInstance).project.ProjectId)
	assert.Equal(s.T(), "translation", ws.(*WorkspaceInstance).Name)
	assert.Equal(s.T(), "ubuntu@20.04", ws.(*WorkspaceInstance).Base)
	assert.Equal(s.T(), &SDK{Channel: "latest/stable"}, ws.(*WorkspaceInstance).SDKs["huggingface"])
	assert.NotNil(s.T(), ws)
}

func (s *LaunchTestSuite) TestWorkspaceLaunchFailed() {
	var ws = &WorkspaceInstance{Name: "noname", Base: "ubuntu@20.04", server: s.Srv, fs: s.Fs,
		project: &Project{Path: "/project", fs: s.Fs}}

	s.Srv.On("LaunchWorkspaceInstance", "noname", "ubuntu@20.04").Return(api.StatusErrorf(404, "Not found"))
	ws.Launch(s.Store)
	s.Srv.AssertExpectations(s.T())
}

func (s *LaunchTestSuite) TestWorkspaceLaunchEmpty() {
	var name = "translation"
	var file = s.createTestWorkspace(name, dataNoSDK)
	var project = srv.WorkspaceDevice{
		Name:       "workspace.project",
		Properties: map[string]string{"type": "disk", "source": s.Project.GetProjectDirectory(), "path": filepath.Join("/project")},
	}

	s.Srv.
		On("LaunchWorkspaceInstance", name, "ubuntu@20.04").Return(nil).
		On("AddWorkspaceConfig", name, mock.Anything).Return(nil).
		On("AddWorkspaceDevice", name, project).Return(nil).
		On("SetWorkspaceState", name, "start").Return(nil).
		On("AddWorkspaceConfig", name, &srv.WorkspaceConfig{Name: "user.workspace.state", Value: "ready"}).Return(nil)

	ws, err := NewWorkspace(s.Srv, s.Project, s.Fs, file)
	assert.ErrorIs(s.T(), err, nil)

	err = ws.Launch(s.Store)
	assert.ErrorIs(s.T(), err, nil)

	s.Srv.AssertExpectations(s.T())
}

func (s *LaunchTestSuite) TestWorkspaceLaunchWithAnSDK() {
	name, filename := "translation", "huggingface_19.sdk"

	var blob = store.SDKFile{
		Name:     "huggingface",
		Filename: filepath.Join(util.SdksDir, filename),
		Revision: 19,
	}

	device := srv.WorkspaceDevice{
		Name:       blob.Name,
		Properties: map[string]string{"type": "disk", "source": blob.Filename, "path": filepath.Join("/root", filename)},
	}

	// Make the exec return immediately
	done := make(chan bool)
	close(done)

	s.Srv.
		On("LaunchWorkspaceInstance", name, "ubuntu@20.04").Return(nil).
		On("AddWorkspaceConfig", name, mock.Anything).Return(nil).
		On("SetWorkspaceState", name, "start").Return(nil).
		On("AddWorkspaceDevice", name, mock.Anything).Return(nil).
		On("AddWorkspaceDevice", name, device).Return(nil).
		On("Exec", name, "root", []string{"tar",
			"--extract",
			"--file",
			filepath.Join("/root", filename),
			"--one-top-level=" + filepath.Join(util.WorkspaceSdksDir, blob.Name),
			"--no-same-owner"}).Return(done, nil).
		On("RemoveWorkspaceDevice", name, blob.Name).Return(nil).
		On("AddWorkspaceConfig", name, &srv.WorkspaceConfig{Name: "user.workspace.state", Value: "ready"}).Return(nil)

	s.Store.On("FetchSDK", blob.Name, "latest/stable", util.SdksDir).Return(blob, nil)

	ws, err := NewWorkspace(s.Srv, s.Project, s.Fs,
		s.createTestWorkspace(name, dataWithSDK))
	assert.ErrorIs(s.T(), err, nil)

	err = ws.Launch(s.Store)
	assert.ErrorIs(s.T(), err, nil)
	s.Srv.AssertExpectations(s.T())
}

func TestRunLaunchTests(t *testing.T) {
	suite.Run(t, &LaunchTestSuite{})
}
