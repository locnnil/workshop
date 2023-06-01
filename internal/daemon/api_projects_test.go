package daemon

import (
	"context"
	"net/http"
	"os/user"
	"time"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/project"
	"github.com/canonical/workspace/internal/testutil"
	"github.com/canonical/workspace/internal/workspacebackend"
	"github.com/spf13/afero"
	"gopkg.in/check.v1"
)

func (s *apiSuite) TestProjectsNoPathProvided(c *check.C) {
	restore := state.FakeTime(time.Date(2023, 04, 21, 1, 2, 3, 0, time.UTC))
	defer restore()

	// Setup
	s.daemon(c)
	defer testutil.FakeMockupFunc(func(uid string) (*user.User, error) {
		return &user.User{Username: "testuser"}, nil
	}, &LookupUsername)()

	defer testutil.FakeMockupFunc(func(ctx context.Context, backend workspacebackend.WorkspaceBackend, fs afero.Fs) ([]*project.Project, error) {
		c.Assert(ctx.Value(workspacebackend.ContextUser).(string), check.Equals, "testuser")

		return []*project.Project{
			{ProjectId: "2345gtfs", Path: "/home/testuser/project"},
			{ProjectId: "6789gtfs", Path: "/home/testuser/project2"}}, nil
	}, &project.RetrieveAllProjects)()

	projectsCmd := apiCmd("/v1/projects")

	// Execute
	req, err := http.NewRequest("GET", "/v1/projects", nil)
	c.Assert(err, check.IsNil)
	req.RemoteAddr = "pid=11;uid=1000;socket=(/var/lib/workspace/.socket);"

	rsp := v1Projects(projectsCmd, req, nil).(*resp)

	// Verify
	c.Check(rsp.Type, check.Equals, ResponseTypeSync)
	c.Check(rsp.Status, check.Equals, 200)

	res, err := rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(string(res), check.Matches, `.*\[{"path":"/home/testuser/project","project-id":"2345gtfs"},{"path":"/home/testuser/project2","project-id":"6789gtfs"}\].*`)
}
