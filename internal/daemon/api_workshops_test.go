package daemon

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/overlord/healthstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/workshopstate"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
)

func (s *apiSuite) launchWorkshop(ctx context.Context, name string, c *check.C) *workshopbackend.Workshop {
	b := s.d.overlord.WorkshopBackend()
	err := os.WriteFile(filepath.Join(s.project.Path, fmt.Sprintf(`.workshop.%s.yaml`, name)), []byte(fmt.Sprintf(`name: %s
base: ubuntu@20.04
`, name)), 0644)
	c.Assert(err, check.IsNil)
	err = b.LaunchWorkshop(ctx, name, "ubuntu@20.04")
	c.Assert(err, check.IsNil)
	ws, err := b.Workshop(ctx, name)
	c.Assert(err, check.IsNil)
	return ws
}

func (s *apiSuite) TestProjectsGetWorkshops(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workshops")
	s.vars = map[string]string{"id": s.project.ProjectId}
	req, err := s.createProjectsRequest("GET", "/v1/projects/"+s.project.ProjectId+"/workshops", nil)
	c.Assert(err, check.IsNil)

	restore := testutil.FakeFunc(func() time.Time { return time.Date(2023, 04, 25, 1, 2, 3, 0, time.UTC) }, &workshopbackend.InstallTimeNow)
	defer restore()

	workshop := s.launchWorkshop(s.ctx, "ws-test", c)
	workshop.LinkSdk(s.ctx, sdk.Setup{Name: "go", Channel: "latest/stable", Revision: 234})

	// Execute
	rsp := v1GetProjectWorkshops(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	_, err = rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(rsp.Result, check.DeepEquals, []*WorkshopInfo{
		{
			Name:      "ws-test",
			Base:      "ubuntu@20.04",
			ProjectId: s.project.ProjectId,
			Status:    "Ready",
			Content: []*SdkInfo{
				{
					Name:        "go",
					Channel:     "latest/stable",
					Revision:    "234",
					InstallTime: time.Date(2023, 04, 25, 1, 2, 3, 0, time.UTC),
				},
			},
			Notes: nil,
		},
	})
}

func (s *apiSuite) TestProjectsGetWorkshop(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workshops/{name}")
	s.vars = map[string]string{"id": s.project.ProjectId, "name": "ws-test"}
	req, err := s.createProjectsRequest("GET", "/v1/projects/"+s.project.ProjectId+"/workshops/ws-test", nil)
	c.Assert(err, check.IsNil)

	workshop := s.launchWorkshop(s.ctx, "ws-test", c)
	restoreTime := testutil.FakeFunc(func() time.Time { return time.Date(2023, 04, 25, 1, 2, 3, 0, time.UTC) }, &workshopbackend.InstallTimeNow)
	workshop.LinkSdk(s.ctx, sdk.Setup{Name: "go", Channel: "latest/stable", Revision: 234})
	workshop.LinkSdk(s.ctx, sdk.Setup{Name: "java", Channel: "latest/stable", Revision: 324})
	restoreTime()

	// Execute
	restoreHealth := FakeWorkshopHealth(func(mgr *workshopstate.WorkshopManager, w *workshopbackend.Workshop) healthstate.HealthState {
		return healthstate.HealthState{Status: healthstate.ReadyStatus, SdkHealth: map[string]healthstate.HealthCheck{
			"go": {Sdk: "go", Message: "test health check message", Code: "check-waiting", CheckResult: healthstate.CheckWaiting},
		}}
	})
	restoreMounts := FakeSdkMounts(func(st *state.State, repo *interfaces.Repository, projectId, workshop, sdk string) []*Mount {
		return []*Mount{
			{Source: "/home/user/" + sdk, Target: "/home/workshop/" + sdk, Plug: interfaces.PlugRef{ProjectId: projectId, Workshop: workshop, Sdk: sdk, Name: "content-plug"}},
		}
	})

	rsp := v1GetProjectWorkshop(projectsCmd, req, nil).(*resp)

	restoreHealth()
	restoreMounts()

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	_, err = rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(rsp.Result, check.DeepEquals, &WorkshopInfo{
		Name:      "ws-test",
		Base:      "ubuntu@20.04",
		ProjectId: s.project.ProjectId,
		Status:    "Ready",
		Notes:     nil,
		Content: []*SdkInfo{
			{
				Name:        "go",
				Channel:     "latest/stable",
				Revision:    "234",
				InstallTime: time.Date(2023, 04, 25, 1, 2, 3, 0, time.UTC),
				Health: &HealthCheckInfo{
					Message: "test health check message",
					Code:    "check-waiting",
				},
				Mounts: []*Mount{
					{Source: "/home/user/go", Target: "/home/workshop/go", Plug: interfaces.PlugRef{ProjectId: s.project.ProjectId, Workshop: "ws-test", Sdk: "go", Name: "content-plug"}}},
			},
			{
				Name:        "java",
				Channel:     "latest/stable",
				Revision:    "324",
				InstallTime: time.Date(2023, 04, 25, 1, 2, 3, 0, time.UTC),
				Mounts: []*Mount{
					{Source: "/home/user/java", Target: "/home/workshop/java", Plug: interfaces.PlugRef{ProjectId: s.project.ProjectId, Workshop: "ws-test", Sdk: "java", Name: "content-plug"}}},
			},
		},
	})
}

type expectedResp struct {
	Type    ResponseType
	Status  int
	Message string
	Kind    string
	Summary string
}

func (s *apiSuite) runActionTest(c *check.C, buffers []*bytes.Buffer, expected []*expectedResp, ens func(st *state.State, d time.Duration)) {
	s.daemon(c)
	s.vars = map[string]string{"id": s.project.ProjectId}
	projectsCmd := apiCmd("/v1/projects/{id}/workshops")
	requests := []*http.Request{}

	for _, i := range buffers {
		req, err := s.createProjectsRequest("POST", "/v1/projects/"+s.project.ProjectId+"/workshops", i)
		c.Assert(err, check.IsNil)
		requests = append(requests, req)
	}

	restoreEnsure := testutil.FakeFunc(ens, &ensureStateSoon)
	defer restoreEnsure()

	s.launchWorkshop(s.ctx, "ws", c)
	s.launchWorkshop(s.ctx, "ws1", c)

	for num, req := range requests {
		// Execute
		rsp := v1PostProjectWorkshop(projectsCmd, req, nil).(*resp)

		// Verify
		c.Check(rsp.Type, check.Equals, expected[num].Type)
		c.Assert(rsp.Status, check.Equals, expected[num].Status, check.Commentf("case: %v", num))
		if rsp.Type == ResponseTypeError {
			c.Assert(rsp.Result.(*errorResult).Message, check.Equals, expected[num].Message)
		}

		if rsp.Type == ResponseTypeAsync {
			st := s.d.state
			st.Lock()
			change := s.d.state.Change(rsp.Change)
			st.Unlock()
			c.Assert(change, check.NotNil)
			c.Assert(change.Kind(), check.Equals, expected[num].Kind)
			c.Assert(change.Summary(), check.Equals, expected[num].Summary)
		}
	}
}

func (s *apiSuite) TestProjectsPostProjectWorkshopLaunch(c *check.C) {
	// Setup

	name := "workshop"
	err := os.WriteFile(filepath.Join(s.project.Path, fmt.Sprintf(`.workshop.%s.yaml`, name)), []byte(fmt.Sprintf(`name: %s
base: ubuntu@20.04
`, name)), 0644)
	c.Check(err, check.IsNil)

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["workshop", "workshop", "workshop"],"action":"launch"}`),
		bytes.NewBufferString(`{"names":[],"action":"launch"}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "workshop" workshop`,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "cannot launch: at least one workshop name must be provided",
		},
		{
			Type:    ResponseTypeError,
			Message: "cannot launch: \"ws\" already exists",
			Status:  http.StatusBadRequest,
		},
	}

	soon := 0
	ensure := func(st *state.State, d time.Duration) {
		soon++
	}

	s.runActionTest(c, requests, expected, ensure)

	// all successful responses must initiate the ensure call
	c.Assert(soon, check.Equals, 1)
}

func (s *apiSuite) TestProjectsRefreshWorkshopIncorrectInput(c *check.C) {
	// Setup
	requests := []*bytes.Buffer{
		// try continue without starting wait-on-error
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh", "options": {"refresh-mode":"continue"}}`),

		// unknown refresh option
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh", "options": {"refresh-mode":"unknown"}}`),

		// a workshop name is a must
		bytes.NewBufferString(`{"names":[],"action":"refresh"}`),

		// non-transactional refresh is only supported for a single workshop
		bytes.NewBufferString(`{"names":["ws", "ws1"],"action":"refresh","options": {"refresh-mode":"wait-on-error"}}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "cannot continue, no refresh in progress",
		}, {
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `cannot refresh: refresh mode must be any of: "transactional", "wait-on-error", "continue", "abort"`,
		}, {
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "cannot refresh: at least one workshop name must be provided",
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "wait-on-error is not supported for multiple workshops",
		},
	}

	s.runActionTest(c, requests, expected, func(st *state.State, d time.Duration) {})
}

func (s *apiSuite) TestProjectsRefreshWorkshopContinue(c *check.C) {
	// Setup
	requests := []*bytes.Buffer{
		// start - continue (success) - continue (fail, already finished)
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"wait-on-error"}}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"continue"}}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "refresh",
			Summary: `Refresh "ws" workshop`,
		},
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "refresh",
			Summary: `Refresh "ws" workshop`,
		},
	}
	s.runActionTest(c, requests, expected, func(st *state.State, d time.Duration) {
		chg := s.d.state.Change("1")
		chg.SetStatus(state.WaitStatus)
	})
}

func (s *apiSuite) TestProjectsRefreshWorkshopContinueAbortNoRefreshInProgress(c *check.C) {
	// Setup
	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"continue"}}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"abort"}}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "cannot continue, no refresh in progress",
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "cannot abort, no refresh in progress",
		},
	}

	s.runActionTest(c, requests, expected, func(st *state.State, d time.Duration) {})
}

func (s *apiSuite) TestProjectsRefreshWorkshopTransactional(c *check.C) {
	// Setup
	requests := []*bytes.Buffer{
		// start transactional (success) - attempt abort or continue (failure)
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"transactional"}}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "refresh",
			Summary: `Refresh "ws" workshop`,
		},
	}

	soon := 0
	s.runActionTest(c, requests, expected, func(st *state.State, d time.Duration) { soon++ })
	c.Assert(soon, check.Equals, 1)
}

func (s *apiSuite) TestProjectsRefreshWorkshopRefreshAbort(c *check.C) {
	// Setup
	requests := []*bytes.Buffer{

		// start - abort (both success)
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"wait-on-error"}}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"abort"}}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "refresh",
			Summary: `Refresh "ws" workshop`,
		},
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "refresh",
			Summary: `Refresh "ws" workshop`,
		},
	}

	soon := 0
	s.runActionTest(c, requests, expected, func(st *state.State, d time.Duration) {
		chg := s.d.state.Change("1")
		chg.SetStatus(state.WaitStatus)
		soon++
	})
	c.Assert(soon, check.Equals, 2)
}

func (s *apiSuite) TestProjectsPostProjectWorkshopStart(c *check.C) {
	// Setup
	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["ws"],"action":"stop"}`),
		//
		bytes.NewBufferString(`{"names":["ws"],"action":"start"}`),
		// a second attempt to start the workshop that is already in Pending (i.e. being started)
		bytes.NewBufferString(`{"names":["ws"],"action":"start"}`),
		// ensure another operation is not going to work when the workshop in Pending
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh", "options": {"refresh-mode":"transactional"}}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "stop",
			Summary: `Stop "ws" workshop`,
		},
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "start",
			Summary: `Start "ws" workshop`,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `cannot start: "ws" status is "Pending", must be one of: "Stopped"`,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `cannot refresh: "ws" status is "Pending", must be one of: "Ready"`,
		},
	}

	soon := 0
	s.runActionTest(c, requests, expected, func(st *state.State, d time.Duration) {
		if soon == 0 {
			err := s.b.StopWorkshop(s.ctx, "ws", true)
			c.Assert(err, check.IsNil)
			chg := s.d.state.Change("1")
			c.Assert(chg, check.NotNil)
			chg.SetStatus(state.DoneStatus)
		}
		soon++
	})

	// all successful responses must initiate the ensure call
	c.Assert(soon, check.Equals, 2)
}

func (s *apiSuite) TestProjectsPostProjectWorkshopStop(c *check.C) {
	// Setup
	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["ws"],"action":"stop"}`),
		// a second attempt to stop the workshop that is already in Pending (e.g being stopped)
		bytes.NewBufferString(`{"names":["ws"],"action":"stop"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "stop",
			Summary: `Stop "ws" workshop`,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `cannot stop: "ws" status is "Pending", must be one of: "Ready", "Stopped"`,
		},
	}

	soon := 0
	s.runActionTest(c, requests, expected, func(st *state.State, d time.Duration) { soon++ })
	// all successful responses must initiate the ensure call
	c.Assert(soon, check.Equals, 1)
}

func (s *apiSuite) TestProjectsPostProjectWorkshopRemove(c *check.C) {
	// Setup

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["ws"],"action":"remove"}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"remove"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "remove",
			Summary: `Remove "ws" workshop`,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `cannot remove: "ws" status is "Pending", must be one of: "Ready", "Error", "Stopped"`,
		},
	}

	soon := 0
	s.runActionTest(c, requests, expected, func(st *state.State, d time.Duration) { soon++ })
	// all successful responses must initiate the ensure call
	c.Assert(soon, check.Equals, 1)
}
