package workspace

import (
	"path/filepath"
	"testing"

	util "github.com/canonical/workspace/internal"
	store "github.com/canonical/workspace/internal/fakestore"
	"github.com/canonical/workspace/internal/server"
	"github.com/lxc/lxd/shared/api"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type LaunchTestSuite struct {
	suite.Suite
	Fs    afero.Fs
	Srv   MockServer
	Store StoreClientMock
}

func (suite *LaunchTestSuite) SetupTest() {
	suite.Fs = afero.NewMemMapFs()
	suite.Srv = MockServer{}
	suite.Fs.MkdirAll(util.DataDir, 0700)
	suite.Fs.MkdirAll(util.SdksDir, 0700)
}

func (suite *LaunchTestSuite) TestWorkspaceLaunchWithNoSDKs() {
	server := suite.Srv
	name := "translation"
	filename := ".workspace.translation.yaml"
	afero.WriteFile(suite.Fs, filename,
		[]byte(`name: translation
base: ubuntu@20.04`), 0644)
	file, _ := suite.Fs.Stat(filename)

	mockCall := server.On("LaunchWorkspaceInstance", name, "ubuntu@20.04").Return(nil)
	server.On("SetWorkspaceState", name, "start").Return(nil)

	ws, err := NewWorkspace(&server, suite.Fs, WorkspaceFile{Name: name, File: file})
	assert.ErrorIs(suite.T(), err, nil)

	err = ws.Launch(&suite.Store)
	assert.ErrorIs(suite.T(), err, nil)
	server.AssertExpectations(suite.T())
	mockCall.Unset()

	server.On("LaunchWorkspaceInstance", name, "ubuntu@20.04").Return(api.StatusErrorf(404, "Not found"))
	err = ws.Launch(&suite.Store)
	server.AssertExpectations(suite.T())
	assert.True(suite.T(), api.StatusErrorCheck(err, 404))
}

func (suite *LaunchTestSuite) TestWorkspaceLaunchWithAnSDK() {
	data := `name: translation
base: ubuntu@20.04
sdks:
  huggingface:
    channel: latest/stable`
	name, sdkname, filename := "translation", "huggingface", "huggingface_19.sdk"
	afero.WriteFile(suite.Fs, ".workspace.translation.yaml",
		[]byte(data), 0644)
	wsfile, _ := suite.Fs.Stat(".workspace.translation.yaml")

	srv := suite.Srv
	sdkFile := filepath.Join(util.SdksDir, filename)
	devices := server.WorkspaceDevices{
		sdkname: {"type": "disk", "source": sdkFile, "path": filepath.Join("/root", filename)},
	}

	srv.On("LaunchWorkspaceInstance", name, "ubuntu@20.04").Return(nil)
	srv.On("SetWorkspaceState", name, "start").Return(nil)
	srv.On("GetWorkspaceDevices").Return(make(server.WorkspaceDevices), nil)
	srv.On("UpdateWorkspaceDevices", devices).Return(nil)
	srv.On("Exec", name, "root", []string{"tar",
		"--extract",
		"--file",
		filepath.Join("/root", filename),
		"--one-top-level=" + filepath.Join(util.WorkspaceSdksDir, sdkname),
		"--no-same-owner",
	}).Return(nil)
	srv.On("UpdateWorkspaceDevices", make(server.WorkspaceDevices)).Return(nil)

	suite.Store.On("FetchSDK", sdkname, "latest/stable", util.SdksDir).Return(store.SDKFile{
		Filename: sdkFile,
		Revision: 19,
	}, nil)

	ws, err := NewWorkspace(&srv, suite.Fs, WorkspaceFile{Name: name, File: wsfile})
	assert.ErrorIs(suite.T(), err, nil)

	err = ws.Launch(&suite.Store)
	assert.ErrorIs(suite.T(), err, nil)
	srv.AssertExpectations(suite.T())
}

func TestRunLaunchTests(t *testing.T) {
	suite.Run(t, &LaunchTestSuite{})
}
