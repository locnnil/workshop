package main

import (
	"fmt"
	"net/http"

	"gopkg.in/check.v1"
)

type WorkshopInfo struct {
	BaseWorkshopSuite
	prjDir string
	prjId  string
}

var _ = check.Suite(&WorkshopInfo{})

func (m *WorkshopInfo) SetUpTest(c *check.C) {
	m.prjDir = c.MkDir()
	m.prjId = "42424242"
	m.BaseWorkshopSuite.SetUpTest(c)
}

var mockWorkshopWithContent = `{"type":"sync","status-code":200,"status":"OK","result":{"name":"ws","base":"ubuntu@22.04","project-id":"42424242","status":"Error","content":[{"name":"go","channel":"latest/edge","revision":"1","install-time":"2017-03-22T09:01:00.0Z"}],"notes":["missing-file"]}}`

func (m *WorkshopInfo) TestWorkshopInfo(c *check.C) {
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
			fmt.Fprintln(w, mockWorkshopWithContent)
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

var mockWorkshopWithHealth = `{"type":"sync","status-code":200,"status":"OK","result":{"name":"ws","base":"ubuntu@22.04","project-id":"42424242","status":"Pending","notes":["workshop-note"],"content":[{"name":"go","channel":"latest/edge","revision":"1","install-time":"2017-03-22T09:01:00.0Z","health-check":{"message":"Waiting for all required modules to be installed","code":"try-later"}}]}}`

func (m *WorkshopInfo) TestWorkshopInfoWithSdkHealthReport(c *check.C) {
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
			fmt.Fprintln(w, mockWorkshopWithHealth)
		default:
			c.Errorf("expected 2 calls, now on %d", n)
		}
	})

	err := cmd.Run(cmd.Command(), []string{workshop})
	c.Assert(err, check.IsNil)
	c.Assert(m.stdout.String(), check.Matches, fmt.Sprintf(`name:     ws
base:     ubuntu@22.04
project:  %s
status:   pending
notes:    workshop-note,try-later
content:
    go:
        channel:  latest/edge  2017-03-22  1
        message:  Waiting for all required modules to be installed
`, m.prjDir))
}
