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
	return req.WithContext(userCtx), err
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

	// will be called when project is created
	restore := testutil.FakeFunc(func() (string, error) { return "b8639dea", nil }, &workspacebackend.NewProjectId)
	defer restore()

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
	// will be called when project is created
	restore := testutil.FakeFunc(func() (string, error) { return "b8639dea", nil }, &workspacebackend.NewProjectId)
	defer restore()
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
	s.vars = map[string]string{"id": "b8639dea"}

	req, err := s.createProjectsRequest("GET", "/v1/projects/b8639dea/workspaces", nil)
	c.Assert(err, check.IsNil)
	b := s.d.overlord.WorkspaceBackend()
	ctx := context.WithValue(req.Context(), workspacebackend.ContextProjectId, "b8639dea")
	b.LaunchWorkspace(ctx, "ws-test", "ubuntu@20.04")
	ws, _ := b.GetWorkspace(ctx, "ws-test")
	ws.LinkSdk(ctx, &sdk.SdkInfo{Name: "go", Channel: "latest/stable", Revision: 234})

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
	s.vars = map[string]string{"id": "b8639dea", "name": "ws-test"}

	req, err := s.createProjectsRequest("GET", "/v1/projects/b8639dea/workspaces/ws-test", nil)
	c.Assert(err, check.IsNil)
	b := s.d.overlord.WorkspaceBackend()
	b.LaunchWorkspace(context.WithValue(req.Context(), workspacebackend.ContextProjectId, "b8639dea"), "ws-test", "ubuntu@20.04")

	// Execute
	rsp := v1GetProjectWorkspace(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	_, err = rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(rsp.Result, check.DeepEquals, &WorkspaceInfo{
		Name:      "ws-test",
		ProjectId: "b8639dea",
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

func (s *apiSuite) TestProjectsPostProjectLaunchWorkspace(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workspaces")
	s.vars = map[string]string{"id": "b8639dea"}
	projectDir := c.MkDir()
	os.WriteFile(filepath.Join(projectDir, ".workspace.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04`), 0644)

	buf := bytes.NewBufferString(`{"names":["ws"],"action":"launch"}`)
	req, err := s.createProjectsRequest("POST", "/v1/projects/b8639dea/workspaces", buf)
	c.Assert(err, check.IsNil)

	// will be called when project is created
	restoreNewId := testutil.FakeFunc(func() (string, error) { return "b8639dea", nil }, &workspacebackend.NewProjectId)
	defer restoreNewId()

	soon := 0
	restoreEnsure := testutil.FakeFunc(func(st *state.State, d time.Duration) { soon++ }, &ensureStateSoon)
	defer restoreEnsure()

	b := s.d.overlord.WorkspaceBackend()
	prj, _, err := b.CreateOrLoadProject(req.Context(), projectDir)
	c.Assert(err, check.IsNil)

	// Execute
	rsp := v1PostProjectWorkspace(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeAsync)
	c.Assert(rsp.Status, check.Equals, http.StatusAccepted)
	c.Assert(soon, check.Equals, 1)
	c.Assert(rsp.Change, check.Equals, "1")

	st := s.d.overlord.State()
	st.Lock()
	defer st.Unlock()
	var user string
	var projectKey workspacebackend.Project
	st.Change("1").Get("user", &user)
	st.Change("1").Get("project-key", &projectKey)

	c.Assert(user, check.Equals, s.username)
	c.Assert(&projectKey, check.DeepEquals, prj)
}
