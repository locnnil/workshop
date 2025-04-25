package main

import (
	"fmt"
	"net/http"
	"os/user"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/osutil"
)

type workshopRemove struct {
	BaseWorkshopSuite
	prjDir string
	prjId  string
	user   *user.User
}

var _ = check.Suite(&workshopRemove{})

func (m *workshopRemove) SetUpTest(c *check.C) {
	m.BaseWorkshopSuite.SetUpTest(c)

	m.prjDir = c.MkDir()
	m.prjId = "42424242"
	var err error
	m.user, err = osutil.UserMaybeSudoUser()
	c.Assert(err, check.IsNil)
}

// Test explanation:
//
//	cli ----> daemon   |   query project info              |    POST /v1/projects
//	daemon ----> cli   |   project.id == 42424242          |
//	cli ----> daemon   |   query workshop `ws` info        |    GET  /v1/projects/42424242/workshops/ws
//	daemon ----> cli   |   ws.status == "Waiting"          |
//	cli ----> daemon   |   abort the failed refresh        |    POST /v1/projects/42424242/workshops
//	daemon ----> cli   |   change.id == 41                 |
//	cli ----> daemon   |   query change 41 progress        |    GET  /v1/changes/41
//	daemon ----> cli   |   change 41 finished              |
//	cli ----> daemon   |   query workshop `ws-1` info      |    GET  /v1/projects/42424242/workshops/ws-1
//	daemon ----> cli   |   ws-1.status == "Ready"          |
//	cli ----> daemon   |   remove two workshops            |    POST /v1/projects/42424242/workshops
//	daemon ----> cli   |   change.id == 42                 |
//	cli ----> daemon   |   query change 42 progress        |    GET  /v1/changes/42
//	daemon ----> cli   |   change 42 finished              |
//
// 7 requests in total
func (m *workshopRemove) TestRemoveSuccess(c *check.C) {
	cmd := &CmdRemove{root: &CmdRoot{}}
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
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops/ws", m.prjId))
			w.WriteHeader(202)
			fmt.Fprintf(w, `{"type":"sync","status-code":202,"result":{"name":"ws","status":"Waiting"}}`)
		case 3:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			w.WriteHeader(202)
			fmt.Fprintln(w, `{"type":"async", "change": "41", "status-code": 202}`)
		case 4:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, "/v1/changes/41")
			fmt.Fprintln(w, mockReadyChangeJSON)
		case 5:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops/ws-1", m.prjId))
			w.WriteHeader(202)
			fmt.Fprintf(w, `{"type":"sync","status-code":202,"result":{"name":"ws-1","status":"Ready"}}`)
		case 6:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			w.WriteHeader(202)
			fmt.Fprintln(w, `{"type":"async", "change": "42", "status-code": 202}`)
		case 7:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, "/v1/changes/42")
			fmt.Fprintln(w, mockReadyChangeJSON)
		default:
			c.Errorf("expected 7 calls, now on %d", n)
		}
	})

	err := cmd.Run(cmd.Command(), []string{"ws", "ws-1", "ws"})
	c.Assert(err, check.IsNil)
	c.Assert(m.stdout.String(), check.Matches, `Reverting incomplete change for "ws"...\n"ws" removed\n"ws-1" removed\n`)
}
