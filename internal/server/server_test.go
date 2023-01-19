package server_test

import (
	"net/http"
	"testing"

	"github.com/canonical/workspace/internal/mocks"
	"github.com/canonical/workspace/internal/server"
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
	Srv      server.LxdServer
	InstMock *mocks.MockInstanceServer
	ImgMock  *mocks.MockImageServer
}

var ApiErrNotFound = api.StatusErrorf(http.StatusNotFound, "")
var ApiErrCancelled = api.StatusErrorf(int(api.Cancelled), "")

func (s *LxdServerTestSuite) SetupTest() {
	s.Fs = afero.NewMemMapFs()
	s.Srv = server.LxdServer{Fs: s.Fs}
	s.InstMock = mocks.NewMockInstanceServer(s.T())
	s.Srv.InstanceServer = s.InstMock
	s.ImgMock = mocks.NewMockImageServer(s.T())
}

func (s *LxdServerTestSuite) TestLaunchLocalImageExists() {
	var name, base, fingerprint string = "test", "ubuntu@20.04", "FS34DS"
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
		On("GetInstance", name).Return((*api.Instance)(nil), "", ApiErrNotFound).
		On("GetImageAlias", "ubuntu@20.04").Return(&alias, "", nil).
		On("GetImage", fingerprint).Return(&image, "", nil).
		On("CreateInstanceFromImage", &s.Srv, image, mock.Anything).Return(op, nil)

	op.
		On("AddHandler", mock.Anything).Return((*lxd.EventTarget)(nil), nil).
		On("Wait").Return(nil)

	err := s.Srv.LaunchWorkspaceInstance(name, base)
	assert.Equal(s.T(), err, nil)
	s.InstMock.AssertExpectations(s.T())
}

func (s *LxdServerTestSuite) TestLaunchNoLocalImage() {
	var name, base, fingerprint string = "test", "ubuntu@20.04", "FS34DS"
	var remoteImageAlias api.ImageAliasesEntry
	var localImageAlias api.ImageAliasesPost
	var image api.Image
	var op = mocks.NewMockRemoteOperation(s.T())
	var err error

	oldSimpleStreams := server.ConnectSimpleStreams
	server.ConnectSimpleStreams = func(url string, args *lxd.ConnectionArgs) (lxd.ImageServer, error) {
		return s.ImgMock, nil
	}
	defer func() { server.ConnectSimpleStreams = oldSimpleStreams }()

	remoteImageAlias.Target = fingerprint
	localImageAlias.Name = base
	localImageAlias.Target = fingerprint
	image.Fingerprint = fingerprint

	s.InstMock.
		On("GetInstance", name).Return((*api.Instance)(nil), "", ApiErrNotFound).
		On("GetImageAlias", "ubuntu@20.04").Return((*api.ImageAliasesEntry)(nil), "", ApiErrNotFound).
		On("CreateImageAlias", localImageAlias).Return(nil).
		On("CreateInstanceFromImage", s.ImgMock, image, mock.Anything).Return(op, nil)

	// The image is provided by a remote image server via an alias
	s.ImgMock.
		On("GetImageAlias", "20.04/amd64").Return(&remoteImageAlias, "", nil).
		On("GetImage", fingerprint).Return(&image, "", nil)

	op.
		On("AddHandler", mock.Anything).Return((*lxd.EventTarget)(nil), nil).
		On("Wait").Return(nil)

	err = s.Srv.LaunchWorkspaceInstance(name, base)
	assert.NoError(s.T(), err)
	s.InstMock.AssertExpectations(s.T())
	s.ImgMock.AssertExpectations(s.T())
}

func (s *LxdServerTestSuite) TestExecCommandStatusCodes() {
	var name string = "translation"
	var metadata = map[error]api.Operation{
		nil:                        {Metadata: map[string]any{"return": 0.0}},
		&server.ErrExec{Status: 2}: {Metadata: map[string]any{"return": 2.0}},
	}

	var op = mocks.NewMockOperation(s.T())

	s.InstMock.On("ExecInstance", name, mock.Anything,
		mock.Anything).Return(op, nil)

	op.On("Wait").Return(nil)

	// Test the command's error code handling (success and fail)
	for i, k := range metadata {
		c := op.On("Get").Return(k)

		_, err := s.Srv.Exec(name, "root", []string{"id"})
		assert.Equal(s.T(), err, i)
		s.InstMock.AssertExpectations(s.T())
		c.Unset()
	}
}

func (s *LxdServerTestSuite) TestExecCommandLxdOpFails() {
	var name string = "translation"

	var op = mocks.NewMockOperation(s.T())

	s.InstMock.On("ExecInstance", name, mock.Anything,
		mock.Anything).Return(op, nil)
	op.On("Wait").Return(ApiErrCancelled)
	_, err := s.Srv.Exec(name, "root", []string{"id"})
	assert.Equal(s.T(), err, ApiErrCancelled)
	s.InstMock.AssertExpectations(s.T())
}

func TestRunLxdServerTests(t *testing.T) {
	suite.Run(t, &LxdServerTestSuite{})
}
