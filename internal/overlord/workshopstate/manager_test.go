package workshopstate_test

import (
	"context"
	"os"
	"path/filepath"

	"github.com/canonical/workshop/internal/overlord/healthstate"
	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/operation"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/workshopstate"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
	"gopkg.in/check.v1"
)

type ManagerSuite struct {
	state   *state.State
	backend workshopbackend.WorkshopBackend
	runner  *state.TaskRunner
	manager *workshopstate.WorkshopManager
	ctx     context.Context
	project *workshopbackend.Project
}

var _ = check.Suite(&ManagerSuite{})

func (s *ManagerSuite) SetUpTest(c *check.C) {
	s.state = state.New(nil)
	s.backend = workshopbackend.NewFakeWorkshopBackend()
	s.runner = state.NewTaskRunner(s.state)
	s.manager = workshopstate.New(s.state, s.runner, s.backend)
	ctx := context.WithValue(context.TODO(), workshopbackend.ContextUser, "testuser")
	s.project, _, _ = s.backend.CreateOrLoadProject(ctx, c.MkDir())
	s.ctx = context.WithValue(ctx, workshopbackend.ContextProjectId, s.project.ProjectId)
}

func (s *ManagerSuite) TestAddHandlers(c *check.C) {
	workshopstate.New(s.state, s.runner, s.backend)

	c.Assert(s.runner.KnownTaskKinds(), testutil.DeepUnsortedMatches, []string{
		"create-workshop",
		"start-workshop",
		"stop-workshop",
		"remove-workshop",
		"mount-project",
		"remove-workshop-stash",
		"stash-workshop",
		"create-state-storage",
		"remove-state-storage",
	})
}

func (s *ManagerSuite) launchWorkshopWithTestSdk(c *check.C) *workshopbackend.Workshop {
	// Setup
	os.WriteFile(filepath.Join(s.project.Path, ".workshop.test.yaml"), []byte(`name: test
base: ubuntu@20.04
sdks:
  test-sdk:
    channel: latest/stable
`), 0644)
	err := s.backend.LaunchWorkshop(s.ctx, "test", "ubuntu@20.04")
	c.Assert(err, check.IsNil)
	ws, err := s.backend.Workshop(s.ctx, "test")
	c.Assert(err, check.IsNil)
	return ws
}

func (s *ManagerSuite) TestWorkshopHealthReady(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	workshop := s.launchWorkshopWithTestSdk(c)
	health := s.manager.WorkshopHealth(workshop)

	c.Assert(health.Status, check.Equals, healthstate.ReadyStatus)
	c.Check(health.SdkHealth, check.HasLen, 0)
	c.Check(health.Message, check.HasLen, 0)
	c.Check(health.Code, check.HasLen, 0)
}

func (s *ManagerSuite) TestWorkshopHealthStopped(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	workshop := s.launchWorkshopWithTestSdk(c)
	c.Assert(s.backend.StopWorkshop(s.ctx, "test", true), check.IsNil)
	health := s.manager.WorkshopHealth(workshop)

	c.Assert(health.Status, check.Equals, healthstate.StoppedStatus)
	c.Check(health.SdkHealth, check.HasLen, 0)
	c.Check(health.Message, check.HasLen, 0)
	c.Check(health.Code, check.HasLen, 0)
}

func (s *ManagerSuite) TestWorkshopHealthMissingProject(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	workshop := s.launchWorkshopWithTestSdk(c)
	c.Assert(os.RemoveAll(s.project.Path), check.IsNil)
	health := s.manager.WorkshopHealth(workshop)

	c.Assert(health.Status, check.Equals, healthstate.ErrorStatus)
	c.Check(health.SdkHealth, check.HasLen, 0)
	c.Check(health.Message, check.HasLen, 0)
	c.Check(health.Code, check.Equals, "missing-project")
}

func (s *ManagerSuite) TestWorkshopHealthMissingFile(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	workshop := s.launchWorkshopWithTestSdk(c)
	c.Assert(os.RemoveAll(filepath.Join(s.project.Path, ".workshop.test.yaml")), check.IsNil)
	health := s.manager.WorkshopHealth(workshop)

	c.Assert(health.Status, check.Equals, healthstate.ErrorStatus)
	c.Check(health.SdkHealth, check.HasLen, 0)
	c.Check(health.Message, check.HasLen, 0)
	c.Check(health.Code, check.Equals, "missing-file")
}

func (s *ManagerSuite) TestWorkshopHealthOperationInProgress(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	chg := s.state.NewChange("launch", "test")

	workshop := s.launchWorkshopWithTestSdk(c)
	err := operation.StartOperation(s.state, "test", s.project.ProjectId, operation.Operation{
		ChangeId:  chg.ID(),
		Operation: "launch",
	})
	c.Assert(err, check.IsNil)
	health := s.manager.WorkshopHealth(workshop)

	c.Assert(health.Status, check.Equals, healthstate.PendingStatus)
	c.Check(health.SdkHealth, check.HasLen, 0)
	c.Check(health.Message, check.HasLen, 0)
	c.Check(health.Code, check.HasLen, 0)
}

func (s *ManagerSuite) TestWorkshopHealthOperationInProgressWithNotes(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	chg := s.state.NewChange("refresh", "test")
	chg.SetStatus(state.WaitStatus)

	workshop := s.launchWorkshopWithTestSdk(c)
	err := operation.StartOperation(s.state, "test", s.project.ProjectId, operation.Operation{
		ChangeId:  chg.ID(),
		Operation: "refresh",
	})
	c.Assert(err, check.IsNil)
	health := s.manager.WorkshopHealth(workshop)

	c.Assert(health.Status, check.Equals, healthstate.PendingStatus)
	c.Check(health.SdkHealth, check.HasLen, 0)
	c.Check(health.Message, check.HasLen, 0)
	c.Check(health.Code, check.Equals, "wait-on-error")
}

func (s *ManagerSuite) TestWorkshopHealthSdkHealth(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	chg := s.state.NewChange("launch", "test")
	task := s.state.NewTask("run-hook", "test task")
	healthCheck := healthstate.HealthCheck{
		Sdk:         "test",
		CheckResult: healthstate.CheckWaiting,
		Message:     "still waiting",
		Code:        "how-much-longer",
	}
	task.Set("health", healthCheck)
	chg.AddTask(task)

	workshop := s.launchWorkshopWithTestSdk(c)
	err := operation.StartOperation(s.state, "test", s.project.ProjectId, operation.Operation{
		ChangeId:  chg.ID(),
		Operation: "launch",
	})
	c.Assert(err, check.IsNil)
	health := s.manager.WorkshopHealth(workshop)

	c.Assert(health.Status, check.Equals, healthstate.PendingStatus)
	c.Assert(health.SdkHealth, check.HasLen, 1)
	c.Assert(health.SdkHealth["test"], check.DeepEquals, healthCheck)
	c.Check(health.Message, check.HasLen, 0)
	c.Check(health.Code, check.HasLen, 0)
}

func (s *ManagerSuite) TestRefreshSdkWasAdded(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	// Setup
	s.launchWorkshopWithTestSdk(c)

	// a user added an SDK to the workshop file and called refresh
	err := os.WriteFile(filepath.Join(s.project.Path, ".workshop.test.yaml"), []byte(`name: test
base: ubuntu@20.04
sdks:
  test-sdk:
    channel: latest/stable
  new:
    channel: latest/stable
`), 0644)
	c.Check(err, check.IsNil)

	// Execute
	ts, err := s.manager.RefreshMany(s.ctx, []string{"test"}, s.project.ProjectId, operation.RefreshTransactional, "1")
	c.Check(err, check.IsNil)

	// Validate
	s.validateStateHooksTasksSetup(c, ts, []string{"test-sdk"}, []string{"test-sdk"})
}

func (s *ManagerSuite) TestRefreshSdkWasRemoved(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	// Setup
	s.launchWorkshopWithTestSdk(c)

	// a user removed an SDK in the workshop file and called refresh
	err := os.WriteFile(filepath.Join(s.project.Path, ".workshop.test.yaml"), []byte(`name: test
base: ubuntu@20.04
`), 0644)
	c.Check(err, check.IsNil)

	// Execute
	ts, err := s.manager.RefreshMany(s.ctx, []string{"test"}, s.project.ProjectId, operation.RefreshTransactional, "1")
	c.Check(err, check.IsNil)

	// Validate
	s.validateStateHooksTasksSetup(c, ts, []string{}, []string{})
}

func (s *ManagerSuite) TestRefreshSdkChannelWasUpdated(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	// Setup
	s.launchWorkshopWithTestSdk(c)

	// a user updated an SDK in the workshop file and called refresh
	err := os.WriteFile(filepath.Join(s.project.Path, ".workshop.test.yaml"), []byte(`name: test
base: ubuntu@20.04
sdks:
  test-sdk:
    channel: latest/edge
`), 0644)
	c.Check(err, check.IsNil)

	// Execute
	ts, err := s.manager.RefreshMany(s.ctx, []string{"test"}, s.project.ProjectId, operation.RefreshTransactional, "1")
	c.Check(err, check.IsNil)

	// Validate
	s.validateStateHooksTasksSetup(c, ts, []string{"test-sdk"}, []string{"test-sdk"})
}

// the save state shall be called only for the previously installed SDK
// the restore state shall be called for both, the old and the new SDK
func (*ManagerSuite) validateStateHooksTasksSetup(c *check.C, ts []*state.TaskSet, expectedSave, expectedRestore []string) {
	obtainedSave := []string{}
	obtainedRestore := []string{}
	for _, t := range ts[0].Tasks() {
		if t.Kind() == "run-hook" {
			var setup hookstate.HookSetup
			err := t.Get("hook-setup", &setup)
			c.Assert(err, check.IsNil)
			switch setup.HookType {
			case hookstate.SaveState:
				obtainedSave = append(obtainedSave, setup.Sdk)
			case hookstate.RestoreState:
				obtainedRestore = append(obtainedRestore, setup.Sdk)
			}
		}
	}

	// the save state shall be called only for the previously installed SDK
	c.Assert(obtainedSave, testutil.DeepUnsortedMatches, expectedSave)
	// the restore state shall be called for the new previously installed SDK
	c.Assert(obtainedRestore, testutil.DeepUnsortedMatches, expectedRestore)
}
