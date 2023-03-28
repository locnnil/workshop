package workspacebackend_test

import (
	"net/http"
	"testing"

	"github.com/canonical/workspace/internal/mocks"
	"github.com/canonical/workspace/internal/workspacebackend"
	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type LxdServerTestSuite struct {
	suite.Suite
	Fs       afero.Fs
	Srv      workspacebackend.LxdBackend
	InstMock *mocks.MockInstanceServer
	ImgMock  *mocks.MockImageServer
}

var ApiErrNotFound = api.StatusErrorf(http.StatusNotFound, "")
var ApiErrCancelled = api.StatusErrorf(int(api.Cancelled), "")

func (s *LxdServerTestSuite) SetupTest() {
	s.Fs = afero.NewMemMapFs()
	s.Srv = workspacebackend.LxdBackend{}
	s.InstMock = mocks.NewMockInstanceServer(s.T())
	s.Srv.InstanceServer = s.InstMock
	s.ImgMock = mocks.NewMockImageServer(s.T())
}

func (s *LxdServerTestSuite) TestLaunchLocalImageExists() {
	var name, base, project, fingerprint string = "test", "ubuntu@20.04", "12345", "FS34DS"
	var op = mocks.NewMockRemoteOperation(s.T())
	var image = api.Image{
		Fingerprint: fingerprint,
	}
	var alias = api.ImageAliasesEntry{
		ImageAliasesEntryPut: api.ImageAliasesEntryPut{
			Target: fingerprint,
		},
	}

	s.InstMock.
		On("GetInstance", "test-12345").Return((*api.Instance)(nil), "", ApiErrNotFound).
		On("GetImageAlias", "ubuntu@20.04").Return(&alias, "", nil).
		On("GetImage", fingerprint).Return(&image, "", nil).
		On("CreateInstanceFromImage", &s.Srv, image, mock.Anything).Return(op, nil)

	op.On("Wait").Return(nil)

	err := s.Srv.LaunchWorkspaceInstance(name, base, project)
	assert.Equal(s.T(), err, nil)
	s.InstMock.AssertExpectations(s.T())
}

func (s *LxdServerTestSuite) TestLaunchNoLocalImage() {
	var name, base, project, fingerprint string = "test", "ubuntu@20.04", "12345", "FS34DS"
	var remoteImageAlias api.ImageAliasesEntry
	var localImageAlias api.ImageAliasesPost
	var image api.Image
	var op = mocks.NewMockRemoteOperation(s.T())
	var err error

	oldSimpleStreams := workspacebackend.ConnectSimpleStreams
	workspacebackend.ConnectSimpleStreams = func(url string, args *lxd.ConnectionArgs) (lxd.ImageServer, error) {
		return s.ImgMock, nil
	}
	defer func() { workspacebackend.ConnectSimpleStreams = oldSimpleStreams }()

	remoteImageAlias.Target = fingerprint
	localImageAlias.Name = base
	localImageAlias.Target = fingerprint
	image.Fingerprint = fingerprint

	s.InstMock.
		On("GetInstance", "test-12345").Return((*api.Instance)(nil), "", ApiErrNotFound).
		On("GetImageAlias", "ubuntu@20.04").Return((*api.ImageAliasesEntry)(nil), "", ApiErrNotFound).
		On("CreateImageAlias", localImageAlias).Return(nil).
		On("CreateInstanceFromImage", s.ImgMock, image, mock.Anything).Return(op, nil)

	// The image is provided by a remote image server via an alias
	s.ImgMock.
		On("GetImageAlias", "20.04/amd64").Return(&remoteImageAlias, "", nil).
		On("GetImage", fingerprint).Return(&image, "", nil)

	op.On("Wait").Return(nil)

	err = s.Srv.LaunchWorkspaceInstance(name, base, project)
	assert.NoError(s.T(), err)
	s.InstMock.AssertExpectations(s.T())
	s.ImgMock.AssertExpectations(s.T())
}

func TestRunLxdServerTests(t *testing.T) {
	suite.Run(t, &LxdServerTestSuite{})
}
