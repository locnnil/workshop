package workshopstate_test

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/canonical/workshop/internal/overlord/healthstate"
	"github.com/canonical/workshop/internal/overlord/operation"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/workshopstate"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
	"gopkg.in/check.v1"
)

type managerSuite struct {
	state   *state.State
	backend workshopbackend.WorkshopBackend
	runner  *state.TaskRunner
	manager *workshopstate.WorkshopManager
	ctx     context.Context
	project *workshopbackend.Project
}

var _ = check.Suite(&managerSuite{})

func (s *managerSuite) SetUpTest(c *check.C) {
	s.state = state.New(nil)
	s.backend = workshopbackend.NewFakeWorkshopBackend()
	s.runner = state.NewTaskRunner(s.state)
	s.manager = workshopstate.New(s.state, s.runner, s.backend)
	ctx := context.WithValue(context.TODO(), workshopbackend.ContextUser, "testuser")
	s.project, _, _ = s.backend.CreateOrLoadProject(ctx, c.MkDir())
	s.ctx = context.WithValue(ctx, workshopbackend.ContextProjectId, s.project.ProjectId)
}

func (s *managerSuite) TestAddHandlers(c *check.C) {
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

func (s *managerSuite) launchWorkshopWithSDKs(c *check.C, ws string, sdks []sdk.Setup) *workshopbackend.Workshop {
	t, err := template.New("workshop").Parse(fmt.Sprintf(workshopTemplate, ws))
	c.Assert(err, check.IsNil)

	var workshopFile = bytes.NewBuffer([]byte{})
	t.Execute(workshopFile, sdks)

	err = os.WriteFile(filepath.Join(s.project.Path, fmt.Sprintf(".workshop.%s.yaml", ws)), workshopFile.Bytes(), 0644)
	c.Assert(err, check.IsNil)

	err = s.backend.LaunchWorkshop(s.ctx, ws, "ubuntu@20.04")
	c.Assert(err, check.IsNil)

	workshop, err := s.backend.Workshop(s.ctx, ws)
	c.Assert(err, check.IsNil)
	return workshop
}

func (s *managerSuite) TestWorkshopManagerStartOperationOK(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	chg := s.state.NewChange("refresh", "test")
	s.launchWorkshopWithSDKs(c, "test-1", []sdk.Setup{{Name: "test", Channel: "latest/stable"}})
	s.launchWorkshopWithSDKs(c, "test-2", []sdk.Setup{{Name: "test", Channel: "latest/stable"}})

	err := workshopstate.StartOperation(s.manager, []string{"test-1", "test-2"}, s.project.ProjectId, operation.Operation{Operation: operation.OperationRefresh, ChangeId: chg.ID()})
	c.Assert(err, check.IsNil)
	c.Assert(operation.OperationInProgress(s.state, "test-1", s.project.ProjectId), check.NotNil)
	c.Assert(operation.OperationInProgress(s.state, "test-2", s.project.ProjectId), check.NotNil)
}

func (s *managerSuite) TestWorkshopManagerStartOperationFail(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	chg := s.state.NewChange("refresh", "test")
	s.launchWorkshopWithSDKs(c, "test-2", []sdk.Setup{{Name: "test", Channel: "latest/stable"}})
	_, err := s.manager.RefreshMany(s.ctx, []string{"test-2"}, s.project.ProjectId, operation.RefreshTransactional, chg.ID())
	c.Assert(err, check.IsNil)

	err = workshopstate.StartOperation(s.manager, []string{"test-1", "test-2"}, s.project.ProjectId, operation.Operation{Operation: operation.OperationRefresh, ChangeId: chg.ID()})
	c.Assert(err, check.ErrorMatches, `cannot refresh: refresh operation is in progress`)
	c.Assert(operation.OperationInProgress(s.state, "test-1", s.project.ProjectId), check.IsNil)
}

func (s *managerSuite) TestWorkshopHealthReady(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	workshop := s.launchWorkshopWithSDKs(c, "test", nil)
	health := s.manager.WorkshopHealth(workshop)

	c.Assert(health.Status, check.Equals, healthstate.ReadyStatus)
	c.Check(health.SdkHealth, check.HasLen, 0)
	c.Check(health.Message, check.HasLen, 0)
	c.Check(health.Code, check.HasLen, 0)
}

func (s *managerSuite) TestWorkshopHealthStopped(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	workshop := s.launchWorkshopWithSDKs(c, "test", nil)
	c.Assert(s.backend.StopWorkshop(s.ctx, "test", true), check.IsNil)
	health := s.manager.WorkshopHealth(workshop)

	c.Assert(health.Status, check.Equals, healthstate.StoppedStatus)
	c.Check(health.SdkHealth, check.HasLen, 0)
	c.Check(health.Message, check.HasLen, 0)
	c.Check(health.Code, check.HasLen, 0)
}

func (s *managerSuite) TestWorkshopHealthMissingProject(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	workshop := s.launchWorkshopWithSDKs(c, "test", nil)
	c.Assert(os.RemoveAll(s.project.Path), check.IsNil)
	health := s.manager.WorkshopHealth(workshop)

	c.Assert(health.Status, check.Equals, healthstate.ErrorStatus)
	c.Check(health.SdkHealth, check.HasLen, 0)
	c.Check(health.Message, check.HasLen, 0)
	c.Check(health.Code, check.Equals, "missing-project")
}

func (s *managerSuite) TestWorkshopHealthMissingFile(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	workshop := s.launchWorkshopWithSDKs(c, "test", nil)
	c.Assert(os.RemoveAll(filepath.Join(s.project.Path, ".workshop.test.yaml")), check.IsNil)
	health := s.manager.WorkshopHealth(workshop)

	c.Assert(health.Status, check.Equals, healthstate.ErrorStatus)
	c.Check(health.SdkHealth, check.HasLen, 0)
	c.Check(health.Message, check.HasLen, 0)
	c.Check(health.Code, check.Equals, "missing-file")
}

func (s *managerSuite) TestWorkshopHealthOperationInProgress(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	chg := s.state.NewChange("launch", "test")

	workshop := s.launchWorkshopWithSDKs(c, "test", nil)
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

func (s *managerSuite) TestWorkshopHealthOperationInProgressWithNotes(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	chg := s.state.NewChange("refresh", "test")
	chg.SetStatus(state.WaitStatus)

	workshop := s.launchWorkshopWithSDKs(c, "test", nil)
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

func (s *managerSuite) TestWorkshopHealthSdkHealth(c *check.C) {
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

	workshop := s.launchWorkshopWithSDKs(c, "test", []sdk.Setup{{Name: "test", Channel: "latest/stable"}})
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

func (s *managerSuite) TestRefreshManyOK(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	chg := s.state.NewChange("refresh", "test")
	s.launchWorkshopWithSDKs(c, "test-1", []sdk.Setup{{Name: "test", Channel: "latest/stable"}})
	s.launchWorkshopWithSDKs(c, "test-2", []sdk.Setup{{Name: "test", Channel: "latest/stable"}})

	_, err := s.manager.RefreshMany(s.ctx, []string{"test-1", "test-2"}, s.project.ProjectId, operation.RefreshTransactional, chg.ID())
	c.Assert(err, check.IsNil)

	op := operation.OperationInProgress(s.state, "test-1", s.project.ProjectId)
	c.Check(op, check.NotNil)
	c.Assert(op.Operation, check.Equals, "refresh")
	c.Assert(op.ChangeId, check.Equals, chg.ID())
	c.Assert(op.WaitOnError, check.Equals, false)

	op = operation.OperationInProgress(s.state, "test-2", s.project.ProjectId)
	c.Check(op, check.NotNil)
	c.Assert(op.Operation, check.Equals, "refresh")
	c.Assert(op.ChangeId, check.Equals, chg.ID())
	c.Assert(op.WaitOnError, check.Equals, false)
}

func (s *managerSuite) TestRefreshManyWorkshopHasOperationPending(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	chg := s.state.NewChange("refresh", "test")
	chg2 := s.state.NewChange("refresh", "test")
	s.launchWorkshopWithSDKs(c, "test-1", []sdk.Setup{{Name: "test", Channel: "latest/stable"}})
	s.launchWorkshopWithSDKs(c, "test-2", []sdk.Setup{{Name: "test", Channel: "latest/stable"}})

	operation.StartOperation(s.state, "test-1", s.project.ProjectId, operation.Operation{Operation: operation.OperationRefresh, ChangeId: chg.ID()})

	_, err := s.manager.RefreshMany(s.ctx, []string{"test-1", "test-2"}, s.project.ProjectId, operation.RefreshTransactional, chg2.ID())
	c.Assert(err, check.ErrorMatches, `cannot refresh: "test-1" status is "Pending", must be one of: "Ready"`)
	c.Assert(operation.OperationInProgress(s.state, "test-2", s.project.ProjectId), check.IsNil)
}

func (s *managerSuite) TestRefreshRequireStatusReady(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	chg := s.state.NewChange("refresh", "test")
	s.launchWorkshopWithSDKs(c, "test-1", []sdk.Setup{{Name: "test", Channel: "latest/stable"}})
	workshop2 := s.launchWorkshopWithSDKs(c, "test-2", []sdk.Setup{{Name: "test", Channel: "latest/stable"}})
	err := s.backend.StopWorkshop(s.ctx, workshop2.Name, true)
	c.Assert(err, check.IsNil)

	_, err = s.manager.RefreshMany(s.ctx, []string{"test-1", "test-2"}, s.project.ProjectId, operation.RefreshTransactional, chg.ID())
	c.Assert(err, check.ErrorMatches, `cannot refresh: "test-2" status is "Stopped", must be one of: "Ready"`)
	c.Assert(operation.OperationInProgress(s.state, "test-1", s.project.ProjectId), check.IsNil)
	c.Assert(operation.OperationInProgress(s.state, "test-2", s.project.ProjectId), check.IsNil)
}

func (s *managerSuite) TestRefreshRequireWorkshopExistance(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	chg := s.state.NewChange("refresh", "test")
	s.launchWorkshopWithSDKs(c, "test-1", []sdk.Setup{{Name: "test", Channel: "latest/stable"}})

	_, err := s.manager.RefreshMany(s.ctx, []string{"test-1", "test-2"}, s.project.ProjectId, operation.RefreshTransactional, chg.ID())
	c.Assert(err, check.ErrorMatches, `cannot refresh: status check for "test-2" failed \(workshop not found\)`)
	c.Assert(operation.OperationInProgress(s.state, "test-1", s.project.ProjectId), check.IsNil)
	c.Assert(operation.OperationInProgress(s.state, "test-2", s.project.ProjectId), check.IsNil)
}
