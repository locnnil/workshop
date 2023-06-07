package client_test

import (
	"io"

	"github.com/canonical/workspace/client"
	"gopkg.in/check.v1"
)

func (cs *clientSuite) TestClientProject(c *check.C) {
	cs.rsp = `{"type": "sync", "result": {
			"id":   "42ws42ws",
			"path": "/home/francua/workspace"}
		  }`
	prj, err := cs.cli.Project("/home/francua/workspace")
	c.Assert(err, check.IsNil)
	c.Assert(prj, check.DeepEquals, &client.ProjectResponse{"42ws42ws", "/home/francua/workspace"})
	c.Check(cs.req.Method, check.Equals, "POST")

	body, err := io.ReadAll(cs.req.Body)
	c.Assert(err, check.IsNil)

	c.Assert(string(body), check.Equals, "{\"path\":\"/home/francua/workspace\"}\n")
}
