package workshopstate_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/canonical/workshop/internal/overlord"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/workshopstate"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
	"github.com/spf13/afero"
	"gopkg.in/check.v1"
	"gopkg.in/tomb.v2"
)

type workshopHandlers struct {
	fs      afero.Fs
	backend *workshopbackend.FakeWorkshopBackend
	state   *state.State
	runner  *state.TaskRunner
	se      *overlord.StateEngine
	wrkmgr  *workshopstate.WorkshopManager
	ctx     context.Context
	project *workshopbackend.Project
}

var _ = check.Suite(&workshopHandlers{})

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

func (s *workshopHandlers) SetUpTest(c *check.C) {
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
	s.wrkmgr = workshopstate.New(s.state, s.runner, s.backend)

	// error-provoking task handler
	erroringHandler := func(task *state.Task, _ *tomb.Tomb) error {
		return ErrTrigger
	}
	s.runner.AddHandler("error-trigger", erroringHandler, nil)

	s.se = overlord.NewStateEngine(s.state)
	s.se.AddManager(s.wrkmgr)
	s.se.AddManager(s.runner)
	err = s.se.StartUp()
	c.Check(err, check.IsNil)
}

func (s *workshopHandlers) TearDownTest(c *check.C) {
}

func (s *workshopHandlers) TestStopPeriodicProgressUpdate(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	err := os.WriteFile(filepath.Join(s.project.Path, ".workshop.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04
`), 0644)
	c.Check(err, check.IsNil)

	err = s.backend.LaunchWorkshop(s.ctx, "ws", "ubuntu@20.04")
	c.Check(err, check.IsNil)

	t1, err := s.wrkmgr.StopMany(s.ctx, []string{"ws"}, s.project.ProjectId, "1")
	c.Check(err, check.IsNil)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t1.Tasks()...)
	chg.Set("user", "testuser")
	chg.AddAll(t1)

	oldInterval := workshopstate.StopLogInterval
	workshopstate.StopLogInterval = 100 * time.Millisecond

	restore := testutil.FakeFunc(func(_ workshopbackend.WorkshopBackend, ctx context.Context, name string, force bool) error {
		time.Sleep(150 * time.Millisecond)
		return nil
	}, &workshopstate.StopWorkshop)
	defer restore()

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	c.Assert(t1.Tasks()[0].Log()[0], check.Matches, ".*Still waiting for \"ws\" to stop; no change in the last 30 seconds...")
	c.Assert(t1.Tasks()[0].Log(), check.HasLen, 1)
	c.Check(chg.Err(), check.Equals, nil)
	workshopstate.StopLogInterval = oldInterval
}

func (s *workshopHandlers) TestUndoStash(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	err := os.WriteFile(filepath.Join(s.project.Path, ".workshop.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04
sdks:
  test:
    channel: latest/stable
  test2:
    channel: latest/edge
`), 0644)
	c.Check(err, check.IsNil)

	err = s.backend.LaunchWorkshop(s.ctx, "ws", "ubuntu@20.04")
	c.Check(err, check.IsNil)

	chg := s.state.NewChange("sample", "...")
	t1 := s.state.NewTask("stash-workshop", "...")
	t2 := s.state.NewTask("error-trigger", "...")
	t2.WaitFor(t1)
	setWorkshopProject("ws", s.project, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	chg.AddTask(t2)

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	c.Assert(t1.Status(), check.Equals, state.UndoneStatus)
	lane1 := chg.LaneTasks(1)
	c.Assert(lane1, check.HasLen, 1)
	c.Assert(lane1[0].Kind(), check.Equals, "auto-connect")

	lane2 := chg.LaneTasks(2)
	c.Assert(lane2, check.HasLen, 1)
	c.Assert(lane2[0].Kind(), check.Equals, "auto-connect")

	for _, t := range append(lane1, lane2...) {
		var name string
		c.Assert(t.Get("sdk", &name), check.IsNil)
		c.Assert(name, check.Matches, "^test.?$")

		c.Assert(t.Get("workshop", &name), check.IsNil)
		c.Assert(name, check.Equals, "ws")

		var prj workshopbackend.Project
		c.Assert(t.Get("project", &prj), check.IsNil)
		c.Assert(prj, check.DeepEquals, *s.project)
	}
	c.Assert(lane1[0].HaltTasks(), testutil.DeepUnsortedMatches, lane2[0:])
	c.Assert(lane1[0].WaitTasks(), testutil.DeepUnsortedMatches, []*state.Task{})
}
