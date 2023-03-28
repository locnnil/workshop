package workspace_test

import (
	"testing"

	store "github.com/canonical/workspace/internal/fakestore"
	"github.com/canonical/workspace/internal/mocks"
	"github.com/canonical/workspace/internal/overlord"
	"github.com/canonical/workspace/internal/overlord/projectstate"
	"github.com/canonical/workspace/internal/overlord/state"
	workspace "github.com/canonical/workspace/internal/overlord/workspacestate"
	"github.com/canonical/workspace/internal/workspacebackend"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/tomb.v2"
)

type HandlersTestSuite struct {
	suite.Suite
	fs      afero.Fs
	backend *mocks.MockWorkspaceBackend
	state   *state.State
	runner  *state.TaskRunner
	se      *overlord.StateEngine
	wsmgr   *workspace.WorkspaceManager
}

func TestRunProjectTests(t *testing.T) {
	suite.Run(t, &HandlersTestSuite{})
}

func fakeHandler(task *state.Task, _ *tomb.Tomb) error {
	return nil
}

func (s *HandlersTestSuite) SetupTest() {
	s.fs = afero.NewMemMapFs()
	s.backend = mocks.NewMockWorkspaceBackend(s.T())
	s.state = state.New(nil)
	s.runner = state.NewTaskRunner(s.state)
	s.runner.AddHandler("fake-task", fakeHandler, nil)
	s.wsmgr = workspace.NewWorkspaceManager(s.runner, s.backend)

	s.se = overlord.NewStateEngine(s.state)
	s.se.AddManager(s.wsmgr)
	s.se.AddManager(s.runner)
}

func (s *HandlersTestSuite) TestDoLinkSdkSuccess() {
	s.state.Lock()
	defer s.state.Unlock()

	projectKey := projectstate.ProjectKey{
		Path:      "projectPath",
		ProjectId: "projectId",
	}

	newSdk := store.SdkBlob{"new", "latest/stable", 2}
	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-blob", newSdk)
	t1 := s.state.NewTask("link-sdk", "test")
	t1.Set("sdk-retrieve-task", t.ID())

	chg := s.state.NewChange("sample", "...")
	chg.Set("workspace", "ws")
	chg.Set("project-key", &projectKey)
	chg.AddTask(t1)
	chg.AddTask(t)

	w := &workspacebackend.WorkspaceProps{}

	s.backend.On("GetWorkspace", "ws", "projectId").Return(w, nil)
	s.backend.On("AddWorkspaceConfig", "ws", "projectId",
		mock.Anything).Return(nil)
	done := make(chan bool)
	close(done)
	s.backend.On("Exec", "ws", "projectId", mock.Anything).Return(done, nil)

	s.state.Unlock()
	s.se.Ensure()
	s.se.Wait()
	s.state.Lock()

	assert.NoError(s.T(), chg.Err())
}
