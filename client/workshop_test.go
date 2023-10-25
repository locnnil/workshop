package client_test

import (
	"time"

	"github.com/canonical/workshop/client"
	"gopkg.in/check.v1"
)

func (cs *clientSuite) TestClientListProjectWorkshops(c *check.C) {
	cs.rsp = `{"type": "sync", "result": [{"name":"workshop","base":"ubuntu@20.04","project-id":"42ws42ws","state":"Ready",
	"notes":["missing-project"],
	"content":[{"name":"go","channel":"latest/stable","revision":"453","install-time":"2023-04-25T01:02:03Z"}]
	}]}`
	prj, err := cs.cli.ListWorkshops(&client.ListOptions{ProjectId: "42ws42ws"})
	c.Assert(err, check.IsNil)
	c.Assert(prj, check.DeepEquals, []*client.Workshop{
		{
			ProjectId: "42ws42ws",
			Name:      "workshop",
			Base:      "ubuntu@20.04",
			State:     "Ready",
			Notes:     []string{"missing-project"},
			Content: []*client.Sdk{
				{Name: "go", Channel: "latest/stable", Revision: "453", InstallTime: time.Date(2023, 04, 25, 1, 2, 3, 0, time.UTC)},
			},
		},
	})
	c.Check(cs.req.Method, check.Equals, "GET")
}
