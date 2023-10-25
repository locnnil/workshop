package main

import (
	"fmt"
	"net/http"

	"gopkg.in/check.v1"
)

type WorkspaceInfo struct {
	BaseWorkspaceSuite
	prjDir string
	prjId  string
}

var _ = check.Suite(&WorkspaceInfo{})

func (m *WorkspaceInfo) SetUpTest(c *check.C) {
	m.prjDir = c.MkDir()
	m.prjId = "42424242"
	m.BaseWorkspaceSuite.SetUpTest(c)
}

var mockWorkspaceWithContent = `{"type":"sync","status-code":200,"status":"OK","result":{"name":"ws","base":"ubuntu@22.04","project-id":"42424242","state":"Error","content":[{"name":"go","channel":"latest/edge","revision":"1","install-time":"2017-03-22T09:01:00.0Z"}],"notes":["missing-file"]}}`

func (m *WorkspaceInfo) TestWorkspaceInfo(c *check.C) {
	cmd := &CmdInfo{}
	workshop := "ws"
	n := 0
	m.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		n++
		switch n {
		case 1:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, "/v1/projects")
			r := fmt.Sprintf(`{"type": "sync", "result": {"id":"%s","path":"%s"}}`, m.prjId, m.prjDir)
			fmt.Fprintln(w, r)
		case 2:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops/%s", m.prjId, workshop))
			w.WriteHeader(200)
			fmt.Fprintln(w, mockWorkspaceWithContent)
		default:
			c.Errorf("expected 2 calls, now on %d", n)
		}
	})

	err := cmd.Run(cmd.Command(), []string{workshop})
	c.Assert(err, check.IsNil)
	c.Assert(m.stdout.String(), check.Matches, fmt.Sprintf(`name:     ws
base:     ubuntu@22.04
project:  %s
status:   error
notes:    missing-file
content:
    go:
        channel:  latest/edge  2017-03-22  1
`, m.prjDir))
}
