package server

import (
	"net/http"
	"testing"

	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type LaunchTestSuite struct {
	suite.Suite
	Fs       afero.Fs
	Srv      LxdServer
	InstMock MockLxdInstanceServer
	ImgMock  MockLxdImageServer
}

func (suite *LaunchTestSuite) SetupTest() {
	suite.Fs = afero.NewMemMapFs()
	suite.Srv = LxdServer{filesystem: suite.Fs}
	suite.InstMock = MockLxdInstanceServer{}
	suite.Srv.InstanceServer = &suite.InstMock
	suite.ImgMock = MockLxdImageServer{}
}

func (suite *LaunchTestSuite) TestWorkspaceLxdLaunchLocalImageExists() {
	suite.InstMock.On("GetImage", "ubuntu:20.04").Return((*api.Image)(nil), "",
		nil)

	err := suite.Srv.LaunchWorkspaceInstance("test", "ubuntu@20.04")
	assert.Equal(suite.T(), err, nil)
	suite.InstMock.AssertExpectations(suite.T())
}

func (suite *LaunchTestSuite) TestWorkspaceLxdLaunchNoLocalImage() {

	ConnectSimpleStreams = func(url string, args *lxd.ConnectionArgs) (lxd.ImageServer, error) {
		return &suite.ImgMock, nil
	}
	//var image api.Image
	var imageAlias api.ImageAliasesEntry
	var err error
	var notFoundError = api.StatusErrorf(http.StatusNotFound, "Not found")

	imageAlias.Target = "2DFSJF359FNS"

	suite.InstMock.On("GetImage", "ubuntu:20.04").Return((*api.Image)(nil), "",
		notFoundError)
	suite.ImgMock.On("GetImageAlias", "20.04").Return(&imageAlias, "",
		nil)
	suite.ImgMock.On("GetImage", imageAlias.Target).Return((*api.Image)(nil), "",
		notFoundError)

	err = suite.Srv.LaunchWorkspaceInstance("test", "ubuntu@20.04")

	assert.Equal(suite.T(), err, notFoundError)
	suite.InstMock.AssertExpectations(suite.T())
}

func TestRunLxdServerTests(t *testing.T) {
	suite.Run(t, &LaunchTestSuite{})
}
