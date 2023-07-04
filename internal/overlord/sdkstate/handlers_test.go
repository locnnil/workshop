package sdkstate_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/canonical/workspace/internal/overlord"
	"github.com/canonical/workspace/internal/overlord/sdkstate"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/sdk"
	"github.com/canonical/workspace/internal/testutil"
	"github.com/canonical/workspace/internal/workspacebackend"

	"github.com/spf13/afero"
	"gopkg.in/check.v1"
	. "gopkg.in/check.v1"
	"gopkg.in/tomb.v2"
)

type H struct {
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

var _ = Suite(&H{})

func Test(t *testing.T) { TestingT(t) }

func fakeHandler(task *state.Task, _ *tomb.Tomb) error {
	return nil
}

func setWorkspaceProject(w string, p *workspacebackend.Project, tasks ...*state.Task) {
	for _, i := range tasks {
		i.Set("workspace", w)
		i.Set("project-key", p)
	}
}

var ErrTrigger = errors.New("error out")

func (s *H) SetUpTest(c *C) {
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
	s.se.StartUp()
	s.se.AddManager(s.wsmgr)
	s.se.AddManager(s.runner)
}

func (s *H) TearDownTest(c *C) {
	s.restoreProjectId()
}

func (s *H) TestDoInstallSdkSuccess(c *C) {
	s.state.Lock()
	defer s.state.Unlock()
	newSdk := sdk.SdkInfo{"new", "latest/stable", 2}
	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", newSdk)
	t1 := s.state.NewTask("install-sdk", "test")
	t1.Set("sdk-retrieve-task", t.ID())

	chg := s.state.NewChange("sample", "...")
	setWorkspaceProject("ws", s.project, t, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	chg.AddTask(t)

	s.backend.LaunchWorkspace(s.ctx, "ws", "ubuntu@20.04")
	s.backend.DoExec = func(ctx context.Context, name string, args *workspacebackend.ExecArgs) (chan bool, error) {
		c.Check(args.Command, DeepEquals, []string{
			"tar",
			"--extract",
			"--file",
			"/root/new_2.sdk",
			"--one-top-level=/var/lib/workspace/sdk/new/2",
			"--no-same-owner",
		})
		return workspacebackend.DoExecDefault(ctx, name, args)
	}

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	/* Install must be successful */
	c.Check(chg.Err(), Equals, nil)
	props, _ := s.backend.GetWorkspace(s.ctx, "ws")
	c.Check(props.Devices["new"], DeepEquals, map[string]string(nil))
}

func (s *H) TestDoInstallSdkExecFail(c *C) {
	s.state.Lock()
	defer s.state.Unlock()

	newSdk := sdk.SdkInfo{"new", "latest/stable", 2}
	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", newSdk)
	t1 := s.state.NewTask("install-sdk", "test")
	t1.Set("sdk-retrieve-task", t.ID())

	chg := s.state.NewChange("sample", "...")
	setWorkspaceProject("ws", s.project, t, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	chg.AddTask(t)

	s.backend.LaunchWorkspace(s.ctx, "ws", "ubuntu@20.04")
	s.backend.DoExec = func(ctx context.Context, name string, args *workspacebackend.ExecArgs) (chan bool, error) {
		args.Stderr.Write([]byte(os.ErrDeadlineExceeded.Error()))
		done := make(chan bool)
		close(done)
		return done, os.ErrDeadlineExceeded
	}

	s.state.Unlock()
	s.se.Ensure()
	s.se.Wait()
	s.state.Lock()

	props, _ := s.backend.GetWorkspace(s.ctx, "ws")
	c.Check(props.Devices["new"], DeepEquals, map[string]string(nil))
	c.Check(strings.HasSuffix(t1.Log()[0], os.ErrDeadlineExceeded.Error()), Equals, true)
}

func (s *H) TestUndoInstallSdkSuccess(c *C) {
	s.state.Lock()
	defer s.state.Unlock()

	newSdk := sdk.SdkInfo{"new", "latest/stable", 2}
	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", newSdk)
	t1 := s.state.NewTask("install-sdk", "test")
	t1.Set("sdk-retrieve-task", t.ID())

	terr := s.state.NewTask("error-trigger", "provoking total undo")
	terr.WaitFor(t1)

	chg := s.state.NewChange("sample", "...")
	chg.Set("workspace", "ws")
	chg.Set("project-key", s.project)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	chg.AddTask(t)
	chg.AddTask(terr)

	s.backend.LaunchWorkspace(s.ctx, "ws", "ubuntu@20.04")
	/* emulate install behaviour that unpacks an SDK to a certain directory */
	s.backend.DoExec = func(ctx context.Context, name string, args *workspacebackend.ExecArgs) (chan bool, error) {
		s.backend.WsFs.MkdirAll(filepath.Join(workspacebackend.WorkspaceSdksDir, "new"), 0755)
		return workspacebackend.DoExecDefault(ctx, name, args)
	}

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	props, _ := s.backend.GetWorkspace(s.ctx, "ws")
	c.Check(props.Devices["new"], DeepEquals, map[string]string(nil))
	/* make sure SDK dir was removed */
	exist, _ := afero.Exists(s.backend.WsFs, filepath.Join(workspacebackend.WorkspaceSdksDir, "new"))
	c.Check(exist, Equals, false)
}

func (s *H) TestDoLinkSdkSuccess(c *C) {
	s.state.Lock()
	defer s.state.Unlock()

	newSdk := sdk.SdkInfo{"new", "latest/stable", 2}
	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", newSdk)
	t1 := s.state.NewTask("link-sdk", "test")
	t1.Set("sdk-retrieve-task", t.ID())

	chg := s.state.NewChange("sample", "...")
	setWorkspaceProject("ws", s.project, t, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	chg.AddTask(t)

	s.backend.LaunchWorkspace(s.ctx, "ws", "ubuntu@20.04")

	s.state.Unlock()
	s.se.Ensure()
	s.se.Wait()
	s.state.Lock()

	c.Check(chg.Err(), Equals, nil)
	props, _ := s.backend.GetWorkspace(s.ctx, "ws")
	info := props.Content()
	c.Check(info, check.HasLen, 1)
	c.Check(*info[0], check.DeepEquals, newSdk)
}

func (s *H) TestUndoLinkSdkAndRemoveSdk(c *C) {
	s.state.Lock()
	defer s.state.Unlock()

	newSdk := sdk.SdkInfo{"new", "latest/stable", 2}
	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", newSdk)
	link := s.state.NewTask("link-sdk", "test")
	link.Set("sdk-retrieve-task", t.ID())

	terr := s.state.NewTask("error-trigger", "provoking total undo")
	terr.WaitFor(link)

	chg := s.state.NewChange("sample", "...")
	setWorkspaceProject("ws", s.project, link, t)

	chg.Set("user", "testuser")
	chg.AddTask(link)
	chg.AddTask(t)
	chg.AddTask(terr)

	s.backend.LaunchWorkspace(s.ctx, "ws", "ubuntu@20.04")

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	props, _ := s.backend.GetWorkspace(s.ctx, "ws")
	info := props.Content()
	c.Check(info, check.HasLen, 0)
	c.Check(link.Status(), Equals, state.UndoneStatus)
}
