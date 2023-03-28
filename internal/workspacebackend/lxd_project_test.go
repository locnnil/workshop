package workspacebackend_test

import (
	"net/http"
	"testing"

	"github.com/canonical/workspace/internal/mocks"
	"github.com/canonical/workspace/internal/workspacebackend"
	"github.com/lxc/lxd/shared/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ProjectTestSuite struct {
	suite.Suite
	InstMock *mocks.MockInstanceServer
}

func (s *ProjectTestSuite) SetupTest() {
	s.InstMock = mocks.NewMockInstanceServer(s.T())
}

func TestRunLxdProjectTests(t *testing.T) {
	suite.Run(t, &ProjectTestSuite{})
}

func (s *ProjectTestSuite) TestLxdProjectDoesNotExist() {
	projectName, _ := workspacebackend.GetLXDProjectName()
	s.InstMock.On("GetProject", projectName).Return((*api.Project)(nil), "", ApiErrNotFound)
	s.InstMock.On("CreateProject", mock.Anything).Return(nil)

	err := workspacebackend.InitProject(s.InstMock)
	s.InstMock.AssertExpectations(s.T())
	assert.NoError(s.T(), err)
}

func (s *ProjectTestSuite) TestLxdProjectExistsORNotAvail() {
	projectName, _ := workspacebackend.GetLXDProjectName()
	// Make sure we do not attempt to create a project if anything but 404 is returned by LXD
	var errors = []error{api.StatusErrorf(http.StatusBadGateway, ""), nil}

	for _, i := range errors {
		c := s.InstMock.On("GetProject", projectName).Return((*api.Project)(nil), "", i)
		err := workspacebackend.InitProject(s.InstMock)
		s.InstMock.AssertExpectations(s.T())
		assert.Equal(s.T(), err, i)
		c.Unset()
	}
}
