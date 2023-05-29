package client_test

import (
	"gopkg.in/check.v1"
)

func (cs *clientSuite) TestClientSingleProjectId(c *check.C) {
	projects := []struct {
		rsp     string
		in, out string
		outerr  string
	}{
		{`{"type": "sync", "result": [{
			"id":   "42ws42ws",
			"path": "/home/francua/workspace"}]
		  }`, "/home/francua/workspace", "42ws42ws", ""},
		{`{"type": "sync", "result": []
		  }`, "/home/francua", "", "cannot get an unambigous project id for \"/home/francua\""},
	}

	for _, i := range projects {
		cs.rsp = i.rsp
		prj, err := cs.cli.ProjectId(i.in)
		if i.outerr != "" {
			c.Assert(err, check.ErrorMatches, i.outerr)
		} else {
			c.Assert(err, check.IsNil)
		}
		c.Assert(prj, check.Equals, i.out)
	}
}
