package workspace

import (
	"os"
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
	Store store.StoreClient
}

func (suite *LaunchTestSuite) SetupTest() {
	suite.Fs = afero.NewMemMapFs()
	suite.Srv = MockServer{}
	suite.Fs.MkdirAll(util.DataDir, 0700)
	suite.Fs.MkdirAll(util.SdksDir, 0700)
	suite.Store, _ = store.NewStoreClient(suite.Fs)
}

func (suite *LaunchTestSuite) TestWorkspaceLaunchFromFile() {
	_, err := NewWorkspace(&suite.Srv, suite.Fs, "not_found.yaml")
	assert.True(suite.T(), os.IsNotExist(err))
}

func (suite *LaunchTestSuite) TestWorkspaceLaunchWithNoSDKs() {
	afero.WriteFile(suite.Fs, ".workspace.translation.yaml",
		[]byte(`name: translation
base: ubuntu@20.04`), 0644)
	server := suite.Srv

	mockCall := server.On("LaunchWorkspaceInstance", "translation", "ubuntu@20.04").Return(nil)
	server.On("SetWorkspaceState", "translation", "start").Return(nil)

	ws, err := NewWorkspace(&server, suite.Fs, ".workspace.translation.yaml")
	assert.ErrorIs(suite.T(), err, nil)

	err = ws.Launch(suite.Store)
	assert.ErrorIs(suite.T(), err, nil)
	server.AssertExpectations(suite.T())
	mockCall.Unset()

	server.On("LaunchWorkspaceInstance", "translation", "ubuntu@20.04").Return(api.StatusErrorf(404, "Not found"))
	err = ws.Launch(suite.Store)
	server.AssertExpectations(suite.T())
	assert.True(suite.T(), api.StatusErrorCheck(err, 404))
}

func (suite *LaunchTestSuite) TestWorkspaceLaunchWithAnSDK() {
	data := `name: translation
base: ubuntu@20.04
sdks:
  huggingface:
    channel: latest/stable`
	afero.WriteFile(suite.Fs, ".workspace.translation.yaml",
		[]byte(data), 0644)

	srv := suite.Srv
	sdkFile := filepath.Join(util.SdksDir, "huggingface_latest_stable.sdk")
	devices := server.WorkspaceDevices{
		"huggingface": {"type": "disk", "source": sdkFile, "path": "/root"},
	}

	srv.On("LaunchWorkspaceInstance", "translation", "ubuntu@20.04").Return(nil)
	srv.On("SetWorkspaceState", "translation", "start").Return(nil)
	srv.On("GetWorkspaceDevices").Return(make(server.WorkspaceDevices), nil)
	srv.On("UpdateWorkspaceDevices", devices).Return(nil)
	srv.On("UpdateWorkspaceDevices", make(server.WorkspaceDevices)).Return(nil)

	ws, err := NewWorkspace(&srv, suite.Fs, ".workspace.translation.yaml")
	assert.ErrorIs(suite.T(), err, nil)

	err = ws.Launch(suite.Store)
	assert.ErrorIs(suite.T(), err, nil)
	srv.AssertExpectations(suite.T())

	exists, _ := afero.Exists(suite.Fs, sdkFile)
	assert.True(suite.T(), exists)

}

func TestRunLaunchTests(t *testing.T) {
	suite.Run(t, &LaunchTestSuite{})
}
