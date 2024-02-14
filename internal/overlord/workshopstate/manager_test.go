package workshopstate_test

import (
	"context"
	"os"
	"path/filepath"

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

// func (s *ManagerSuite) setupWorkshop(running bool) *workshopbackend.Workshop {
// 	wrkspc := workshopbackend.NewWorkshop(s.backend, s.project, "ws")
// 	wrkspc.SetRunning(running)
// 	return wrkspc
// }

// func (s *ManagerSuite) TestWorkshopStateChanges(c *check.C) {
// 	type stateSetup struct {
// 		status            state.Status
// 		running           bool
// 		refreshInProgress bool
// 		hasErrors         bool
// 		expectedState     healthstate.HealthStatus
// 		expectedErrors    []workshopbackend.WorkshopErrorType
// 	}
// 	cases := []stateSetup{
// 		// running, no operation in progress, no errors
// 		{state.DefaultStatus, true, false, false, healthstate.ReadyStatus, nil},
// 		// running, no operation in prorgess, has errors
// 		{state.DefaultStatus, true, false, true, healthstate.ReadyStatus, []workshopbackend.WorkshopErrorType{workshopbackend.MissingFile}},
// 		// not running, no operation in prorgess, no errors
// 		{state.DefaultStatus, false, false, false, workshopbackend.WorkshopStopped, nil},
// 		// not running, no operation in prorgess, has errors
// 		{state.DefaultStatus, false, false, true, workshopbackend.WorkshopError, []workshopbackend.WorkshopErrorType{workshopbackend.MissingFile}},
// 		// running, has operation in prorgess, waits on error
// 		{state.WaitStatus, true, true, false, workshopbackend.WorkshopPending, []workshopbackend.WorkshopErrorType{workshopbackend.WaitOnError}},
// 		// running, has operation in prorgess, no errors
// 		{state.DoingStatus, true, true, false, workshopbackend.WorkshopPending, nil},
// 		// running, has operation in prorgess, has errors
// 		{state.DoingStatus, true, true, true, workshopbackend.WorkshopError, []workshopbackend.WorkshopErrorType{workshopbackend.MissingFile}},
// 		// not running, has operation in prorgess, no errors
// 		{state.DoingStatus, false, true, false, workshopbackend.WorkshopPending, nil},
// 		// not running, has operation in prorgess, has errors
// 		{state.DoingStatus, false, true, true, workshopbackend.WorkshopError, []workshopbackend.WorkshopErrorType{workshopbackend.MissingFile}},
// 	}

// 	s.state.Lock()
// 	defer s.state.Unlock()

// 	for i, setup := range cases {
// 		// setup
// 		wrkspc := s.setupWorkshop(setup.running)
// 		if setup.hasErrors {
// 			// add any error to emulate error state
// 			wrkspc.AddError(workshopbackend.MissingFile)
// 		}
// 		if setup.refreshInProgress {
// 			chg := s.state.NewChange("test", "...")
// 			chg.SetStatus(setup.status)
// 			err := statecontext.StartOperation(s.state, "ws", "42424242", statecontext.Operation{Operation: statecontext.OperationRefresh, ChangeId: chg.ID(), WaitOnError: true})
// 			c.Assert(err, check.IsNil)
// 		}

// 		// validate
// 		st := workshopstate.WorkshopStatus(s.manager, wrkspc)
// 		c.Assert(st, check.Equals, setup.expectedState, check.Commentf("case num: %v", i))
// 		c.Assert(wrkspc.Errors(), testutil.DeepUnsortedMatches, setup.expectedErrors, check.Commentf("case num: %v", i))
// 		if setup.refreshInProgress {
// 			err := statecontext.StopOperation(s.state, "ws", "42424242", statecontext.OperationRefresh)
// 			c.Assert(err, check.IsNil)
// 		}
// 	}
// }

func (s *ManagerSuite) TestRefreshSdkWasAdded(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	// Setup
	os.WriteFile(filepath.Join(s.project.Path, ".workshop.test.yaml"), []byte(`name: test
base: ubuntu@20.04
sdks:
  test-sdk:
    channel: latest/stable
`), 0644)
	err := s.backend.LaunchWorkshop(s.ctx, "test", "ubuntu@20.04")
	c.Assert(err, check.IsNil)

	// a user added an SDK to the workshop file and called refresh
	err = os.WriteFile(filepath.Join(s.project.Path, ".workshop.test.yaml"), []byte(`name: test
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
	os.WriteFile(filepath.Join(s.project.Path, ".workshop.test.yaml"), []byte(`name: test
base: ubuntu@20.04
sdks:
  test-sdk:
    channel: latest/stable
`), 0644)
	err := s.backend.LaunchWorkshop(s.ctx, "test", "ubuntu@20.04")
	c.Assert(err, check.IsNil)

	// a user removed an SDK in the workshop file and called refresh
	err = os.WriteFile(filepath.Join(s.project.Path, ".workshop.test.yaml"), []byte(`name: test
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
	os.WriteFile(filepath.Join(s.project.Path, ".workshop.test.yaml"), []byte(`name: test
base: ubuntu@20.04
sdks:
  test-sdk:
    channel: latest/stable
`), 0644)
	err := s.backend.LaunchWorkshop(s.ctx, "test", "ubuntu@20.04")
	c.Assert(err, check.IsNil)

	// a user updated an SDK in the workshop file and called refresh
	err = os.WriteFile(filepath.Join(s.project.Path, ".workshop.test.yaml"), []byte(`name: test
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
