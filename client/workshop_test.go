package client_test

import (
	"fmt"
	"io"
	"time"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/client"
)

func (cs *clientSuite) TestClientListProjectWorkshops(c *check.C) {
	cs.rsp = `{"type": "sync", "result": [{"name":"workshop","base":"ubuntu@20.04","project-id":"42ws42ws","status":"Ready","notes":["missing-project"],
	"content":[
		{"name":"go","channel":"latest/stable","revision":"453","install-time":"2023-04-25T01:02:03Z", 
		"health-check":{"timestamp":"2023-04-25T01:02:03Z", "message":"hello from health-check", "code":"check-waiting"}}]
	}]}`
	prj, err := cs.cli.ListWorkshops(&client.ListOptions{ProjectId: "42ws42ws"})
	c.Assert(err, check.IsNil)
	c.Assert(prj, check.DeepEquals, []*client.Workshop{
		{
			ProjectId: "42ws42ws",
			Name:      "workshop",
			Base:      "ubuntu@20.04",
			Status:    "Ready",
			Notes:     []string{"missing-project"},
			Content: []*client.Sdk{
				{
					Name:        "go",
					Channel:     "latest/stable",
					Revision:    "453",
					InstallTime: time.Date(2023, 04, 25, 1, 2, 3, 0, time.UTC),
					Health: &client.HealthCheck{
						Timestamp: time.Date(2023, 04, 25, 1, 2, 3, 0, time.UTC),
						Message:   "hello from health-check",
						Code:      "check-waiting",
					},
				},
			},
		},
	})
	c.Check(cs.req.Method, check.Equals, "GET")
}
func (cs *clientSuite) TestClientProjectWorkshop(c *check.C) {
	cs.rsp = `{"type": "sync", "result": {"name":"workshop","base":"ubuntu@20.04","project-id":"42ws42ws","status":"Ready",
	"content":[
		{"name":"go",
		"channel":"latest/stable",
		"revision":"453",
		"install-time":"2023-04-25T01:02:03Z", 
		"health-check":{"timestamp":"2023-04-25T01:02:03Z", "message":"hello from health-check", "code":"check-waiting"},
		"mounts":[{"host-source":"/home/user/src","workshop-target":"/home/workshop/target", "plug":{"project-id":"42ws42ws","workshop":"workshop","sdk":"go","plug":"plug-name"}}]
	}]}}`
	prj, err := cs.cli.Workshop("42ws42ws", "workshop")
	c.Assert(err, check.IsNil)
	c.Assert(prj, check.DeepEquals, &client.Workshop{
		ProjectId: "42ws42ws",
		Name:      "workshop",
		Base:      "ubuntu@20.04",
		Status:    "Ready",
		Content: []*client.Sdk{
			{
				Name:        "go",
				Channel:     "latest/stable",
				Revision:    "453",
				InstallTime: time.Date(2023, 04, 25, 1, 2, 3, 0, time.UTC),
				Health: &client.HealthCheck{
					Timestamp: time.Date(2023, 04, 25, 1, 2, 3, 0, time.UTC),
					Message:   "hello from health-check",
					Code:      "check-waiting",
				},
				Mounts: []*client.Mount{
					{HostSource: "/home/user/src", WorkshopTarget: "/home/workshop/target", Plug: client.PlugRef{ProjectId: "42ws42ws", Workshop: "workshop", Sdk: "go", Name: "plug-name"}},
				},
			},
		},
	})
	c.Check(cs.req.Method, check.Equals, "GET")
}

func (cs *clientSuite) TestRemountRequest(c *check.C) {
	cs.rsp = `{"type": "async", "status-code": 202, "change": "24"}`

	source := c.MkDir()
	id, err := cs.cli.Remount(&client.PlugRef{ProjectId: "4242", Workshop: "ws", Sdk: "sdk", Name: "plug"}, source)

	c.Check(cs.req.Method, check.Equals, "POST")
	c.Assert(id, check.Equals, "24")
	c.Assert(err, check.IsNil)

	body, err := io.ReadAll(cs.req.Body)
	c.Assert(err, check.IsNil)

	c.Assert(string(body), check.Matches, fmt.Sprintf(`{"action":"remount","plug":{"project-id":"4242","workshop":"ws","sdk":"sdk","plug":"plug"},"source":%q}\n`, source))
}
