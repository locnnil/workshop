package server

import (
	"net/http"
	"testing"

	"github.com/lxc/lxd/shared/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ProjectTestSuite struct {
	suite.Suite
	InstMock MockLxdInstanceServer
}

func (suite *ProjectTestSuite) SetupTest() {
	suite.InstMock = MockLxdInstanceServer{}
}

func TestRunLxdProjectTests(t *testing.T) {
	suite.Run(t, &ProjectTestSuite{})
}

func (suite *LaunchTestSuite) TestLxdProjectDoesNotExist() {
	suite.InstMock.On("GetProject", SDK_PROJECT_NAME).Return((*api.Project)(nil), "", api.StatusErrorf(http.StatusNotFound, "Not found"))
	suite.InstMock.On("CreateProject", mock.Anything).Return(nil)

	err := initProject(&suite.InstMock)
	suite.InstMock.AssertExpectations(suite.T())
	assert.NoError(suite.T(), err)
}

func (suite *LaunchTestSuite) TestLxdProjectIsNotAvail() {
	// Make sure we do not attempt to create a project if a server connectivity issue is experienced (e.g. LXD is down)
	var notAvail = api.StatusErrorf(http.StatusBadGateway, "Not found")
	suite.InstMock.On("GetProject", SDK_PROJECT_NAME).Return((*api.Project)(nil), "", notAvail)

	err := initProject(&suite.InstMock)
	suite.InstMock.AssertExpectations(suite.T())
	assert.Error(suite.T(), err, notAvail)
}
