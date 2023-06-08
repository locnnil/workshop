package client_test

import (
	"io"

	"github.com/canonical/workspace/client"
	"gopkg.in/check.v1"
)

func (cs *clientSuite) TestClientProjects(c *check.C) {
	cs.rsp = `{"type": "sync", "result": 
			[{"id":   "42ws42ws","path": "/home/francua/workspace"},{"id":"34hg34gh",
			"path": "/home/francua/test"}]}`
	prjs, err := cs.cli.Projects()
	c.Assert(err, check.IsNil)
	c.Assert(prjs, check.DeepEquals, []*client.Project{{"42ws42ws", "/home/francua/workspace"}, {"34hg34gh", "/home/francua/test"}})
	c.Check(cs.req.Method, check.Equals, "GET")
}

func (cs *clientSuite) TestClientProject(c *check.C) {
	cs.rsp = `{"type": "sync", "result": {
			"id":   "42ws42ws",
			"path": "/home/francua/workspace"}
		  }`
	prj, err := cs.cli.Project("/home/francua/workspace")
	c.Assert(err, check.IsNil)
	c.Assert(prj, check.DeepEquals, &client.Project{"42ws42ws", "/home/francua/workspace"})
	c.Check(cs.req.Method, check.Equals, "POST")

	body, err := io.ReadAll(cs.req.Body)
	c.Assert(err, check.IsNil)

	c.Assert(string(body), check.Equals, "{\"path\":\"/home/francua/workspace\"}\n")
}

func (cs *clientSuite) TestClientLaunch(c *check.C) {
	cs.rsp = `{"type": "async", "status-code": 202, "change": "24"}`

	id, err := cs.cli.Launch("42ws42ws", []string{"ws"})

	c.Check(cs.req.Method, check.Equals, "POST")
	c.Assert(id, check.Equals, "24")
	c.Assert(err, check.IsNil)

	body, err := io.ReadAll(cs.req.Body)
	c.Assert(err, check.IsNil)

	c.Assert(string(body), check.Matches, `{"names":\["ws"\],"action":"launch"}\n`)
}
