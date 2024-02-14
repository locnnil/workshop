package daemon

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/canonical/workshop/internal/overlord/healthstate"
	"github.com/canonical/workshop/internal/overlord/operation"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/workshopstate"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
	"gopkg.in/check.v1"
)

func (s *apiSuite) createProjectsRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	req.RemoteAddr = "pid=11;uid=1000;socket=(/var/lib/workshop/.socket);"
	return req.WithContext(s.ctx), err
}

func (s *apiSuite) TestProjectsGetProjects(c *check.C) {
	// Setup
	s.daemon(c)
	req, err := s.createProjectsRequest("GET", "/v1/projects", nil)
	c.Assert(err, check.IsNil)

	projectsCmd := apiCmd("/v1/projects")

	// Execute
	rsp := v1GetProjects(projectsCmd, req, nil).(*resp)

	// Verify
	c.Check(rsp.Type, check.Equals, ResponseTypeSync)
	c.Check(rsp.Status, check.Equals, 200)

	_, err = rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(rsp.Result, testutil.DeepUnsortedMatches, []*workshopbackend.Project{
		{Path: s.project.Path, ProjectId: s.project.ProjectId},
	})
}

func (s *apiSuite) TestProjectsPostProjectDoesNotExist(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects")

	buf := bytes.NewBufferString(`{"path": "/home/testuser/project"}`)

	req, err := s.createProjectsRequest("POST", "/v1/projects", buf)
	c.Assert(err, check.IsNil)

	// Execute
	rsp := v1PostProjects(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, 201)

	res, err := rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(string(res), check.Matches, `.*{"path":"/home/testuser/project","id":"b8639dea"}.*`)
}

func (s *apiSuite) TestProjectsPostProjectExist(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects")
	buf := bytes.NewBufferString(fmt.Sprintf(`{"path": "%s"}`, s.project.Path))

	req, err := s.createProjectsRequest("POST", "/v1/projects", buf)
	c.Assert(err, check.IsNil)

	// Execute
	rsp := v1PostProjects(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	res, err := rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(string(res), check.Matches,
		fmt.Sprintf(`.*{"path":"%s","id":"%s"}.*`, s.project.Path, s.project.ProjectId))
}

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
			Notes: []string{""},
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
	restore := testutil.FakeFunc(func() time.Time { return time.Date(2023, 04, 25, 1, 2, 3, 0, time.UTC) }, &workshopbackend.InstallTimeNow)
	workshop.LinkSdk(s.ctx, sdk.Setup{Name: "go", Channel: "latest/stable", Revision: 234})
	restore()

	// Execute
	restore = FakeWorkshopHealth(func(mgr *workshopstate.WorkshopManager, w *workshopbackend.Workshop) healthstate.HealthState {
		return healthstate.HealthState{Status: healthstate.ReadyStatus, SdkHealth: map[string]healthstate.HealthCheck{
			"go": {Sdk: "go", Message: "test health check message", Code: "check-waiting", CheckResult: healthstate.CheckWaiting},
		}}
	})
	rsp := v1GetProjectWorkshop(projectsCmd, req, nil).(*resp)
	restore()

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
		Notes:     []string{""},
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
			},
		},
	})
}

func (s *apiSuite) TestProjectsPostProjectWorkshopLaunch(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workshops")
	s.vars = map[string]string{"id": s.project.ProjectId}

	s.launchWorkshop(s.ctx, "ws", c)

	// Mock another workshop file
	err := os.WriteFile(filepath.Join(s.workshopDir, ".workshop.ws1.yaml"), []byte(`name: ws1
base: ubuntu@20.04`), 0644)
	c.Assert(err, check.IsNil)

	buffers := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["ws1", "ws1", "ws1"],"action":"launch"}`),
		bytes.NewBufferString(`{"names":[],"action":"launch"}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"launch"}`),
	}

	requests := []*http.Request{}
	expected := []*struct {
		Type    ResponseType
		Status  int
		Message string
	}{
		{
			Type:   ResponseTypeAsync,
			Status: http.StatusAccepted,
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

	for _, i := range buffers {
		req, err := s.createProjectsRequest("POST", "/v1/projects/"+s.project.ProjectId+"/workshops", i)
		c.Assert(err, check.IsNil)
		requests = append(requests, req)
	}

	soon := 0
	restoreEnsure := testutil.FakeFunc(func(st *state.State, d time.Duration) {
		soon++
	}, &ensureStateSoon)
	defer restoreEnsure()

	for num, i := range requests {
		// Execute
		rsp := v1PostProjectWorkshop(projectsCmd, i, nil).(*resp)
		{
			// Verify
			c.Check(rsp.Type, check.Equals, expected[num].Type)
			c.Assert(rsp.Status, check.Equals, expected[num].Status, check.Commentf("case: %v", num))
			if rsp.Type == ResponseTypeError {
				c.Assert(rsp.Result.(*errorResult).Message, check.Equals, expected[num].Message)
			}
		}
	}

	// all successful responses must initiate the ensure call
	c.Assert(soon, check.Equals, 1)
}

func (s *apiSuite) TestProjectsPostProjectRefreshWorkshopContinue(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workshops")
	s.vars = map[string]string{"id": s.project.ProjectId}

	buffers := []*bytes.Buffer{
		// try continue without starting wait-on-error
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh", "options": {"refresh-mode":"continue"}}`),

		// a workshop name is a must
		bytes.NewBufferString(`{"names":[],"action":"refresh"}`),

		// non-transactional refresh is only supported for a single workshop
		bytes.NewBufferString(`{"names":["ws", "ws1"],"action":"refresh","options": {"refresh-mode":"wait-on-error"}}`),

		// start - attempt transactional - continue (success) - continue (fail, already finished)
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"wait-on-error"}}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"transactional"}}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"continue"}}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"continue"}}`),

		// start transactional (success) - attempt abort or continue (failure)
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"transactional"}}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"continue"}}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"abort"}}`),

		// start - abort (both success)
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"wait-on-error"}}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","options": {"refresh-mode":"abort"}}`),
	}

	requests := []*http.Request{}
	for _, i := range buffers {
		req, err := s.createProjectsRequest("POST", "/v1/projects/"+s.project.ProjectId+"/workshops", i)
		c.Assert(err, check.IsNil)
		requests = append(requests, req)
	}

	expected := []*struct {
		Type    ResponseType
		Status  int
		Message string
	}{
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "cannot continue, no refresh in progress",
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
		{
			Type:   ResponseTypeAsync,
			Status: http.StatusAccepted,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `cannot refresh: "ws" status is "Pending", must be one of: "Ready"`,
		},
		{
			Type:   ResponseTypeAsync,
			Status: http.StatusAccepted,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "cannot continue, no refresh in progress",
		},
		{
			Type:   ResponseTypeAsync,
			Status: http.StatusAccepted,
		},
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
		{
			Type:   ResponseTypeAsync,
			Status: http.StatusAccepted,
		},
		{
			Type:   ResponseTypeAsync,
			Status: http.StatusAccepted,
		},
	}

	// we expect 4 ensure calls according to the scenarios above
	mockRefreshChanges := map[int]state.Status{
		0: state.WaitStatus,
		1: state.DoneStatus,
		2: state.DoneStatus,
		3: state.WaitStatus,
	}

	soon := 0
	restoreEnsure := testutil.FakeFunc(func(st *state.State, d time.Duration) {
		if mockRefreshChanges[soon] == state.DoneStatus {
			operation.StopOperation(st, "ws", s.project.ProjectId, operation.OperationRefresh)
		}
		soon++
	}, &ensureStateSoon)
	defer restoreEnsure()

	s.launchWorkshop(s.ctx, "ws", c)
	s.launchWorkshop(s.ctx, "ws1", c)

	for num, i := range requests {
		// Execute
		rsp := v1PostProjectWorkshop(projectsCmd, i, nil).(*resp)
		{
			// Verify
			c.Check(rsp.Type, check.Equals, expected[num].Type)
			c.Assert(rsp.Status, check.Equals, expected[num].Status, check.Commentf("case: %v", num))
			if rsp.Type == ResponseTypeError {
				c.Assert(rsp.Result.(*errorResult).Message, check.Equals, expected[num].Message, check.Commentf("case: %v", num))
			}
		}
	}

	// all successful responses must initiate the ensure call
	c.Assert(soon, check.Equals, 5)
}

func (s *apiSuite) TestProjectsPostProjectWorkshopStart(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workshops")
	s.vars = map[string]string{"id": s.project.ProjectId}

	s.launchWorkshop(s.ctx, "ws", c)
	err := s.b.StopWorkshop(s.ctx, "ws", false)
	c.Assert(err, check.IsNil)

	buffers := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["ws"],"action":"start"}`),
		// a second attempt to start the workshop that is already in Pending (i.e. being started)
		bytes.NewBufferString(`{"names":["ws"],"action":"start"}`),
		// ensure another operation is not going to work when the workshop in Pending
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh"}`),
	}

	requests := []*http.Request{}
	expected := []*struct {
		Type    ResponseType
		Status  int
		Message string
	}{
		{
			Type:   ResponseTypeAsync,
			Status: http.StatusAccepted,
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

	for _, i := range buffers {
		req, err := s.createProjectsRequest("POST", "/v1/projects/"+s.project.ProjectId+"/workshops", i)
		c.Assert(err, check.IsNil)
		requests = append(requests, req)
	}

	soon := 0
	restoreEnsure := testutil.FakeFunc(func(st *state.State, d time.Duration) {
		soon++
	}, &ensureStateSoon)
	defer restoreEnsure()

	for num, i := range requests {
		// Execute
		rsp := v1PostProjectWorkshop(projectsCmd, i, nil).(*resp)
		{
			// Verify
			c.Check(rsp.Type, check.Equals, expected[num].Type)
			c.Assert(rsp.Status, check.Equals, expected[num].Status, check.Commentf("case: %v", num))
			if rsp.Type == ResponseTypeError {
				c.Assert(rsp.Result.(*errorResult).Message, check.Equals, expected[num].Message)
			}
		}
	}

	s.d.overlord.State().Lock()
	workshopMgr := s.d.overlord.WorkshopManager()
	ws, err := workshopMgr.Workshop(s.ctx, "ws", s.project.ProjectId)
	c.Assert(err, check.IsNil)
	c.Assert(workshopMgr.WorkshopHealth(ws).Status, check.Equals, healthstate.PendingStatus)
	s.d.overlord.State().Unlock()

	// all successful responses must initiate the ensure call
	c.Assert(soon, check.Equals, 1)
}

func (s *apiSuite) TestProjectsPostProjectWorkshopStop(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workshops")
	s.vars = map[string]string{"id": s.project.ProjectId}

	s.launchWorkshop(s.ctx, "ws", c)

	buffers := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["ws"],"action":"stop"}`),
		// a second attempt to stop the workshop that is already in Pending (i.e. being stopped)
		bytes.NewBufferString(`{"names":["ws"],"action":"stop"}`),
		// stop the workshop that is already stopped
		bytes.NewBufferString(`{"names":["ws"],"action":"stop"}`),
	}

	requests := []*http.Request{}
	expected := []*struct {
		Type    ResponseType
		Status  int
		Message string
	}{
		{
			Type:   ResponseTypeAsync,
			Status: http.StatusAccepted,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `cannot stop: "ws" status is "Pending", must be one of: "Ready", "Stopped"`,
		},
		{
			Type:   ResponseTypeAsync,
			Status: http.StatusAccepted,
		},
	}

	for _, i := range buffers {
		req, err := s.createProjectsRequest("POST", "/v1/projects/"+s.project.ProjectId+"/workshops", i)
		c.Assert(err, check.IsNil)
		requests = append(requests, req)
	}

	soon := 0
	restoreEnsure := testutil.FakeFunc(func(st *state.State, d time.Duration) {
		soon++
	}, &ensureStateSoon)
	defer restoreEnsure()

	for num, i := range requests {
		// Execute
		rsp := v1PostProjectWorkshop(projectsCmd, i, nil).(*resp)

		// Verify
		c.Check(rsp.Type, check.Equals, expected[num].Type)
		c.Assert(rsp.Status, check.Equals, expected[num].Status, check.Commentf("case: %v, msg: %v", num, rsp.Result))
		if rsp.Type == ResponseTypeError {
			c.Assert(rsp.Result.(*errorResult).Message, check.Equals, expected[num].Message)

			s.d.state.Lock()
			// stop the workshop stop operation here, so it is ready for the next command
			err := operation.StopOperation(s.d.state, "ws", s.project.ProjectId, operation.OperationStop)
			c.Assert(err, check.IsNil)
			s.b.StopWorkshop(s.ctx, "ws", false)
			s.d.state.Unlock()
		}
	}
	// all successful responses must initiate the ensure call
	c.Assert(soon, check.Equals, 2)
}

func (s *apiSuite) TestProjectsPostProjectWorkshopRemove(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workshops")
	s.vars = map[string]string{"id": s.project.ProjectId}

	s.launchWorkshop(s.ctx, "ws", c)

	buffers := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["ws"],"action":"remove"}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"remove"}`),
	}

	requests := []*http.Request{}
	expected := []*struct {
		Type    ResponseType
		Status  int
		Message string
	}{
		{
			Type:   ResponseTypeAsync,
			Status: http.StatusAccepted,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `cannot remove: "ws" status is "Pending", must be one of: "Ready", "Error", "Stopped"`,
		},
	}

	for _, i := range buffers {
		req, err := s.createProjectsRequest("POST", "/v1/projects/"+s.project.ProjectId+"/workshops", i)
		c.Assert(err, check.IsNil)
		requests = append(requests, req)
	}

	soon := 0
	restoreEnsure := testutil.FakeFunc(func(st *state.State, d time.Duration) {
		soon++
	}, &ensureStateSoon)
	defer restoreEnsure()

	for num, i := range requests {
		// Execute
		rsp := v1PostProjectWorkshop(projectsCmd, i, nil).(*resp)

		// Verify
		c.Check(rsp.Type, check.Equals, expected[num].Type)
		c.Assert(rsp.Status, check.Equals, expected[num].Status, check.Commentf("case: %v, msg: %v", num, rsp.Result))
		if rsp.Type == ResponseTypeError {
			c.Assert(rsp.Result.(*errorResult).Message, check.Equals, expected[num].Message)
		}

		s.d.state.Lock()
		op := operation.OperationInProgress(s.d.state, "ws", s.project.ProjectId)
		c.Assert(op, check.NotNil)
		c.Assert(op.Operation, check.Equals, operation.OperationRemove)
		s.d.state.Unlock()
	}
	// all successful responses must initiate the ensure call
	c.Assert(soon, check.Equals, 1)
}
