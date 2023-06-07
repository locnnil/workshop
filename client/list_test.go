package client_test

import (
	"github.com/canonical/workspace/client"
	"gopkg.in/check.v1"
)

func (cs *clientSuite) TestClientListProjectWorkspaces(c *check.C) {
	cs.rsp = `{"type": "sync", "result": [{"name":"workspace","project-id":"42ws42ws","state":"Ready",
	"notes":["missing-project"],
	"content":[{"name":"go","channel":"latest/stable","revision":"453"}]
	}]}`
	prj, err := cs.cli.ListWorkspaces(&client.ListOptions{ProjectId: "42ws42ws"})
	c.Assert(err, check.IsNil)
	c.Assert(prj, check.DeepEquals, []*client.Workspace{
		{
			ProjectId: "42ws42ws",
			Name:      "workspace",
			State:     "Ready",
			Notes:     []string{"missing-project"},
			Content: []*client.Sdk{
				{Name: "go", Channel: "latest/stable", Revision: "453"},
			},
		},
	})
	c.Check(cs.req.Method, check.Equals, "GET")
}
