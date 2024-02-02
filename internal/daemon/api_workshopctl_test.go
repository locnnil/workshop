package daemon

import (
	"bytes"
	"net/http"

	"gopkg.in/check.v1"
)

func (s *apiSuite) TestWorkshopCtlNoContext(c *check.C) {
	// Setup
	s.daemon(c)
	wctl := apiCmd("/v1/workshopctl")

	buf := bytes.NewBufferString(`{"context-id": "some-context"}`)

	req, err := s.createProjectsRequest("POST", "/v1/workshopctl", buf)
	c.Assert(err, check.IsNil)

	// Execute
	rsp := v1PostWorkshopCtl(wctl, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeError)
	c.Assert(rsp.Status, check.Equals, http.StatusNotFound)

	res, err := rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	c.Check(string(res), check.Matches, `.*cannot get workshop cookies.*`)
}
