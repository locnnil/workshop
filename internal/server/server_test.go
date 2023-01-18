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

var ApiErrNotFound = api.StatusErrorf(http.StatusNotFound, "")

func (s *LaunchTestSuite) SetupTest() {
	s.Fs = afero.NewMemMapFs()
	s.Srv = LxdServer{filesystem: s.Fs}
	s.InstMock = MockLxdInstanceServer{}
	s.Srv.InstanceServer = &s.InstMock
	s.ImgMock = MockLxdImageServer{}
}

func (s *LaunchTestSuite) TestWorkspaceLxdLaunchLocalImageExists() {
	var name, base, fingerprint string = "test", "ubuntu@20.04", "FS34DS"
	var op MockRemoteOperation
	var image = api.Image{
		Fingerprint: fingerprint,
	}
	var alias = api.ImageAliasesEntry{
		ImageAliasesEntryPut: api.ImageAliasesEntryPut{
			Target: fingerprint,
		},
	}

	s.InstMock.On("GetInstance", name).Return((*api.Instance)(nil), "", ApiErrNotFound)
	s.InstMock.On("GetImageAlias", "ubuntu@20.04").Return(&alias, "", nil)
	s.InstMock.On("GetImage", fingerprint).Return(&image, "", nil)
	s.InstMock.On("CreateInstanceFromImage", &s.Srv, image, mock.Anything).Return(&op, nil)

	op.On("AddHandler", mock.Anything).Return((*lxd.EventTarget)(nil), nil)
	op.On("Wait").Return(nil)

	err := s.Srv.LaunchWorkspaceInstance(name, base)
	assert.Equal(s.T(), err, nil)
	s.InstMock.AssertExpectations(s.T())
}

func (s *LaunchTestSuite) TestWorkspaceLxdLaunchNoLocalImage() {
	ConnectSimpleStreams = func(url string, args *lxd.ConnectionArgs) (lxd.ImageServer, error) {
		return &s.ImgMock, nil
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

	s.InstMock.On("GetInstance", name).Return((*api.Instance)(nil), "", ApiErrNotFound)
	s.InstMock.On("GetImageAlias", "ubuntu@20.04").Return((*api.ImageAliasesEntry)(nil), "", ApiErrNotFound)
	s.ImgMock.On("GetImageAlias", "20.04/amd64").Return(&remoteImageAlias, "", nil)
	s.ImgMock.On("GetImage", fingerprint).Return(&image, "", nil)
	s.InstMock.On("CreateInstanceFromImage", &s.ImgMock, image, mock.Anything).Return(&op, nil)

	op.On("AddHandler", mock.Anything).Return((*lxd.EventTarget)(nil), nil)
	op.On("Wait").Return(nil)

	s.InstMock.On("CreateImageAlias", localImageAlias).Return(nil)

	err = s.Srv.LaunchWorkspaceInstance(name, base)
	assert.NoError(s.T(), err)
	s.InstMock.AssertExpectations(s.T())
	s.ImgMock.AssertExpectations(s.T())
}

func TestRunLxdServerTests(t *testing.T) {
	suite.Run(t, &LaunchTestSuite{})
}
