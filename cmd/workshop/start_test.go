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

	"gopkg.in/check.v1"
)

type workshopStart struct {
	BaseWorkshopSuite
	prjDir string
	prjId  string
}

var _ = check.Suite(&workshopStart{})

func (m *workshopStart) SetUpTest(c *check.C) {
	m.prjDir = c.MkDir()
	m.prjId = "42424242"
	m.BaseWorkshopSuite.SetUpTest(c)
}

// TestStartChangeConflictInProgress checks that start explains a blocking
// in-progress change and points the user to the task details.
func (m *workshopStart) TestStartChangeConflictInProgress(c *check.C) {
	cmd := &CmdStart{root: &CmdRoot{}}
	n := 0
	m.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		n++
		switch n {
		case 1:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, "/v1/projects")
			r := fmt.Sprintf(
				`{"type": "sync", "result": {"id":"%s","path":"%s"}}`,
				m.prjId,
				m.prjDir,
			)
			fmt.Fprintln(w, r)
		case 2:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(
				r.URL.Path,
				check.Equals,
				fmt.Sprintf("/v1/projects/%s/workshops", m.prjId),
			)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, `{
				"type": "error",
				"status-code": 400,
				"result": {
					"kind": "change-conflict",
					"message": "cannot start \\\"dev\\\": change is in progress",
					"value": {
						"change-id": "30",
						"change-kind": "launch",
						"change-status": "Do",
						"project-id": "42424242",
						"workshop": "dev"
					}
				}
			}`)
		default:
			c.Errorf("expected 2 calls, now on %d", n)
		}
	})

	err := cmd.Run(cmd.Command(), []string{"dev"})
	c.Assert(
		err,
		check.ErrorMatches,
		`cannot start "dev": change launch is in progress`,
	)
}

// TestStartRefreshConflictWaiting checks that start explains a blocked refresh
// waiting on error and shows the recovery commands.
func (m *workshopStart) TestStartRefreshConflictWaiting(c *check.C) {
	cmd := &CmdStart{root: &CmdRoot{}}
	n := 0
	m.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		n++
		switch n {
		case 1:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, "/v1/projects")
			r := fmt.Sprintf(
				`{"type": "sync", "result": {"id":"%s","path":"%s"}}`,
				m.prjId,
				m.prjDir,
			)
			fmt.Fprintln(w, r)
		case 2:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(
				r.URL.Path,
				check.Equals,
				fmt.Sprintf("/v1/projects/%s/workshops", m.prjId),
			)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, `{
				"type": "error",
				"status-code": 400,
				"result": {
					"kind": "change-conflict",
					"message": "cannot start \\\"dev\\\": waiting on error",
					"value": {
						"change-id": "29",
						"change-kind": "refresh",
						"change-status": "Wait",
						"project-id": "42424242",
						"workshop": "dev"
					}
				}
			}`)
		default:
			c.Errorf("expected 2 calls, now on %d", n)
		}
	})

	err := cmd.Run(cmd.Command(), []string{"dev"})
	c.Assert(err, check.ErrorMatches, `cannot start "dev": refresh change is waiting on error`)
}

func (m *workshopStart) TestStartSuccess(c *check.C) {
	cmd := &CmdStart{root: &CmdRoot{}}
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
	c.Assert(m.stdout.String(), check.Matches, `"ws" started\n"ws-1" started\n`)
}
