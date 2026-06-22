// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/client"
)

type workshopLaunch struct {
	BaseWorkshopSuite
	prjDir string
	prjId  string
}

var _ = check.Suite(&workshopLaunch{})

func (m *workshopLaunch) SetUpTest(c *check.C) {
	m.prjDir = c.MkDir()
	m.prjId = "42424242"
	m.BaseWorkshopSuite.SetUpTest(c)
}

func (m *workshopLaunch) TestLaunchSuccess(c *check.C) {
	cmd := &CmdLaunch{root: &CmdRoot{}}
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
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			w.WriteHeader(202)
			fmt.Fprintln(w, `{"type":"async", "change": "42", "status-code": 202}`)
		case 3:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, "/v1/changes/42")
			fmt.Fprintln(w, mockReadyChangeJSON)
		default:
			c.Errorf("expected 3 calls, now on %d", n)
		}
	})

	err := cmd.Run(cmd.Command(), []string{"ws", "ws-1", "ws"})
	c.Assert(err, check.IsNil)
	c.Assert(m.stdout.String(), check.Matches, `"ws", "ws-1" launched\n`)
}

func (m *workshopLaunch) TestLaunchWaitOnErrorFailed(c *check.C) {
	cmd := &CmdLaunch{root: &CmdRoot{}}
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
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			c.Check(DecodedRequestBody(c, r), check.DeepEquals, map[string]any{"action": "launch",
				"names": []any{"ws"}, "options": map[string]any{"mode": "wait-on-error"}})
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
	c.Assert(err, check.ErrorMatches, "cannot complete launch for \"ws\", execution is paused\n\n"+
		"To proceed, resolve the issue and run \"workshop launch --continue ws\"\n"+
		"To cancel and undo: \"workshop launch --abort ws\"\n"+
		"To view more information: \"workshop tasks 42\"")
}

func (m *workshopLaunch) TestLaunchWaitOnErrorAbortedSuccessfully(c *check.C) {
	cmd := &CmdLaunch{root: &CmdRoot{}}
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
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			c.Check(DecodedRequestBody(c, r), check.DeepEquals, map[string]any{"action": "launch",
				"names": []any{"ws"}, "options": map[string]any{"mode": "abort"}})
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
	c.Assert(m.stdout.String(), check.Matches, `"ws" launch aborted\n`)
}

// TestLaunchAbortNoWaitingChange checks that aborting with no paused launch
// reports the spec message built from the command's own context, without the
// generic launch-aborted wrapper.
func (m *workshopLaunch) TestLaunchAbortNoWaitingChange(c *check.C) {
	cmd := &CmdLaunch{root: &CmdRoot{}}
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
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			c.Check(DecodedRequestBody(c, r), check.DeepEquals, map[string]any{"action": "launch",
				"names": []any{"ws"}, "options": map[string]any{"mode": "abort"}})
			w.WriteHeader(400)
			fmt.Fprintln(w, `{"type":"error","status-code":400,"result":{"message":"cannot abort: no waiting change in progress","kind":"no-waiting-change-in-progress"}}`)
		default:
			c.Errorf("expected 2 calls, now on %d", n)
		}
	})

	err := cmd.Run(nil, []string{"ws"})
	c.Assert(err, check.NotNil)
	c.Check(err.Error(), check.Equals, "cannot abort: no launch in progress")
	c.Check(n, check.Equals, 2)
}

// TestLaunchAbortChangeConflict checks that aborting while another change is in
// progress reports the blocking change's kind.
func (m *workshopLaunch) TestLaunchAbortChangeConflict(c *check.C) {
	cmd := &CmdLaunch{root: &CmdRoot{}}
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
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			w.WriteHeader(400)
			fmt.Fprintln(w, `{"type":"error","status-code":400,"result":{"message":"workshop \"ws\" has \"refresh\" change in progress","kind":"change-conflict","value":{"change-kind":"refresh","workshop":"ws"}}}`)
		default:
			c.Errorf("expected 2 calls, now on %d", n)
		}
	})

	err := cmd.Run(nil, []string{"ws"})
	c.Assert(err, check.NotNil)
	c.Check(err.Error(), check.Equals, `cannot launch "ws": refresh change is in progress`)
	c.Check(n, check.Equals, 2)
}

func (m *workshopLaunch) TestLaunchWaitOnErrorContinuedSuccessfully(c *check.C) {
	cmd := &CmdLaunch{root: &CmdRoot{}}
	cmd.Continue = true

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
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			c.Check(DecodedRequestBody(c, r), check.DeepEquals, map[string]any{"action": "launch",
				"names": []any{"ws"}, "options": map[string]any{"mode": "continue"}})
			w.WriteHeader(202)
			fmt.Fprintln(w, `{"type":"async", "change": "42", "status-code": 202}`)
		case 3:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, "/v1/changes/42")
			fmt.Fprintln(w, mockReadyChangeJSON)
		default:
			c.Errorf("expected 3 calls, now on %d", n)
		}
	})

	err := cmd.Run(nil, []string{"ws"})
	c.Assert(err, check.IsNil)
	c.Assert(m.stdout.String(), check.Matches, `"ws" launched\n`)
}

func (m *workshopLaunch) TestLaunchCompletions(c *check.C) {
	wsInfo := []*client.WorkshopInfo{
		{
			ProjectId: m.prjId,
			Name:      "workshop-exists",
			Status:    "Ready",
		},
	}

	wsFile := []*client.WorkshopFile{
		{
			ProjectId: m.prjId,
			Name:      "workshop-exists",
			Path:      "/tmp/workshop",
		},
		{
			ProjectId: m.prjId,
			Name:      "workshop-notexists",
			Path:      "/tmp/workshop1",
		},
	}

	w := client.Workshops{
		Workshops: wsInfo,
		Files:     wsFile,
	}

	cmd := CmdLaunch{
		root: &CmdRoot{},
	}

	m.listRedirectHelper(c, w, m.prjId, m.prjDir, len(wsFile))

	completions, compDirective := cmd.complete(cmd.Command(), nil, "")
	c.Assert(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
	c.Check(completions, check.DeepEquals, []string{"workshop-notexists"})
}
