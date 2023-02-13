package workspace

import (
	"io/fs"
	"path/filepath"
	"testing"

	util "github.com/canonical/workspace/internal"
	store "github.com/canonical/workspace/internal/fakestore"
	"github.com/canonical/workspace/internal/mocks"
	"github.com/canonical/workspace/internal/server"
	"github.com/lxc/lxd/shared/api"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type LaunchTestSuite struct {
	suite.Suite
	Fs    afero.Fs
	Srv   *mocks.MockWorkspaceServer
	Store *mocks.MockStoreClient
}

func (s *LaunchTestSuite) SetupTest() {
	s.Fs = afero.NewMemMapFs()
	s.Srv = mocks.NewMockWorkspaceServer(s.T())
	s.Store = mocks.NewMockStoreClient(s.T())
	s.Fs.MkdirAll(util.DataDir, 0700)
	s.Fs.MkdirAll(util.SdksDir, 0700)
}

func createTestWorkspaceFile(fs afero.Fs, filename string, data []byte) fs.FileInfo {
	dir := filepath.Dir(filename)
	fs.MkdirAll(dir, 0644)
	afero.WriteFile(fs, filename, []byte(data), 0644)
	file, _ := fs.Stat(filename)
	return file
}

func (s *LaunchTestSuite) TestWorkspaceLaunchFailed() {
	var ws = &WorkspaceInstance{Name: "noname", Base: "ubuntu@20.04", server: s.Srv}

	s.Srv.On("LaunchWorkspaceInstance", "noname", "ubuntu@20.04").Return(api.StatusErrorf(404, "Not found"))
	ws.Launch(s.Store)
	s.Srv.AssertExpectations(s.T())
}

func (s *LaunchTestSuite) TestWorkspaceLaunchEmpty() {
	var data = []byte(`name: translation
base: ubuntu@20.04`)
	var name = "translation"
	var wsfile = WorkspaceFile{Name: name, Project: "/home/user/project",
		File: createTestWorkspaceFile(s.Fs, "/home/user/project/.workspace.translation.yaml", data)}

	var project = server.WorkspaceDevice{
		Name:       "workspace.project",
		Properties: map[string]string{"type": "disk", "source": wsfile.Project, "path": filepath.Join("/project")},
	}

	s.Srv.
		On("LaunchWorkspaceInstance", name, "ubuntu@20.04").Return(nil).
		On("AddWorkspaceDevice", name, project).Return(nil).
		On("SetWorkspaceState", name, "start").Return(nil)

	ws, err := NewWorkspace(s.Srv, s.Fs, wsfile)
	assert.ErrorIs(s.T(), err, nil)

	err = ws.Launch(s.Store)
	assert.ErrorIs(s.T(), err, nil)
	s.Srv.AssertExpectations(s.T())
}

func (s *LaunchTestSuite) TestWorkspaceLaunchWithAnSDK() {
	var data = []byte(`name: translation
base: ubuntu@20.04
sdks:
  huggingface:
    channel: latest/stable`)

	name, filename := "translation", "huggingface_19.sdk"

	var blob = store.SDKFile{
		Name:     "huggingface",
		Filename: filepath.Join(util.SdksDir, filename),
		Revision: 19,
	}

	device := server.WorkspaceDevice{
		Name:       blob.Name,
		Properties: map[string]string{"type": "disk", "source": blob.Filename, "path": filepath.Join("/root", filename)},
	}

	// Make the exec return immediately
	done := make(chan bool)
	close(done)

	s.Srv.
		On("LaunchWorkspaceInstance", name, "ubuntu@20.04").Return(nil).
		On("SetWorkspaceState", name, "start").Return(nil).
		On("AddWorkspaceDevice", name, mock.Anything).Return(nil).
		On("AddWorkspaceDevice", name, device).Return(nil).
		On("Exec", name, "root", []string{"tar",
			"--extract",
			"--file",
			filepath.Join("/root", filename),
			"--one-top-level=" + filepath.Join(util.WorkspaceSdksDir, blob.Name),
			"--no-same-owner"}).Return(done, nil).
		On("RemoveWorkspaceDevice", name, blob.Name).Return(nil)

	s.Store.On("FetchSDK", blob.Name, "latest/stable", util.SdksDir).Return(blob, nil)

	ws, err := NewWorkspace(s.Srv, s.Fs,
		WorkspaceFile{
			Name: name,
			File: createTestWorkspaceFile(s.Fs, ".workspace.translation.yaml", data)})
	assert.ErrorIs(s.T(), err, nil)

	err = ws.Launch(s.Store)
	assert.ErrorIs(s.T(), err, nil)
	s.Srv.AssertExpectations(s.T())
}

func TestRunLaunchTests(t *testing.T) {
	suite.Run(t, &LaunchTestSuite{})
}
