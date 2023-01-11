package server

import (
	"net/http"
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
	var name, base, fingerprint string = "test", "ubuntu@20.04", "FS34DS"
	var image api.Image
	image.Fingerprint = fingerprint
	var op MockRemoteOperation
	alias := api.ImageAliasesEntry{
		ImageAliasesEntryPut: api.ImageAliasesEntryPut{Target: fingerprint},
	}

	suite.InstMock.On("GetInstance", name).Return((*api.Instance)(nil),
		"", api.StatusErrorf(http.StatusNotFound, ""))
	suite.InstMock.On("GetImageAlias", "ubuntu@20.04").Return(&alias, "",
		nil)
	suite.InstMock.On("GetImage", fingerprint).Return(&image, "",
		nil)
	suite.InstMock.On("CreateInstanceFromImage", &suite.Srv, image, mock.Anything).Return(&op,
		nil)

	op.On("AddHandler", mock.Anything).Return((*lxd.EventTarget)(nil), nil)
	op.On("Wait").Return(nil)

	err := suite.Srv.LaunchWorkspaceInstance(name, base)
	assert.Equal(suite.T(), err, nil)
	suite.InstMock.AssertExpectations(suite.T())
}

func (suite *LaunchTestSuite) TestWorkspaceLxdLaunchNoLocalImage() {

	ConnectSimpleStreams = func(url string, args *lxd.ConnectionArgs) (lxd.ImageServer, error) {
		return &suite.ImgMock, nil
	}

	var name, base, fingerprint string = "test", "ubuntu@20.04", "FS34DS"

	var remoteImageAlias api.ImageAliasesEntry
	remoteImageAlias.Target = fingerprint

	var localImageAlias api.ImageAliasesPost
	localImageAlias.Name = base
	localImageAlias.Target = fingerprint

	var image api.Image
	image.Fingerprint = fingerprint

	var op MockRemoteOperation
	var err error

	suite.InstMock.On("GetInstance", name).Return((*api.Instance)(nil),
		"", api.StatusErrorf(http.StatusNotFound, ""))
	suite.InstMock.On("GetImageAlias", "ubuntu@20.04").Return((*api.ImageAliasesEntry)(nil), "",
		api.StatusErrorf(http.StatusNotFound, ""))
	suite.ImgMock.On("GetImageAlias", "20.04/amd64").Return(&remoteImageAlias, "",
		nil)
	suite.ImgMock.On("GetImage", fingerprint).Return(&image, "",
		nil)

	suite.InstMock.On("CreateInstanceFromImage", &suite.ImgMock, image, mock.Anything).Return(&op,
		nil)

	op.On("AddHandler", mock.Anything).Return((*lxd.EventTarget)(nil), nil)
	op.On("Wait").Return(nil)

	suite.InstMock.On("CreateImageAlias", localImageAlias).Return(nil)

	err = suite.Srv.LaunchWorkspaceInstance(name, base)
	assert.NoError(suite.T(), err)
	suite.InstMock.AssertExpectations(suite.T())
}

func TestRunLxdServerTests(t *testing.T) {
	suite.Run(t, &LaunchTestSuite{})
}
