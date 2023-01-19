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

func (s *LaunchTestSuite) TestLaunchSucceededStartFailed() {
	var ws = &WorkspaceInstance{Name: "noname", Base: "ubuntu@20.04", server: s.Srv}

	s.Srv.
		On("LaunchWorkspaceInstance", ws.Name, "ubuntu@20.04").Return(nil).
		On("SetWorkspaceState", ws.Name, "start").Return(api.StatusErrorf(int(api.Failure), ""))
	ws.Launch(s.Store)
	s.Srv.AssertExpectations(s.T())
}

func (s *LaunchTestSuite) TestWorkspaceLaunchWithNoSDKs() {
	var data = []byte(`name: translation
base: ubuntu@20.04`)
	name, filename := "translation", ".workspace.translation.yaml"
	file := createTestWorkspaceFile(s.Fs, filename, data)

	s.Srv.
		On("LaunchWorkspaceInstance", name, "ubuntu@20.04").Return(nil).
		On("SetWorkspaceState", name, "start").Return(nil)

	ws, err := NewWorkspace(s.Srv, s.Fs, WorkspaceFile{Name: name, File: file})
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

	name, sdkname, filename := "translation", "huggingface", "huggingface_19.sdk"
	wsfile := createTestWorkspaceFile(s.Fs, ".workspace.translation.yaml", data)

	sdkFile := filepath.Join(util.SdksDir, filename)
	devices := server.WorkspaceDevices{
		sdkname: {"type": "disk", "source": sdkFile, "path": filepath.Join("/root", filename)},
	}

	// Make the exec return immediately
	done := make(chan bool)
	close(done)

	s.Srv.
		On("LaunchWorkspaceInstance", name, "ubuntu@20.04").Return(nil).
		On("SetWorkspaceState", name, "start").Return(nil).
		On("GetWorkspaceDevices", name).Return(make(server.WorkspaceDevices), nil).
		On("UpdateWorkspaceDevices", name, devices).Return(nil).
		On("Exec", name, "root", []string{"tar",
			"--extract",
			"--file",
			filepath.Join("/root", filename),
			"--one-top-level=" + filepath.Join(util.WorkspaceSdksDir, sdkname),
			"--no-same-owner"}).Return(done, nil).
		On("UpdateWorkspaceDevices", name, make(server.WorkspaceDevices)).Return(nil)

	s.Store.On("FetchSDK", sdkname, "latest/stable", util.SdksDir).Return(store.SDKFile{
		Filename: sdkFile,
		Revision: 19,
	}, nil)

	ws, err := NewWorkspace(s.Srv, s.Fs, WorkspaceFile{Name: name, File: wsfile})
	assert.ErrorIs(s.T(), err, nil)

	err = ws.Launch(s.Store)
	assert.ErrorIs(s.T(), err, nil)
	s.Srv.AssertExpectations(s.T())
}

func TestRunLaunchTests(t *testing.T) {
	suite.Run(t, &LaunchTestSuite{})
}
