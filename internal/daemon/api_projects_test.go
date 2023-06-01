package daemon

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/project"
	"github.com/canonical/workspace/internal/testutil"
	"github.com/canonical/workspace/internal/workspacebackend"
	"github.com/spf13/afero"
	"gopkg.in/check.v1"
)

func (s *apiSuite) createProjectsRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	req.RemoteAddr = "pid=11;uid=1000;socket=(/var/lib/workspace/.socket);"
	userCtx := context.WithValue(req.Context(), workspacebackend.ContextUser, s.username)
	return req.WithContext(userCtx), err
}

func (s *apiSuite) TestProjectsGetProjects(c *check.C) {
	restore := state.FakeTime(time.Date(2023, 04, 21, 1, 2, 3, 0, time.UTC))
	defer restore()

	// Setup
	s.daemon(c)
	defer testutil.FakeFunc(func(ctx context.Context, backend workspacebackend.WorkspaceBackend, fs afero.Fs) ([]*project.Project, error) {
		c.Assert(ctx.Value(workspacebackend.ContextUser).(string), check.Equals, "testuser")

		return []*project.Project{
			{ProjectId: "2345gtfs", Path: "/home/testuser/project"},
			{ProjectId: "6789gtfs", Path: "/home/testuser/project2"}}, nil
	}, &project.RetrieveAllProjects)()

	projectsCmd := apiCmd("/v1/projects")

	// Execute
	req, err := s.createProjectsRequest("GET", "/v1/projects", nil)
	c.Assert(err, check.IsNil)

	rsp := v1GetProjects(projectsCmd, req, nil).(*resp)

	// Verify
	c.Check(rsp.Type, check.Equals, ResponseTypeSync)
	c.Check(rsp.Status, check.Equals, 200)

	res, err := rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(string(res), check.Matches, `.*\[{"path":"/home/testuser/project","id":"2345gtfs"},{"path":"/home/testuser/project2","id":"6789gtfs"}\].*`)
}

func (s *apiSuite) TestProjectsPostProjectDoesNotExist(c *check.C) {
	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects")
	defer testutil.FakeFunc(func(ctx context.Context, backend workspacebackend.WorkspaceBackend, fs afero.Fs, path string) (*project.Project, error) {
		return nil, project.ErrProjectNotFound
	}, &project.RetrieveProject)()

	defer testutil.FakeFunc(func(fs afero.Fs, path string) (*project.Project, error) {
		return &project.Project{ProjectId: "3j5h6g3t", Path: "/home/testuser/project"}, nil
	}, &project.NewProject)()

	// Execute
	buf := bytes.NewBufferString(`{"path": "/home/test/project"}`)

	req, err := s.createProjectsRequest("POST", "/v1/projects", buf)
	c.Assert(err, check.IsNil)

	rsp := v1PostProjects(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, 201)

	res, err := rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(string(res), check.Matches, `.*{"path":"/home/testuser/project","id":"3j5h6g3t"}.*`)
}
