package daemon

import (
	"bytes"
	"net/http"

	"gopkg.in/check.v1"
)

func (s *apiSuite) TestWorkshopHelpCtlNoContext(c *check.C) {
	// Setup
	s.daemon(c)
	wctl := apiCmd("/v1/workshopctl")

	buf := bytes.NewBufferString(`{"args":["-h"]}`)

	req, err := s.createProjectsRequest("POST", "/v1/workshopctl", buf)
	c.Assert(err, check.IsNil)

	// Execute
	rsp := v1PostWorkshopCtl(wctl, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	_, err = rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
}
