package daemon

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/testutil"
	"gopkg.in/check.v1"
)

func (s *apiSuite) setupExec(c *check.C) *Command {
	s.daemon(c)
	projectsCmd := apiCmd("/v1/projects/{id}/workspaces/{name}/exec")

	s.vars = map[string]string{"id": s.project.ProjectId, "name": "ws"}
	os.WriteFile(filepath.Join(s.workspaceDir, ".workspace.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04`), 0644)

	err := s.b.LaunchWorkspace(s.ctx, "ws", "ubuntu@20.04")
	c.Assert(err, check.IsNil)
	return projectsCmd
}

func (s *apiSuite) TestExecSuccess(c *check.C) {
	// Setup
	projectsCmd := s.setupExec(c)

	body := bytes.NewBufferString(`{"command":["ls"]}`)

	req, err := s.createProjectsRequest("POST", "/v1/projects/"+s.project.ProjectId+"/workspaces/ws/exec", body)
	c.Assert(err, check.IsNil)

	soon := 0
	restoreEnsure := testutil.FakeFunc(func(st *state.State, d time.Duration) {
		soon++
	}, &ensureStateSoon)
	defer restoreEnsure()

	// Execute
	rsp := v1PostWorkspaceExec(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Status, check.Equals, http.StatusAccepted)
	c.Assert(soon, check.Equals, 1)
}
