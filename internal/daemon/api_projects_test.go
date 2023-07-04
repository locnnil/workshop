package daemon

import (
	"bytes"
	"context"
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
	userCtx := context.WithValue(req.Context(), workspacebackend.ContextUser, s.username)
	projectCts := context.WithValue(userCtx, workspacebackend.ContextProjectId, s.projectId)
	return req.WithContext(projectCts), err
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
	buf := bytes.NewBufferString(`{"path": "/home/testuser/project"}`)

	req, err := s.createProjectsRequest("POST", "/v1/projects", buf)
	c.Assert(err, check.IsNil)
	b := s.d.overlord.WorkspaceBackend()

	b.CreateOrLoadProject(req.Context(), "/home/testuser/project")

	// Execute
	rsp := v1PostProjects(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	res, err := rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(string(res), check.Matches, `.*{"path":"/home/testuser/project","id":"b8639dea"}.*`)
}

func (s *apiSuite) TestProjectsGetWorkspaces(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workspaces")
	s.vars = map[string]string{"id": s.projectId}

	req, err := s.createProjectsRequest("GET", "/v1/projects/b8639dea/workspaces", nil)
	c.Assert(err, check.IsNil)
	b := s.d.overlord.WorkspaceBackend()
	b.CreateOrLoadProject(req.Context(), s.workspaceDir)

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
			ProjectId: "b8639dea",
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
	s.vars = map[string]string{"id": s.projectId, "name": "ws-test"}

	req, err := s.createProjectsRequest("GET", "/v1/projects/b8639dea/workspaces/ws-test", nil)
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
		ProjectId: s.projectId,
		State:     "Ready",
	})
}

func (s *apiSuite) TestProjectsGetWorkspaceNoNameProvided(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workspaces/{name}")
	s.vars = map[string]string{"id": "b8639dea"}

	req, err := s.createProjectsRequest("GET", "/v1/projects/b8639dea/workspaces/ws-test", nil)
	c.Assert(err, check.IsNil)

	// Execute
	rsp := v1GetProjectWorkspace(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeError)
	c.Assert(rsp.Status, check.Equals, http.StatusBadRequest)
	c.Assert(rsp.Result.(*errorResult).Message, check.Matches, "workspace name must be provided")
}

func (s *apiSuite) TestProjectsGetWorkspaceNoProjectIdProvided(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workspaces/{name}")
	s.vars = map[string]string{}

	req, err := s.createProjectsRequest("GET", "/v1/projects/b8639dea/workspaces/ws-test", nil)
	c.Assert(err, check.IsNil)

	// Execute
	rsp := v1GetProjectWorkspace(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeError)
	c.Assert(rsp.Status, check.Equals, http.StatusBadRequest)
	c.Assert(rsp.Result.(*errorResult).Message, check.Matches, "project-id must be provided")
}

func (s *apiSuite) TestProjectsPostProjectRefreshWorkspace(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workspaces")
	s.vars = map[string]string{"id": s.projectId}
	os.WriteFile(filepath.Join(s.workspaceDir, ".workspace.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04`), 0644)

	soon := 0
	restoreEnsure := testutil.FakeFunc(func(st *state.State, d time.Duration) { soon++ }, &ensureStateSoon)
	defer restoreEnsure()

	buffers := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["ws"],"action":"launch"}`),
		bytes.NewBufferString(`{"names":[],"action":"refresh"}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh"}`),
		bytes.NewBufferString(`{"names":["ws"],"action":"refresh", "hold-on-error": true}`),
		bytes.NewBufferString(`{"names":["ws", "ws1"],"action":"refresh", "hold-on-error": true}`),
	}

	requests := []*http.Request{}
	expected := []*struct {
		Type       ResponseType
		Status     int
		ChangeHold bool
	}{
		{
			Type:   ResponseTypeAsync,
			Status: http.StatusAccepted,
		},
		{
			Type:   ResponseTypeError,
			Status: http.StatusBadRequest,
		},
		{
			Type:   ResponseTypeAsync,
			Status: http.StatusAccepted,
		},
		{
			Type:       ResponseTypeAsync,
			Status:     http.StatusAccepted,
			ChangeHold: true,
		},
		{
			Type:   ResponseTypeError,
			Status: http.StatusBadRequest,
		},
	}

	for _, i := range buffers {
		req, err := s.createProjectsRequest("POST", "/v1/projects/b8639dea/workspaces", i)
		c.Assert(err, check.IsNil)
		requests = append(requests, req)
	}

	b := s.d.overlord.WorkspaceBackend()
	prj, _, err := b.CreateOrLoadProject(requests[0].Context(), s.workspaceDir)
	c.Assert(err, check.IsNil)

	b.LaunchWorkspace(requests[0].Context(), "ws", "ubuntu@20.04")
	b.LaunchWorkspace(requests[0].Context(), "ws1", "ubuntu@20.04")
	st := s.d.overlord.State()

	for num, i := range requests {
		// Execute
		rsp := v1PostProjectWorkspace(projectsCmd, i, nil).(*resp)
		{
			// Verify
			c.Check(rsp.Type, check.Equals, expected[num].Type)
			c.Assert(rsp.Status, check.Equals, expected[num].Status)

			if rsp.Type != ResponseTypeError {
				st.Lock()
				var user string
				var projectKey workspacebackend.Project
				var hold bool
				chg := st.Change(rsp.Change)
				chg.Get("user", &user)
				chg.Get("project-key", &projectKey)
				chg.Get("hold-on-error", &hold)

				c.Assert(user, check.Equals, s.username)
				c.Assert(&projectKey, check.DeepEquals, prj)
				c.Assert(hold, check.Equals, expected[num].ChangeHold)
				st.Unlock()
			}
		}
	}

	// all successful responses must initiate the ensure call
	c.Assert(soon, check.Equals, 3)
}
