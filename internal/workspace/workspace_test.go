package workspace

import (
	"os"
	"testing"

	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type LaunchTestSuite struct {
	suite.Suite
	Fs  afero.Fs
	Srv MockServer
}

type MockServer struct {
	lxd.InstanceServer
	mock.Mock
}

func (s *MockServer) LaunchWorkspaceInstance(name, base string) error {
	args := s.Called(name, base)
	return args.Error(0)
}

func (suite *LaunchTestSuite) SetupTest() {
	suite.Fs = afero.NewMemMapFs()
	suite.Srv = MockServer{}
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

func TestRunLaunchTests(t *testing.T) {
	suite.Run(t, &LaunchTestSuite{})
}
