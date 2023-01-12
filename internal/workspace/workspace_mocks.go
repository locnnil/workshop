package workspace

import (
	store "github.com/canonical/workspace/internal/fakestore"
	"github.com/canonical/workspace/internal/server"
	lxd "github.com/lxc/lxd/client"
	"github.com/stretchr/testify/mock"
)

type MockServer struct {
	lxd.InstanceServer
	mock.Mock
}

func (s *MockServer) LaunchWorkspaceInstance(name, base string) error {
	args := s.Called(name, base)
	return args.Error(0)
}

func (s *MockServer) SetWorkspaceState(name, action string) error {
	args := s.Called(name, action)
	return args.Error(0)
}

func (s *MockServer) UpdateWorkspaceDevices(name string, devices server.WorkspaceDevices) error {
	args := s.Called(devices)
	return args.Error(0)
}

func (s *MockServer) GetWorkspaceDevices(name string) (server.WorkspaceDevices, error) {
	args := s.Called()
	return args.Get(0).(server.WorkspaceDevices), args.Error(1)
}

func (s *MockServer) Exec(name, user string, args []string) error {
	mockArgs := s.Called(name, user, args)
	return mockArgs.Error(0)
}

type StoreClientMock struct {
	mock.Mock
}

func (s *StoreClientMock) FetchSDK(name, channel, destination string) (store.SDKFile, error) {
	args := s.Called(name, channel, destination)
	return args.Get(0).(store.SDKFile), args.Error(1)
}
