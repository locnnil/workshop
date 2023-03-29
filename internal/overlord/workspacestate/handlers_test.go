package workspace_test

import (
	"errors"

	store "github.com/canonical/workspace/internal/fakestore"
	"github.com/canonical/workspace/internal/overlord"
	"github.com/canonical/workspace/internal/overlord/projectstate"
	"github.com/canonical/workspace/internal/overlord/state"
	workspace "github.com/canonical/workspace/internal/overlord/workspacestate"
	"github.com/canonical/workspace/internal/workspacebackend"

	"github.com/spf13/afero"
	. "gopkg.in/check.v1"
	"gopkg.in/tomb.v2"
)

type H struct {
	fs      afero.Fs
	backend *workspacebackend.FakeWorkspaceBackend
	state   *state.State
	runner  *state.TaskRunner
	se      *overlord.StateEngine
	wsmgr   *workspace.WorkspaceManager
}

var _ = Suite(&H{})

func fakeHandler(task *state.Task, _ *tomb.Tomb) error {
	return nil
}

func (s *H) SetUpTest(c *C) {
	s.fs = afero.NewMemMapFs()
	s.backend = workspacebackend.NewFakeWorkspaceBackend()

	s.state = state.New(nil)
	s.runner = state.NewTaskRunner(s.state)

	/* empty task handler */
	s.runner.AddHandler("fake-task", fakeHandler, nil)
	s.wsmgr = workspace.NewWorkspaceManager(s.runner, s.backend)

	/* error-provoking task handler */
	erroringHandler := func(task *state.Task, _ *tomb.Tomb) error {
		return errors.New("error out")
	}
	s.runner.AddHandler("error-trigger", erroringHandler, nil)

	s.se = overlord.NewStateEngine(s.state)
	s.se.AddManager(s.wsmgr)
	s.se.AddManager(s.runner)
}

func (s *H) TestDoLinkSdkSuccess(c *C) {
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

	s.backend.LaunchWorkspaceInstance("ws", "ubuntu@20.04", "projectId")

	s.state.Unlock()
	s.se.Ensure()
	s.se.Wait()
	s.state.Lock()

	c.Check(chg.Err(), Equals, nil)
	props, _ := s.backend.GetWorkspace("ws", "projectId")
	c.Check(props.Config["user.workspace.sdk"], NotNil)
	c.Check(props.Config["user.workspace.sdk"], Equals,
		"{\"new\":[{\"channel\":\"latest/stable\",\"revision\":2}]}")
}

func (s *H) TestunDoLinkSdkSuccess(c *C) {
	s.state.Lock()
	defer s.state.Unlock()

	projectKey := projectstate.ProjectKey{
		Path:      "projectPath",
		ProjectId: "projectId",
	}

	newSdk := store.SdkBlob{"new", "latest/stable", 2}
	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-blob", newSdk)
	link := s.state.NewTask("link-sdk", "test")
	link.Set("sdk-retrieve-task", t.ID())

	terr := s.state.NewTask("error-trigger", "provoking total undo")
	terr.WaitFor(link)

	chg := s.state.NewChange("sample", "...")
	chg.Set("workspace", "ws")
	chg.Set("project-key", &projectKey)
	chg.AddTask(link)
	chg.AddTask(t)
	chg.AddTask(terr)

	s.backend.LaunchWorkspaceInstance("ws", "ubuntu@20.04", "projectId")

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	props, _ := s.backend.GetWorkspace("ws", "projectId")
	_, ok := props.Config["user.workspace.sdk"]
	c.Check(ok, Equals, false)
}
