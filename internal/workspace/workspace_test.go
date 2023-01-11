package workspace

import (
	"os"
	"path/filepath"
	"testing"

	util "github.com/canonical/workspace/internal"
	"github.com/lxc/lxd/shared/api"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type LaunchTestSuite struct {
	suite.Suite
	Fs  afero.Fs
	Srv MockServer
}

func (suite *LaunchTestSuite) SetupTest() {
	suite.Fs = afero.NewMemMapFs()
	suite.Srv = MockServer{}
	suite.Fs.MkdirAll(util.DataDir, 0700)
	suite.Fs.MkdirAll(util.SdksDir, 0700)
}

func (suite *LaunchTestSuite) TestWorkspaceLaunchFromFile() {
	_, err := NewWorkspace(&suite.Srv, suite.Fs, "not_found.yaml")
	assert.True(suite.T(), os.IsNotExist(err))
}

func (suite *LaunchTestSuite) TestWorkspaceLaunch() {
	afero.WriteFile(suite.Fs, ".workspace.translation.yaml",
		[]byte(`name: translation
base: ubuntu@20.04`), 0644)
	server := suite.Srv

	mockCall := server.On("LaunchWorkspaceInstance", "translation", "ubuntu@20.04").Return(nil)
	server.On("SetInstanceState", "translation", "start").Return(nil)

	ws, err := NewWorkspace(&server, suite.Fs, ".workspace.translation.yaml")
	assert.ErrorIs(suite.T(), err, nil)

	err = ws.Launch()
	assert.ErrorIs(suite.T(), err, nil)
	server.AssertExpectations(suite.T())
	mockCall.Unset()

	server.On("LaunchWorkspaceInstance", "translation", "ubuntu@20.04").Return(api.StatusErrorf(404, "Not found"))
	err = ws.Launch()
	server.AssertExpectations(suite.T())
	assert.True(suite.T(), api.StatusErrorCheck(err, 404))
}

func (suite *LaunchTestSuite) TestSDKDownload() {
	data := `name: translation
base: ubuntu@20.04
sdks:
  huggingface:
    channel: latest/stable`
	afero.WriteFile(suite.Fs, ".workspace.translation.yaml",
		[]byte(data), 0644)

	server := suite.Srv

	server.On("LaunchWorkspaceInstance", "translation", "ubuntu@20.04").Return(nil)
	server.On("SetInstanceState", "translation", "start").Return(nil)

	ws, err := NewWorkspace(&server, suite.Fs, ".workspace.translation.yaml")
	assert.ErrorIs(suite.T(), err, nil)

	err = ws.Launch()
	assert.ErrorIs(suite.T(), err, nil)
	server.AssertExpectations(suite.T())

	exists, _ := afero.Exists(suite.Fs, filepath.Join(util.SdksDir, "huggingface_latest_stable.sdk"))
	assert.True(suite.T(), exists)

}

func TestRunLaunchTests(t *testing.T) {
	suite.Run(t, &LaunchTestSuite{})
}
