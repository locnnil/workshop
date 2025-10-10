package client_test

import (
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/client"
)

func (cs *clientSuite) TestClientSdks(c *check.C) {
	cs.rsp = `{"type": "sync", "result": [
		{"name": "ollama", "version": "1.0-053c828", "revision": "82", "summary": "Large language model runtime"},
		{"name": "ros2", "revision": "5", "summary": "ROS2 SDK"}
	]}`

	sdks, err := cs.cli.Sdks()
	c.Assert(err, check.IsNil)
	c.Assert(sdks, check.DeepEquals, []client.SdkVolume{
		{Name: "ollama", Version: "1.0-053c828", Revision: "82"},
		{Name: "ros2", Revision: "5"},
	})

	c.Check(cs.req.Method, check.Equals, "GET")
	c.Check(cs.req.URL.Path, check.Equals, "/v1/sdks")
}
