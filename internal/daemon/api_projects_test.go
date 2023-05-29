package daemon

import (
	"net/http"
	"time"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/project"
	"github.com/canonical/workspace/internal/workspacebackend"
	"github.com/spf13/afero"
	"gopkg.in/check.v1"
)

func (s *apiSuite) TestProjectsWithPathProvided(c *check.C) {
	restore := state.FakeTime(time.Date(2023, 04, 21, 1, 2, 3, 0, time.UTC))
	defer restore()

	// Setup
	s.daemon(c)
	id, pth := "2345gtfs", "/home/user/project"
	project.LoadProject = func(backend workspacebackend.WorkspaceBackend, fs afero.Fs, path string) (*project.Project, error) {
		return &project.Project{ProjectId: id, Path: pth}, nil
	}
	projectsCmd := apiCmd("/v1/projects")

	// Execute
	req, err := http.NewRequest("GET", "/v1/projects?path=/home/user/project", nil)
	c.Assert(err, check.IsNil)
	rsp := v1Projects(projectsCmd, req, nil).(*resp)

	// Verify
	c.Check(rsp.Type, check.Equals, ResponseTypeSync)
	c.Check(rsp.Status, check.Equals, 200)

	res, err := rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(string(res), check.Matches, `.*\[{"path":"/home/user/project","project-id":"2345gtfs"}\].*`)
}

func (s *apiSuite) TestProjectsNoPathProvided(c *check.C) {
	restore := state.FakeTime(time.Date(2023, 04, 21, 1, 2, 3, 0, time.UTC))
	defer restore()

	// Setup
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects")

	// Execute
	req, err := http.NewRequest("GET", "/v1/projects", nil)
	c.Assert(err, check.IsNil)
	rsp := v1Projects(projectsCmd, req, nil).(*resp)

	// Verify
	c.Check(rsp.Type, check.Equals, ResponseTypeSync)
	c.Check(rsp.Status, check.Equals, 200)

	res, err := rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(string(res), check.Matches, `.*\[\].*`)
}
