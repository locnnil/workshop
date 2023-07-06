package daemon

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/sdk"
	"github.com/canonical/workspace/internal/testutil"
	"github.com/canonical/workspace/internal/workspacebackend"
	"gopkg.in/check.v1"
)

func (s *apiSuite) createProjectsRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	req.RemoteAddr = "pid=11;uid=1000;socket=(/var/lib/workspace/.socket);"
	return req.WithContext(s.ctx), err
}

func (s *apiSuite) TestProjectsGetProjects(c *check.C) {
	// Setup
	s.daemon(c)
	req, err := s.createProjectsRequest("GET", "/v1/projects", nil)
	c.Assert(err, check.IsNil)

	b := s.d.overlord.WorkspaceBackend()
	// will be called when project is created
	numCalls := 0
	ids := []string{"b8639dea", "a8639dea"}
	restoreId := testutil.FakeFunc(func() (string, error) { numCalls = numCalls + 1; return ids[numCalls-1], nil }, &workspacebackend.NewProjectId)
	defer restoreId()
	b.CreateOrLoadProject(req.Context(), "/home/testuser/project")
	b.CreateOrLoadProject(req.Context(), "/home/testuser/project2")

	projectsCmd := apiCmd("/v1/projects")

	// Execute
	rsp := v1GetProjects(projectsCmd, req, nil).(*resp)

	// Verify
	c.Check(rsp.Type, check.Equals, ResponseTypeSync)
	c.Check(rsp.Status, check.Equals, 200)

	_, err = rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(rsp.Result, check.DeepEquals, []*workspacebackend.Project{
		{Path: "/home/testuser/project", ProjectId: "b8639dea"},
		{Path: "/home/testuser/project2", ProjectId: "a8639dea"},
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

func (s *apiSuite) TestProjectsGetWorkspaces(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workspaces")
	s.vars = map[string]string{"id": s.project.ProjectId}

	req, err := s.createProjectsRequest("GET", "/v1/projects/"+s.project.ProjectId+"/workspaces", nil)
	c.Assert(err, check.IsNil)
	b := s.d.overlord.WorkspaceBackend()

	b.LaunchWorkspace(req.Context(), "ws-test", "ubuntu@20.04")
	ws, _ := b.GetWorkspace(req.Context(), "ws-test")
	ws.LinkSdk(req.Context(), &sdk.SdkInfo{Name: "go", Channel: "latest/stable", Revision: 234})

	// Execute
	rsp := v1GetProjectWorkspaces(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	_, err = rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(rsp.Result, check.DeepEquals, []*WorkspaceInfo{
		{
			Name:      "ws-test",
			ProjectId: s.project.ProjectId,
			State:     "Ready",
			Content: []*SdkInfo{
				{
					Name:     "go",
					Channel:  "latest/stable",
					Revision: "234",
				},
			},
		},
	})
}

func (s *apiSuite) TestProjectsGetWorkspace(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workspaces/{name}")
	s.vars = map[string]string{"id": s.project.ProjectId, "name": "ws-test"}

	req, err := s.createProjectsRequest("GET", "/v1/projects/"+s.project.ProjectId+"/workspaces/ws-test", nil)
	c.Assert(err, check.IsNil)
	b := s.d.overlord.WorkspaceBackend()
	b.CreateOrLoadProject(req.Context(), s.workspaceDir)
	b.LaunchWorkspace(req.Context(), "ws-test", "ubuntu@20.04")

	// Execute
	rsp := v1GetProjectWorkspace(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	_, err = rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(rsp.Result, check.DeepEquals, &WorkspaceInfo{
		Name:      "ws-test",
		ProjectId: s.project.ProjectId,
		State:     "Ready",
	})
}

func (s *apiSuite) TestProjectsPostProjectWorkspaceAction(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workspaces")
	s.vars = map[string]string{"id": s.project.ProjectId}
	err := os.WriteFile(filepath.Join(s.workspaceDir, ".workspace.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04`), 0644)
	c.Assert(err, check.IsNil)

	soon := 0
	restoreEnsure := testutil.FakeFunc(func(st *state.State, d time.Duration) { soon++ }, &ensureStateSoon)
	defer restoreEnsure()

	buffers := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["ws"],"action":"launch"}`),
		bytes.NewBufferString(`{"names":[],"action":"refresh"}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","refresh-mode":"transactional"}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","refresh-mode":"hold-on-error"}`),
		bytes.NewBufferString(`{"names":["ws", "ws1"],"action":"refresh","refresh-mode": "hold-on-error"}`),
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
			Message: "at least one workspace name must be provided",
		},
		{
			Type:   ResponseTypeAsync,
			Status: http.StatusAccepted,
		},
		{
			Type:   ResponseTypeAsync,
			Status: http.StatusAccepted,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "hold-on-error is not supported for multiple workspaces",
		},
	}

	for _, i := range buffers {
		req, err := s.createProjectsRequest("POST", "/v1/projects/"+s.project.ProjectId+"/workspaces", i)
		c.Assert(err, check.IsNil)
		requests = append(requests, req)
	}

	b := s.d.overlord.WorkspaceBackend()
	b.LaunchWorkspace(s.ctx, "ws", "ubuntu@20.04")

	for num, i := range requests {
		// Execute
		rsp := v1PostProjectWorkspace(projectsCmd, i, nil).(*resp)
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
	c.Assert(soon, check.Equals, 3)
}

func (s *apiSuite) TestProjectsPostProjectRefreshWorkspaceContinue(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workspaces")
	s.vars = map[string]string{"id": s.project.ProjectId}
	os.WriteFile(filepath.Join(s.workspaceDir, ".workspace.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04`), 0644)

	buffers := []*bytes.Buffer{
		// try continue without starting hold-on-error
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","refresh-mode":"continue"}`),

		// start - attempt transactional - continue (success) - continue (fail, already finished)
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","refresh-mode":"hold-on-error"}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","refresh-mode":"transactional"}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","refresh-mode":"continue"}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","refresh-mode":"continue"}`),

		// start transactional (success) - attempt abort or continue (failure)
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","refresh-mode":"transactional"}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","refresh-mode":"continue"}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","refresh-mode":"abort"}`),

		// start - abort (both success)
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","refresh-mode":"hold-on-error"}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh","refresh-mode":"abort"}`),
	}

	requests := []*http.Request{}
	for _, i := range buffers {
		req, err := s.createProjectsRequest("POST", "/v1/projects/"+s.project.ProjectId+"/workspaces", i)
		c.Assert(err, check.IsNil)
		requests = append(requests, req)
	}

	expected := []*struct {
		Type       ResponseType
		Status     int
		ChangeHold bool
		Message    string
	}{
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "cannot continue, no refresh in progress",
		},
		{
			Type:       ResponseTypeAsync,
			Status:     http.StatusAccepted,
			ChangeHold: true,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "refresh operation is already in progress for: ws",
		},
		{
			Type:       ResponseTypeAsync,
			Status:     http.StatusAccepted,
			ChangeHold: true,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "cannot continue, no refresh in progress",
		},
		{
			Type:       ResponseTypeAsync,
			Status:     http.StatusAccepted,
			ChangeHold: false,
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
			Type:       ResponseTypeAsync,
			Status:     http.StatusAccepted,
			ChangeHold: true,
		},
		{
			Type:       ResponseTypeAsync,
			Status:     http.StatusAccepted,
			ChangeHold: true,
		},
	}

	refreshResults := []*struct {
		RefreshError error
	}{
		{
			&workspacebackend.ErrExec{Status: 0},
		}, {
			nil,
		},
	}

	soon := 0
	restoreEnsure := testutil.FakeFunc(func(st *state.State, d time.Duration) {
		// the first change is executed in the hold-on-error mode
		// so we emulate its non-transactional behaviour here
		// by setting one of its tasks to the Error state
		for _, i := range st.Changes() {
			hold := false
			i.Get("hold-on-error", &hold)
			if hold {
				s.d.overlord.WorkspaceManager().StartRefresh(st, "ws", s.project.ProjectId, i.ID())
			}
		}

		chg := st.Change("1")
		if !chg.Status().Ready() {
			tsk := chg.Tasks()[0]
			if refreshResults[soon].RefreshError != nil {
				tsk.SetStatus(state.ErrorStatus)
				tsk.Errorf(refreshResults[soon].RefreshError.Error())

			} else {
				for _, i := range chg.Tasks() {
					i.SetStatus(state.DoneStatus)
				}
				chg.Set("hold-on-error", false)
				s.d.overlord.WorkspaceManager().StopRefresh(st, "ws", s.project.ProjectId)
			}
		}
		soon++
	}, &ensureStateSoon)
	defer restoreEnsure()

	s.b.LaunchWorkspace(s.ctx, "ws", "ubuntu@20.04")

	for num, i := range requests {
		// Execute
		rsp := v1PostProjectWorkspace(projectsCmd, i, nil).(*resp)
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
