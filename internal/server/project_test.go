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
	InstMock    MockLxdInstanceServer
	projectName string
}

func (suite *ProjectTestSuite) SetupTest() {
	suite.InstMock = MockLxdInstanceServer{}
}

func TestRunLxdProjectTests(t *testing.T) {
	suite.Run(t, &ProjectTestSuite{})
}

func (suite *ProjectTestSuite) TestLxdProjectDoesNotExist() {
	projectName, _ := GetLXDProjectName()
	suite.InstMock.On("GetProject", projectName).Return((*api.Project)(nil), "", api.StatusErrorf(http.StatusNotFound, "Not found"))
	suite.InstMock.On("CreateProject", mock.Anything).Return(nil)

	err := initProject(&suite.InstMock)
	suite.InstMock.AssertExpectations(suite.T())
	assert.NoError(suite.T(), err)
}

func (suite *ProjectTestSuite) TestLxdProjectIsNotAvail() {
	// Make sure we do not attempt to create a project if a server connectivity issue is experienced (e.g. LXD is down)
	var notAvail = api.StatusErrorf(http.StatusBadGateway, "Not found")
	projectName, _ := GetLXDProjectName()

	suite.InstMock.On("GetProject", projectName).Return((*api.Project)(nil), "", notAvail)

	err := initProject(&suite.InstMock)
	suite.InstMock.AssertExpectations(suite.T())
	assert.Error(suite.T(), err, notAvail)
}
