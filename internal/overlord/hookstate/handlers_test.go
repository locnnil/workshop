package hookstate_test

import (
	"context"
	"errors"
	"testing"

	"github.com/canonical/workspace/internal/overlord"
	"github.com/canonical/workspace/internal/overlord/hookstate"
	"github.com/canonical/workspace/internal/overlord/sdkstate"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/testutil"
	"github.com/canonical/workspace/internal/workspacebackend"
	"github.com/spf13/afero"
	"gopkg.in/check.v1"
	"gopkg.in/tomb.v2"
)

type hookSuite struct {
	fs      afero.Fs
	backend *workspacebackend.FakeWorkspaceBackend
	state   *state.State
	runner  *state.TaskRunner
	se      *overlord.StateEngine
	wsmgr   *sdkstate.SdkManager
	ctx     context.Context
	project *workspacebackend.Project

	restoreProjectId func()
}

var _ = check.Suite(&hookSuite{})

func TestHookSuite(t *testing.T) { check.TestingT(t) }

func fakeHandler(task *state.Task, _ *tomb.Tomb) error {
	return nil
}

func setWorkspaceProject(w string, p *workspacebackend.Project, tasks ...*state.Task) {
	for _, i := range tasks {
		i.Set("workspace", w)
		i.Set("project-id", p.ProjectId)
	}
}

var ErrTrigger = errors.New("error out")

func (s *hookSuite) SetUpTest(c *check.C) {
	s.fs = afero.NewMemMapFs()
	ctx := context.WithValue(context.TODO(), workspacebackend.ContextProjectId, "projectId")
	s.ctx = context.WithValue(ctx, workspacebackend.ContextUser, "testuser")

	s.backend = workspacebackend.NewFakeWorkspaceBackend()
	s.project = &workspacebackend.Project{
		Path:      c.MkDir(),
		ProjectId: "projectId",
	}
	s.restoreProjectId = testutil.FakeFunc(func() (string, error) { return s.project.ProjectId, nil }, &workspacebackend.NewProjectId)
	s.backend.CreateOrLoadProject(s.ctx, s.project.Path)

	s.state = state.New(nil)
	s.runner = state.NewTaskRunner(s.state)

	/* empty task handler */
	s.runner.AddHandler("fake-task", fakeHandler, nil)
	s.wsmgr = sdkstate.NewSdkManager(s.runner, s.backend)

	/* error-provoking task handler */
	erroringHandler := func(task *state.Task, _ *tomb.Tomb) error {
		return ErrTrigger
	}
	s.runner.AddHandler("error-trigger", erroringHandler, nil)

	s.se = overlord.NewStateEngine(s.state)
	s.se.AddManager(s.wsmgr)
	s.se.AddManager(s.runner)
}

func (s *hookSuite) TearDownTest(c *check.C) {
	s.restoreProjectId()
}

func (s *hookSuite) TestExecSetupBaseNoHook(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	newSdk := workspacebackend.Sdk{"new", "latest/stable"}
	hook := hookstate.HookSetup{
		Sdk:      newSdk,
		HookType: workspacebackend.SetupBase,
	}
	t1 := s.state.NewTask("run-hook", "test")
	t1.Set("hook-setup", hook)

	chg := s.state.NewChange("sample", "...")
	setWorkspaceProject("ws", s.project, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	executed := false

	s.backend.LaunchWorkspace(s.ctx, "ws", "ubuntu@20.04")
	s.backend.DoExec = func(ctx context.Context, name string, args *workspacebackend.ExecArgs) (chan bool, error) {
		executed = true
		return workspacebackend.DoExecDefault(ctx, name, args)
	}

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	/* Install must be successful */
	c.Check(chg.Err(), check.Equals, nil)
	c.Check(executed, check.Equals, false)
}
