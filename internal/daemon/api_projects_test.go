package daemon

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/statecontext"
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

func (s *apiSuite) TestProjectsPostProjectExists(c *check.C) {
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

func (s *apiSuite) TestProjectsGetWorkshops(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workshops")
	s.vars = map[string]string{"id": s.project.ProjectId}
	restore := testutil.FakeFunc(func() time.Time { return time.Date(2023, 04, 25, 1, 2, 3, 0, time.UTC) }, &workshopbackend.InstallTimeNow)
	defer restore()

	req, err := s.createProjectsRequest("GET", "/v1/projects/"+s.project.ProjectId+"/workshops", nil)
	c.Assert(err, check.IsNil)
	b := s.d.overlord.WorkshopBackend()
	err = os.WriteFile(filepath.Join(s.project.Path, ".workshop.ws-test.yaml"), []byte(`name: ws-test
base: ubuntu@20.04
`), 0644)
	c.Assert(err, check.IsNil)
	err = b.LaunchWorkshop(req.Context(), "ws-test", "ubuntu@20.04")
	c.Assert(err, check.IsNil)
	ws, err := b.Workshop(req.Context(), "ws-test")
	c.Assert(err, check.IsNil)
	ws.LinkSdk(req.Context(), sdk.Setup{Name: "go", Channel: "latest/stable", Revision: 234})

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
	b := s.d.overlord.WorkshopBackend()
	err = os.WriteFile(filepath.Join(s.project.Path, ".workshop.ws-test.yaml"), []byte(`name: ws-test
base: ubuntu@20.04
`), 0644)
	c.Assert(err, check.IsNil)
	err = b.LaunchWorkshop(s.ctx, "ws-test", "ubuntu@20.04")
	c.Assert(err, check.IsNil)

	// Execute
	rsp := v1GetProjectWorkshop(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	_, err = rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(rsp.Result, check.DeepEquals, &WorkshopInfo{
		Name:      "ws-test",
		ProjectId: s.project.ProjectId,
		Status:    "Ready",
	})
}

func (s *apiSuite) TestProjectsPostProjectWorkshopLaunch(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workshops")
	s.vars = map[string]string{"id": s.project.ProjectId}

	// Mock workshop files
	err := os.WriteFile(filepath.Join(s.workshopDir, ".workshop.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04`), 0644)
	c.Assert(err, check.IsNil)

	err = os.WriteFile(filepath.Join(s.workshopDir, ".workshop.ws1.yaml"), []byte(`name: ws1
base: ubuntu@20.04`), 0644)
	c.Assert(err, check.IsNil)

	err = s.b.LaunchWorkshop(s.ctx, "ws", "ubuntu@20.04")
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
	os.WriteFile(filepath.Join(s.workshopDir, ".workshop.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04`), 0644)
	os.WriteFile(filepath.Join(s.workshopDir, ".workshop.ws1.yaml"), []byte(`name: ws1
base: ubuntu@20.04`), 0644)

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
			Message: "cannot refresh: \"ws\" is in pending; must be ready",
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
			statecontext.StopOperation(st, "ws", s.project.ProjectId, statecontext.OperationRefresh)
		}
		soon++
	}, &ensureStateSoon)
	defer restoreEnsure()

	err := s.b.LaunchWorkshop(s.ctx, "ws", "ubuntu@20.04")
	c.Assert(err, check.IsNil)

	err = s.b.LaunchWorkshop(s.ctx, "ws1", "ubuntu@20.04")
	c.Assert(err, check.IsNil)

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
	os.WriteFile(filepath.Join(s.workshopDir, ".workshop.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04`), 0644)

	err := s.b.LaunchWorkshop(s.ctx, "ws", "ubuntu@20.04")
	c.Assert(err, check.IsNil)
	err = s.b.StopWorkshop(s.ctx, "ws", false)
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
			Message: "cannot start: \"ws\" is in pending; must be stopped",
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "cannot refresh: \"ws\" is in pending; must be ready",
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
	ws, err := s.d.overlord.WorkshopManager().Workshop(s.ctx, "ws", s.project.ProjectId)
	c.Assert(err, check.IsNil)
	c.Assert(ws.Status(), check.Equals, workshopbackend.WorkshopPending)
	s.d.overlord.State().Unlock()

	// all successful responses must initiate the ensure call
	c.Assert(soon, check.Equals, 1)
}

func (s *apiSuite) TestProjectsPostProjectWorkshopStop(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workshops")
	s.vars = map[string]string{"id": s.project.ProjectId}
	os.WriteFile(filepath.Join(s.workshopDir, ".workshop.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04`), 0644)

	err := s.b.LaunchWorkshop(s.ctx, "ws", "ubuntu@20.04")
	c.Assert(err, check.IsNil)

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
			Message: "cannot stop: \"ws\" is in pending; must be stopped or ready",
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
			err := statecontext.StopOperation(s.d.state, "ws", s.project.ProjectId, statecontext.OperationStop)
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
	os.WriteFile(filepath.Join(s.workshopDir, ".workshop.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04`), 0644)

	err := s.b.LaunchWorkshop(s.ctx, "ws", "ubuntu@20.04")
	c.Assert(err, check.IsNil)

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
			Message: "cannot stop: \"ws\" is in pending; must be ready, stopped or error",
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

		s.d.state.Lock()
		op := statecontext.OperationInProgress(s.d.state, "ws", s.project.ProjectId)
		c.Assert(op, check.NotNil)
		c.Assert(op.Operation, check.Equals, statecontext.OperationRemove)
		s.d.state.Unlock()
	}
	// all successful responses must initiate the ensure call
	c.Assert(soon, check.Equals, 1)
}
