package sdkstate_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/overlord"
	"github.com/canonical/workshop/internal/overlord/sdkstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"

	"github.com/spf13/afero"
	"gopkg.in/check.v1"
	"gopkg.in/tomb.v2"
)

type H struct {
	fs          afero.Fs
	backend     *workshopbackend.FakeWorkshopBackend
	state       *state.State
	runner      *state.TaskRunner
	se          *overlord.StateEngine
	wsmgr       *sdkstate.SdkManager
	ctx         context.Context
	project     *workshopbackend.Project
	installTime time.Time

	restoreProjectId   func()
	restoreInstallTime func()
}

var _ = check.Suite(&H{})

func Test(t *testing.T) { check.TestingT(t) }

func fakeHandler(task *state.Task, _ *tomb.Tomb) error {
	return nil
}

func setWorkshopProject(w string, p *workshopbackend.Project, tasks ...*state.Task) {
	for _, i := range tasks {
		i.Set("workshop", w)
		i.Set("project", p)
	}
}

var ErrTrigger = errors.New("error out")

func (s *H) SetUpTest(c *check.C) {
	s.fs = afero.NewMemMapFs()
	ctx := context.WithValue(context.TODO(), workshopbackend.ContextProjectId, "projectId")
	s.ctx = context.WithValue(ctx, workshopbackend.ContextUser, "testuser")

	s.backend = workshopbackend.NewFakeWorkshopBackend()
	s.project = &workshopbackend.Project{
		Path:      c.MkDir(),
		ProjectId: "projectId",
	}
	s.restoreProjectId = testutil.FakeFunc(func() (string, error) { return s.project.ProjectId, nil }, &workshopbackend.NewProjectId)
	s.backend.CreateOrLoadProject(s.ctx, s.project.Path)

	s.state = state.New(nil)
	s.runner = state.NewTaskRunner(s.state)

	/* empty task handler */
	s.runner.AddHandler("fake-task", fakeHandler, nil)
	s.wsmgr = sdkstate.New(s.runner, s.backend)

	/* error-provoking task handler */
	erroringHandler := func(task *state.Task, _ *tomb.Tomb) error {
		return ErrTrigger
	}
	s.runner.AddHandler("error-trigger", erroringHandler, nil)

	s.se = overlord.NewStateEngine(s.state)
	s.se.StartUp()
	s.se.AddManager(s.wsmgr)
	s.se.AddManager(s.runner)

	s.installTime = time.Date(2023, 04, 25, 1, 2, 3, 0, time.UTC)
	s.restoreInstallTime = testutil.FakeFunc(func() time.Time { return s.installTime }, &workshopbackend.InstallTimeNow)

	err := os.WriteFile(filepath.Join(s.project.Path, ".workshop.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04
sdks:
  test:
    channel: latest/stable
`), 0644)
	c.Assert(err, check.IsNil)
	err = s.backend.LaunchWorkshop(s.ctx, "ws", "ubuntu@20.04")
	c.Assert(err, check.IsNil)

	var sdkYaml = `
name: test
base: ubuntu@22.04
plugs:
  plug:
    interface: content
    target: /project/sub
`
	s.mockTestSdk(c, sdkYaml)
}

func (s *H) mockTestSdk(c *check.C, sdkYaml string) {
	sdkPath := filepath.Join(dirs.WorkshopSdksDir, "test", "current", "meta", "sdk.yaml")
	fs, err := s.backend.WorkshopFs(s.ctx, "ws")
	c.Assert(err, check.IsNil)
	err = afero.WriteFile(fs, sdkPath, []byte(sdkYaml), 0644)
	c.Assert(err, check.IsNil)
}

func (s *H) TearDownTest(c *check.C) {
	s.restoreProjectId()
	s.restoreInstallTime()
}

func (s *H) TestDoInstallSdkSuccess(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	newSdk := sdk.Setup{Name: "test-2", Channel: "latest/stable", Revision: 2, InstallTime: s.installTime}
	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", newSdk)
	t1 := s.state.NewTask("install-sdk", "test")
	t1.Set("sdk-retrieve-task", t.ID())

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	chg.AddTask(t)

	s.state.Unlock()
	s.se.Ensure()
	s.se.Wait()
	s.state.Lock()

	c.Assert(s.backend.ExecCalls, check.HasLen, 1)
	c.Assert(s.backend.ExecCalls[0].Args.Command, check.DeepEquals, []string{
		"tar",
		"--extract",
		"--file",
		"/root/test-2_2.sdk",
		"--one-top-level=/var/lib/workshop/sdk/test-2/2",
		"--no-same-owner",
	})

	c.Check(t1.Status(), check.Equals, state.DoneStatus)
}

func (s *H) TestDoInstallSdkExecFail(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	newSdk := sdk.Info{Name: "test"}
	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", newSdk)
	t1 := s.state.NewTask("install-sdk", "test")
	t1.Set("sdk-retrieve-task", t.ID())

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	chg.AddTask(t)

	s.backend.DoExec = func(ctx context.Context, name string, args *workshopbackend.Execution) (workshopbackend.ExecContext, error) {
		args.Stderr.Write([]byte(os.ErrDeadlineExceeded.Error()))
		return workshopbackend.ExecContext{}, os.ErrDeadlineExceeded
	}

	s.state.Unlock()
	s.se.Ensure()
	s.se.Wait()
	s.state.Lock()

	c.Check(strings.HasSuffix(t1.Log()[0], os.ErrDeadlineExceeded.Error()), check.Equals, true)
}

func (s *H) TestUndoInstallSdkSuccess(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	newSdk := sdk.Setup{Name: "test-2", Channel: "latest/stable", Revision: 2, InstallTime: s.installTime}
	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", newSdk)
	t1 := s.state.NewTask("install-sdk", "test")
	t1.Set("sdk-retrieve-task", t.ID())

	terr := s.state.NewTask("error-trigger", "provoking total undo")
	terr.WaitFor(t1)

	chg := s.state.NewChange("sample", "...")
	chg.Set("workshop", "ws")
	chg.Set("project-id", s.project.ProjectId)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	chg.AddTask(t)
	chg.AddTask(terr)

	// emulate install behaviour that unpacks an SDK to a certain directory
	s.backend.DoExec = func(ctx context.Context, name string, args *workshopbackend.Execution) (workshopbackend.ExecContext, error) {
		fs, _ := s.backend.WorkshopFs(ctx, name)
		fs.MkdirAll(filepath.Join(dirs.WorkshopSdksDir, "new"), 0755)
		return workshopbackend.ExecContext{}, nil
	}

	s.state.Unlock()
	s.se.Ensure()
	s.se.Wait()
	s.state.Lock()

	// make sure SDK dir was removed
	fs, err := s.backend.WorkshopFs(s.ctx, "ws")
	c.Check(err, check.IsNil)
	exist, _ := afero.Exists(fs, filepath.Join(dirs.WorkshopSdksDir, "new"))
	c.Check(exist, check.Equals, false)
}

func (s *H) TestDoLinkSdkSuccess(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	testSdk := sdk.Setup{Name: "test", Channel: "latest/stable", Revision: 2, InstallTime: s.installTime}

	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", testSdk)
	t1 := s.state.NewTask("link-sdk", "test")
	t1.Set("sdk-retrieve-task", t.ID())

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	chg.AddTask(t)

	s.state.Unlock()
	s.se.Ensure()
	s.se.Wait()
	s.state.Lock()

	c.Check(chg.Err(), check.Equals, nil)
	props, err := s.backend.Workshop(s.ctx, "ws")
	c.Assert(err, check.IsNil)
	info := props.Content()
	c.Check(info, check.HasLen, 1)
	c.Check(info[0], check.DeepEquals, testSdk)

	sdkInfo, err := props.SdkInfo(s.ctx, info[0].Name)
	c.Assert(err, check.IsNil)
	c.Assert(sdkInfo.Plugs, check.HasLen, 1)
	c.Assert(sdkInfo.Slots, check.HasLen, 0)
}

func (s *H) TestUndoLinkSdkAndRemoveSdk(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	newSdk := sdk.Info{Workshop: "ws", Name: "test"}
	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", newSdk)
	link := s.state.NewTask("link-sdk", "test")
	link.Set("sdk-retrieve-task", t.ID())

	terr := s.state.NewTask("error-trigger", "provoking total undo")
	terr.WaitFor(link)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, link, t)

	chg.Set("user", "testuser")
	chg.AddTask(link)
	chg.AddTask(t)
	chg.AddTask(terr)

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	props, err := s.backend.Workshop(s.ctx, "ws")
	c.Assert(err, check.IsNil)
	info := props.Content()
	c.Check(info, check.HasLen, 0)
	c.Check(link.Status(), check.Equals, state.UndoneStatus)
}
