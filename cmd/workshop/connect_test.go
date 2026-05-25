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
	"slices"

	"github.com/spf13/cobra"
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/client"
)

type connectSuite struct {
	BaseWorkshopSuite
	prjDir string
	prjId  string
}

var _ = check.Suite(&connectSuite{})

func (m *connectSuite) SetUpTest(c *check.C) {
	m.prjDir = c.MkDir()
	m.prjId = "42424242"
	m.BaseWorkshopSuite.SetUpTest(c)
}

func (m *connectSuite) TestConnectAcrossWorkshops(c *check.C) {
	cmd := &CmdConnect{root: &CmdRoot{}}

	n := 0
	m.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		n++
		switch n {
		case 1:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, "/v1/projects")
			r := fmt.Sprintf(`{"type": "sync", "result": {"id":"%s","path":"%s"}}`, m.prjId, m.prjDir)
			fmt.Fprintln(w, r)
		default:
			c.Errorf("expected 1 call, now on %d", n)
		}
	})

	err := cmd.Run(cmd.Command(), []string{"ws/sdk:plug", "ws2/sdk:slot"})
	c.Assert(err, check.ErrorMatches, "cannot connect plugs and slots across different workshops")
}

func (m *connectSuite) TestDisconnectPlugAndSlotProvided(c *check.C) {
	cmd := &CmdConnect{root: &CmdRoot{}}
	body := map[string]any{
		"action": "connect",
		"plugs": []any{
			map[string]any{
				"project-id": "42424242",
				"workshop":   "ws",
				"sdk":        "sdk",
				"plug":       "plug",
			},
		},
		"slots": []any{
			map[string]any{
				"project-id": "42424242",
				"workshop":   "ws",
				"sdk":        "system",
				"slot":       "mount",
			},
		},
	}

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
			c.Assert(r.URL.Path, check.Equals, "/v1/connections")
			c.Check(DecodedRequestBody(c, r), check.DeepEquals, body)
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

	err := cmd.Run(cmd.Command(), []string{"ws/sdk:plug", ":mount"})
	c.Assert(err, check.IsNil)

	n = 0
	body = map[string]any{
		"action": "connect",
		"plugs": []any{
			map[string]any{
				"project-id": "42424242",
				"workshop":   "ws",
				"sdk":        "sdk",
				"plug":       "plug2",
			},
		},
		"slots": []any{
			map[string]any{
				"project-id": "42424242",
				"workshop":   "ws",
				"sdk":        "sdk-2",
				"slot":       "producer",
			},
		},
	}

	err = cmd.Run(cmd.Command(), []string{"ws/sdk:plug2", "ws/sdk-2:producer"})
	c.Assert(err, check.IsNil)

	n = 0
	body = map[string]any{
		"action": "connect",
		"plugs": []any{
			map[string]any{
				"project-id": "42424242",
				"workshop":   "ws",
				"sdk":        "sdk",
				"plug":       "plug2",
			},
		},
		"slots": []any{
			map[string]any{
				"project-id": "42424242",
				"workshop":   "ws",
				"sdk":        "sdk-2",
				"slot":       "",
			},
		},
	}

	err = cmd.Run(cmd.Command(), []string{"ws/sdk:plug2", "ws/sdk-2"})
	c.Assert(err, check.IsNil)
}

func (m *connectSuite) TestDisconnectSlotNotProvided(c *check.C) {
	cmd := &CmdConnect{root: &CmdRoot{}}
	body := map[string]any{
		"action": "connect",
		"plugs": []any{
			map[string]any{
				"project-id": "42424242",
				"workshop":   "ws",
				"sdk":        "sdk",
				"plug":       "plug",
			},
		},
		"slots": []any{
			map[string]any{
				"project-id": "42424242",
				"workshop":   "ws",
				"sdk":        "system",
				"slot":       "plug",
			},
		},
	}

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
			c.Assert(r.URL.Path, check.Equals, "/v1/connections")
			c.Check(DecodedRequestBody(c, r), check.DeepEquals, body)
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

	err := cmd.Run(cmd.Command(), []string{"ws/sdk:plug"})
	c.Assert(err, check.IsNil)
}

func (m *connectSuite) TestConnectCompletionAllDisconnected(c *check.C) {
	plugs, slots := testPlugsSlots(m.prjId)
	conns := client.Connections{
		Established: nil,
		Plugs:       plugs,
		Slots:       slots,
	}

	expected := []string{
		"workshop/sdk:ssh-agent",
		"workshop/sdk:mount",
		"workshop/another-sdk:desktop",
		"another-workshop/sdk:desktop",
		"workshop/sdk:desktop",
	}

	cmd := CmdConnect{
		root: &CmdRoot{},
	}

	m.connectionsRedirectHelper(c, conns, m.prjId, m.prjDir, 10)

	completions, compDirective := cmd.complete(cmd.Command(), nil, "")
	c.Assert(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
	for _, completion := range completions {
		c.Check(slices.Contains(expected, completion), check.Equals, true)
	}

	// Test slots
	for i, plug := range conns.Plugs {
		completions, compDirective := cmd.complete(cmd.Command(), []string{endpoint(plug.Workshop, plug.Sdk, plug.Name)}, "")
		c.Assert(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
		c.Assert(completions, check.DeepEquals, []string{endpoint(conns.Slots[i].Workshop, conns.Slots[i].Sdk, conns.Slots[i].Name)})
	}
}

func (m *connectSuite) TestConnectCompletionSomeDisconnected(c *check.C) {
	plugs, slots := testPlugsSlots(m.prjId)

	plugs[0].Connections = []client.SlotRef{{}}
	plugs[1].Connections = []client.SlotRef{{}}

	conns := client.Connections{
		Established: nil,
		Plugs:       plugs,
		Slots:       slots,
	}

	m.connectionsRedirectHelper(c, conns, m.prjId, m.prjDir, 10)

	cmd := CmdConnect{
		root: &CmdRoot{},
	}

	expected := []string{
		"workshop/sdk:mount",
		"another-workshop/sdk:desktop",
	}

	completions, compDirective := cmd.complete(cmd.Command(), nil, "")
	c.Assert(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
	for _, completion := range completions {
		c.Check(slices.Contains(expected, completion), check.Equals, true)
	}

	// Test slots
	var slotCompletions []string
	for _, plug := range plugs {
		completions, compDirective := cmd.complete(cmd.Command(), []string{endpoint(plug.Workshop, plug.Sdk, plug.Name)}, "")
		c.Assert(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
		slotCompletions = append(slotCompletions, completions...)
	}
	c.Check(slotCompletions, check.DeepEquals, []string{"workshop/sdk:mount", "another-workshop/sdk:desktop"})
}

func (m *connectSuite) TestConnectCompletionAllConnected(c *check.C) {
	plugs, slots := testPlugsSlots(m.prjId)

	for i := range plugs {
		plugs[i].Connections = []client.SlotRef{{}}
	}

	conns := client.Connections{
		Established: nil,
		Plugs:       plugs,
		Slots:       slots,
	}

	m.connectionsRedirectHelper(c, conns, m.prjId, m.prjDir, 10)

	cmd := CmdConnect{
		root: &CmdRoot{},
	}

	completions, compDirective := cmd.complete(cmd.Command(), nil, "")
	c.Assert(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
	c.Check(len(completions), check.Equals, 0)

	// Test slots
	for _, plug := range plugs {
		completions, compDirective := cmd.complete(cmd.Command(), []string{endpoint(plug.Workshop, plug.Sdk, plug.Name)}, "")
		c.Assert(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
		c.Check(len(completions), check.Equals, 0)
	}
}

func (m *connectSuite) TestConnectCompletionWorkshopMismatch(c *check.C) {
	plugs := []client.Plug{
		{
			ProjectId: m.prjId,
			Workshop:  "workshop",
			Sdk:       "sdk",
			Name:      "desktop",
			Interface: "desktop",
		},
	}

	slots := []client.Slot{
		{
			ProjectId: m.prjId,
			Workshop:  "another-workshop",
			Sdk:       "sdk",
			Name:      "desktop",
			Interface: "desktop",
		},
	}

	conns := client.Connections{
		Established: nil,
		Plugs:       plugs,
		Slots:       slots,
	}

	m.connectionsRedirectHelper(c, conns, m.prjId, m.prjDir, 2)

	cmd := CmdConnect{
		root: &CmdRoot{},
	}

	completions, compDirective := cmd.complete(cmd.Command(), []string{"workshop/sdk:desktop"}, "")
	c.Assert(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
	c.Check(len(completions), check.Equals, 0)
}
