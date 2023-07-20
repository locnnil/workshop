package main

import (
	"fmt"
	"net/http"

	"gopkg.in/check.v1"
)

type WorkspaceRefresh struct {
	BaseWorkspaceSuite
	prjDir string
	prjId  string
}

var _ = check.Suite(&WorkspaceRefresh{})

var mockReadyChangeJSON = `{"type": "sync", "result":{
    "id":   "four",
    "kind": "refresh",
    "summary": "...",
    "status": "Done",
    "ready": true,
    "spawn-time": "2015-02-21T01:02:03Z",
    "ready-time": "2015-02-21T01:02:04Z",
    "tasks": [{"kind": "bar", "summary": "some summary", "status": "Done", "progress": {"done": 1, "total": 1}, "spawn-time": "2015-02-21T01:02:03Z", "ready-time": "2015-02-21T01:02:04Z"}]
}}`

var mockWaitChangeJSON = `{"type": "sync", "result":{
    "id":   "four",
    "kind": "refresh",
    "summary": "...",
    "status": "Wait",
    "ready": false,
    "spawn-time": "2015-02-21T01:02:03Z",
    "ready-time": "2015-02-21T01:02:04Z",
    "tasks": [{"kind": "bar", "summary": "some summary", "status": "Wait", "progress": {"done": 1, "total": 1}, "spawn-time": "2015-02-21T01:02:03Z", "ready-time": "2015-02-21T01:02:04Z"}]
}}`

var mockAbortedChangeJSON = `{"type": "sync", "result":{
    "id":   "four",
    "kind": "refresh",
    "summary": "...",
    "status": "Error",
    "ready": true,
    "spawn-time": "2015-02-21T01:02:03Z",
    "ready-time": "2015-02-21T01:02:04Z",
    "tasks": [{"kind": "bar", "summary": "some summary", "status": "Undone", "progress": {"done": 1, "total": 1}, "spawn-time": "2015-02-21T01:02:03Z", "ready-time": "2015-02-21T01:02:04Z"},{"kind": "foo", "summary": "some summary", "status": "Error", "progress": {"done": 1, "total": 1}, "spawn-time": "2015-02-21T01:02:03Z", "ready-time": "2015-02-21T01:02:04Z" , "log":["Aborting \"ws\" workspace refresh..."], "data":{"workspace":"ws"}}]
}}`

func (m *WorkspaceRefresh) SetUpTest(c *check.C) {
	m.prjDir = c.MkDir()
	m.prjId = "42424242"
	m.BaseWorkspaceSuite.SetUpTest(c)
}

func (m *WorkspaceRefresh) TestRefreshTransactionalSuccess(c *check.C) {
	cmd := &CmdRefresh{}
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
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workspaces", m.prjId))
			w.WriteHeader(202)
			fmt.Fprintln(w, `{"type":"async", "change": "42", "status-code": 202}`)
		case 3:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, "/v1/changes/42")
			fmt.Fprintln(w, mockReadyChangeJSON)
		default:
			c.Errorf("expected 4 calls, now on %d", n)
		}
	})

	err := cmd.Run(cmd.Command(), []string{"ws"})
	c.Assert(err, check.IsNil)
	c.Assert(m.stdout.String(), check.Matches, "\"ws\" refreshed\n")
}

func (m *WorkspaceRefresh) TestRefreshWaitOnErrorFailed(c *check.C) {
	cmd := &CmdRefresh{}
	cmd.WaitOnError = true

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
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workspaces", m.prjId))
			c.Check(DecodedRequestBody(c, r), check.DeepEquals, map[string]interface{}{"action": "refresh",
				"names": []interface{}{"ws"}, "options": map[string]interface{}{"refresh-mode": "wait-on-error"}})
			w.WriteHeader(202)
			fmt.Fprintln(w, `{"type":"async", "change": "42", "status-code": 202}`)
		case 3:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, "/v1/changes/42")
			fmt.Fprintln(w, mockWaitChangeJSON)
		default:
			c.Errorf("expected 3 calls, now on %d", n)
		}
	})

	err := cmd.Run(nil, []string{"ws"})
	c.Assert(err, check.NotNil)
	c.Assert(err, check.ErrorMatches, "\"ws\" refresh failed, resolve all errors and run \"workspace refresh --continue\".\nTo abort and get back to the state before run \"workspace refresh --abort\"")
}

func (m *WorkspaceRefresh) TestRefreshWaitOnErrorAbortedSuccessfully(c *check.C) {
	cmd := &CmdRefresh{}
	cmd.Abort = true

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
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workspaces", m.prjId))
			c.Check(DecodedRequestBody(c, r), check.DeepEquals, map[string]interface{}{"action": "refresh",
				"names": []interface{}{"ws"}, "options": map[string]interface{}{"refresh-mode": "abort"}})
			w.WriteHeader(202)
			fmt.Fprintln(w, `{"type":"async", "change": "42", "status-code": 202}`)
		case 3:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, "/v1/changes/42")
			fmt.Fprintln(w, mockAbortedChangeJSON)
		default:
			c.Errorf("expected 3 calls, now on %d", n)
		}
	})

	err := cmd.Run(nil, []string{"ws"})
	c.Assert(err, check.IsNil)
	c.Assert(m.stdout.String(), check.Matches, "\"ws\" refresh aborted\n")
}
