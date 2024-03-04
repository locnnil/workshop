package daemon

import (
	"bytes"
	"fmt"

	"gopkg.in/check.v1"
)

func (s *apiSuite) TestWorkshopRemount(c *check.C) {
	// Setup
	s.daemon(c)
	command := apiCmd("/v1/projects/{id}/workshops/{name}/mounts")

	buf := bytes.NewBufferString(`{"action":"remount","plug":{"sdk":"go","name":"cache"},"source":"/srv/data"}`)

	req, err := s.createProjectsRequest("POST", fmt.Sprintf("/v1/projects/%s/workshops/%s/mounts", s.project.ProjectId, "ws"), buf)
	c.Assert(err, check.IsNil)

	// Execute
	rsp := v1PostWorkshopMount(command, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeError)
}
