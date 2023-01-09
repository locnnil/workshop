package server

import (
	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/stretchr/testify/mock"
)

type MockLxdImageServer struct {
	mock.Mock
	lxd.ImageServer
}

func (s *MockLxdImageServer) GetImage(fingerprint string) (image *api.Image, ETag string, err error) {
	args := s.Called(fingerprint)
	return args.Get(0).(*api.Image), args.String(1), args.Error(2)
}

func (s *MockLxdImageServer) GetImageAlias(name string) (alias *api.ImageAliasesEntry, ETag string, err error) {
	args := s.Called(name)
	return args.Get(0).(*api.ImageAliasesEntry), args.String(1), args.Error(2)
}

type MockLxdInstanceServer struct {
	mock.Mock
	lxd.InstanceServer
}

func (s *MockLxdInstanceServer) GetImage(fingerprint string) (image *api.Image, ETag string, err error) {
	args := s.Called(fingerprint)
	return args.Get(0).(*api.Image), args.String(1), args.Error(2)
}

func (s *MockLxdInstanceServer) GetProject(name string) (project *api.Project, ETag string, err error) {
	args := s.Called(name)
	return args.Get(0).(*api.Project), args.String(1), args.Error(2)
}

func (s *MockLxdInstanceServer) CreateProject(project api.ProjectsPost) (err error) {
	args := s.Called(project)
	return args.Error(0)
}
