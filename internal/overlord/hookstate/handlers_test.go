package hookstate_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/workshop/internal/overlord"
	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
	"github.com/spf13/afero"
	"gopkg.in/check.v1"
	"gopkg.in/tomb.v2"
)

type hookSuite struct {
	fs      afero.Fs
	backend *workshopbackend.FakeWorkshopBackend
	state   *state.State
	runner  *state.TaskRunner
	se      *overlord.StateEngine
	hookmgr *hookstate.HookManager
	ctx     context.Context
	project *workshopbackend.Project
}

var _ = check.Suite(&hookSuite{})

func TestHookSuite(t *testing.T) { check.TestingT(t) }

func fakeHandler(task *state.Task, _ *tomb.Tomb) error {
	return nil
}

func setWorkshopProject(w string, p *workshopbackend.Project, tasks ...*state.Task) {
	for _, i := range tasks {
		i.Set("workshop", w)
		i.Set("project", *p)
	}
}

var ErrTrigger = errors.New("error out")

func (s *hookSuite) SetUpTest(c *check.C) {
	s.fs = afero.NewMemMapFs()
	ctx := context.WithValue(context.Background(), workshopbackend.ContextUser, "testuser")

	s.backend = workshopbackend.NewFakeWorkshopBackend()

	var err error
	s.project, _, err = s.backend.CreateOrLoadProject(ctx, c.MkDir())
	c.Assert(err, check.IsNil)
	s.ctx = context.WithValue(ctx, workshopbackend.ContextProjectId, s.project.ProjectId)

	s.state = state.New(nil)
	s.runner = state.NewTaskRunner(s.state)

	// empty task handler
	s.runner.AddHandler("fake-task", fakeHandler, nil)
	s.hookmgr = hookstate.New(s.runner, s.backend)

	// error-provoking task handler
	erroringHandler := func(task *state.Task, _ *tomb.Tomb) error {
		return ErrTrigger
	}
	s.runner.AddHandler("error-trigger", erroringHandler, nil)

	s.se = overlord.NewStateEngine(s.state)
	s.se.AddManager(s.hookmgr)
	s.se.AddManager(s.runner)
	err = s.se.StartUp()
	c.Check(err, check.IsNil)
}

func (s *hookSuite) TearDownTest(c *check.C) {
}

func (s *hookSuite) TestExecSetupBaseNoHook(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	newSdk := workshopbackend.SdkRecord{Name: "new", Channel: "latest/stable"}

	t1 := hookstate.SetupHook(s.state, &newSdk, hookstate.SetupBase)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)

	err := os.WriteFile(filepath.Join(s.project.Path, ".workshop.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04
`), 0644)
	c.Check(err, check.IsNil)

	err = s.backend.LaunchWorkshop(s.ctx, "ws", "ubuntu@20.04")
	c.Check(err, check.IsNil)

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	c.Assert(s.backend.ExecCalls, check.HasLen, 0)
	c.Check(chg.Err(), check.Equals, nil)
}

func (s *hookSuite) TestExecSaveState(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	newSdk := workshopbackend.SdkRecord{Name: "one", Channel: "latest/stable"}
	t1 := hookstate.SetupHook(s.state, &newSdk, hookstate.SaveState)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)

	s.launchWorkshop(c, newSdk)

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()
	c.Assert(s.backend.ExecCalls, check.HasLen, 1)
	c.Assert(s.backend.ExecCalls[0].Args.Command, testutil.DeepUnsortedMatches,
		[]string{"bash", "-ue", "-o", "pipefail", "-c", "/var/lib/workshop/sdk/one/current/sdk/hooks/save-state"})

	// ensure that the save-state handler has created the required state directory
	ws, err := s.backend.WorkshopFs(s.ctx, "ws")
	c.Check(err, check.IsNil)
	info, err := ws.Stat("/var/lib/workshop/state/sdk/one")
	c.Check(err, check.IsNil)
	c.Assert(info.IsDir(), check.Equals, true)
	c.Assert(t1.Log(), check.HasLen, 0)
}

func (s *hookSuite) TestExecRestoreState(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	newSdk := workshopbackend.SdkRecord{Name: "one", Channel: "latest/stable"}
	t1 := hookstate.SetupHook(s.state, &newSdk, hookstate.RestoreState)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)

	s.launchWorkshop(c, newSdk)

	// setup state storage (usually already set by the save-state)
	ws, err := s.backend.WorkshopFs(s.ctx, "ws")
	c.Check(err, check.IsNil)
	err = ws.MkdirAll("/var/lib/workshop/state/sdk/one", 0755)
	c.Check(err, check.IsNil)

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()
	c.Assert(s.backend.ExecCalls, check.HasLen, 1)
	c.Assert(s.backend.ExecCalls[0].Args.Command, testutil.DeepUnsortedMatches,
		[]string{"bash", "-ue", "-o", "pipefail", "-c", "/var/lib/workshop/sdk/one/current/sdk/hooks/restore-state"})
	c.Assert(s.backend.ExecCalls[0].Args.Environment, testutil.DeepUnsortedMatches, map[string]string{"SDK_STATE_DIR": "/var/lib/workshop/state/sdk/one"})
}

func (s *hookSuite) TestHookFailed(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	newSdk := workshopbackend.SdkRecord{Name: "one", Channel: "latest/stable"}
	t1 := hookstate.SetupHook(s.state, &newSdk, hookstate.SaveState)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)

	s.launchWorkshop(c, newSdk)
	s.backend.DoExec = func(ctx context.Context, name string, args *workshopbackend.Execution) (workshopbackend.ExecContext, error) {
		return workshopbackend.ExecContext{
			WaitExecution: func(ctx context.Context) error {
				return errors.New("hook execution error")
			},
		}, nil
	}
	defer func() {
		s.backend.DoExec = workshopbackend.DoExecDefault
	}()

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()
	c.Assert(s.backend.ExecCalls, check.HasLen, 1)
	c.Assert(s.backend.ExecCalls[0].Args.Command, testutil.DeepUnsortedMatches,
		[]string{"bash", "-ue", "-o", "pipefail", "-c", "/var/lib/workshop/sdk/one/current/sdk/hooks/save-state"})

	// ensure that the save-state handler has created the required state directory
	ws, err := s.backend.WorkshopFs(s.ctx, "ws")
	c.Check(err, check.IsNil)
	info, err := ws.Stat("/var/lib/workshop/state/sdk/one")
	c.Check(err, check.IsNil)
	c.Assert(info.IsDir(), check.Equals, true)
	c.Assert(t1.Log(), check.HasLen, 1)
	c.Assert(t1.Log()[0], check.Matches, ".*hook execution error")
}

func (s *hookSuite) launchWorkshop(c *check.C, newSdk workshopbackend.SdkRecord) {
	err := os.WriteFile(filepath.Join(s.project.Path, ".workshop.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04
sdks:
  one:
    channel: latest/stable
`), 0644)
	c.Check(err, check.IsNil)
	err = s.backend.LaunchWorkshop(s.ctx, "ws", "ubuntu@20.04")
	c.Check(err, check.IsNil)
	ws, err := s.backend.WorkshopFs(s.ctx, "ws")
	c.Check(err, check.IsNil)
	err = ws.MkdirAll(sdk.SdkHooksDir(newSdk.Name), 0744)
	c.Check(err, check.IsNil)
	_, err = ws.Create(sdk.SdkHookPath(newSdk.Name, hookstate.SaveState.String()))
	c.Check(err, check.IsNil)
	_, err = ws.Create(sdk.SdkHookPath(newSdk.Name, hookstate.RestoreState.String()))
	c.Check(err, check.IsNil)
	_, err = ws.Create(sdk.SdkHookPath(newSdk.Name, hookstate.SetupBase.String()))
	c.Check(err, check.IsNil)
}
