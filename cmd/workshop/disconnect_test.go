package main

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/spf13/cobra"
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/client"
)

type disconnectSuite struct {
	BaseWorkshopSuite
	prjDir string
	prjId  string
}

var _ = check.Suite(&disconnectSuite{})

func (m *disconnectSuite) SetUpTest(c *check.C) {
	m.prjDir = c.MkDir()
	m.prjId = "42424242"
	m.BaseWorkshopSuite.SetUpTest(c)
}

func (m *disconnectSuite) TestDisconnectPlugAndSlotProvided(c *check.C) {
	cmd := &CmdDisconnect{root: &CmdRoot{}}
	body := map[string]interface{}{
		"action": "disconnect",
		"plugs": []interface{}{
			map[string]interface{}{
				"project-id": "42424242",
				"workshop":   "ws",
				"sdk":        "sdk",
				"plug":       "plug",
			},
		},
		"slots": []interface{}{
			map[string]interface{}{
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
	body = map[string]interface{}{
		"action": "disconnect",
		"plugs": []interface{}{
			map[string]interface{}{
				"project-id": "42424242",
				"workshop":   "ws",
				"sdk":        "sdk",
				"plug":       "plug2",
			},
		},
		"slots": []interface{}{
			map[string]interface{}{
				"project-id": "42424242",
				"workshop":   "ws",
				"sdk":        "sdk-2",
				"slot":       "producer",
			},
		},
	}

	err = cmd.Run(cmd.Command(), []string{"ws/sdk:plug2", "ws/sdk-2:producer"})
	c.Assert(err, check.IsNil)
}

func (m *disconnectSuite) TestDisconnectPlugOrSlotProvided(c *check.C) {
	cmd := &CmdDisconnect{root: &CmdRoot{}}

	n := 0
	body := map[string]interface{}{
		"action": "disconnect",
		"plugs": []interface{}{
			map[string]interface{}{
				"project-id": "42424242",
				"workshop":   "ws",
				"sdk":        "sdk",
				"plug":       "plug",
			},
		},
		"slots": []interface{}{
			map[string]interface{}{
				"project-id": "",
				"workshop":   "",
				"sdk":        "",
				"slot":       "",
			},
		},
	}
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

func (m *disconnectSuite) TestDisconnectCompletionAllDisconnected(c *check.C) {
	plugs, slots := testPlugsSlots(m.prjId)

	conns := client.Connections{
		Established: nil,
		Plugs:       plugs,
		Slots:       slots,
	}

	cmd := CmdDisconnect{
		root: &CmdRoot{},
	}

	m.connectionsRedirectHelper(c, conns, m.prjId, m.prjDir, 10)

	completions, compDirective := cmd.complete(cmd.Command(), nil, "")
	c.Assert(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
	c.Assert(len(completions), check.Equals, 0)

	// Test slots
	for _, plug := range plugs {
		completions, compDirective := cmd.complete(cmd.Command(), []string{endpoint(plug.Workshop, plug.Sdk, plug.Name)}, "")
		c.Assert(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
		c.Assert(len(completions), check.Equals, 0)
	}
}

func (m *disconnectSuite) TestDisconnectCompletionSomeDisconnected(c *check.C) {
	plugs, slots := testPlugsSlots(m.prjId)
	established := []client.Connection{
		plugSlotToConn(plugs[0], slots[0], true),
		plugSlotToConn(plugs[1], slots[1], true),
	}

	conns := client.Connections{
		Established: established,
		Plugs:       plugs,
		Slots:       slots,
	}

	expected := []string{
		"workshop/sdk:desktop",
		"workshop/sdk:ssh-agent",
	}

	cmd := CmdDisconnect{
		root: &CmdRoot{},
	}

	m.connectionsRedirectHelper(c, conns, m.prjId, m.prjDir, 10)

	completions, compDirective := cmd.complete(cmd.Command(), nil, "")
	c.Assert(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
	for _, completion := range completions {
		c.Check(slices.Contains(expected, completion), check.Equals, true)
	}

	// Test slots
	for _, plug := range plugs {
		completions, compDirective := cmd.complete(cmd.Command(), []string{endpoint(plug.Workshop, plug.Sdk, plug.Name)}, "")
		c.Assert(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
		if len(completions) == 1 {
			c.Check(slices.Contains(expected, completions[0]), check.Equals, true)
			break
		}
		c.Assert(len(completions), check.Equals, 0)
	}
}

func (m *disconnectSuite) TestDisconnectCompletionAllConnected(c *check.C) {
	plugs, slots := testPlugsSlots(m.prjId)
	established := []client.Connection{
		plugSlotToConn(plugs[0], slots[0], true),
		plugSlotToConn(plugs[1], slots[1], true),
		plugSlotToConn(plugs[2], slots[2], true),
		plugSlotToConn(plugs[3], slots[3], true),
	}

	conns := client.Connections{
		Established: established,
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

	m.connectionsRedirectHelper(c, conns, m.prjId, m.prjDir, 10)

	cmd := CmdDisconnect{
		root: &CmdRoot{},
	}

	completions, compDirective := cmd.complete(cmd.Command(), nil, "")
	c.Assert(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
	for _, completion := range completions {
		c.Check(slices.Contains(expected, completion), check.Equals, true)
	}

	// Test slots
	for _, plug := range plugs {
		completions, compDirective := cmd.complete(cmd.Command(), []string{endpoint(plug.Workshop, plug.Sdk, plug.Name)}, "")
		c.Assert(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
		if len(completions) == 1 {
			c.Check(slices.Contains(expected, completions[0]), check.Equals, true)
			break
		}
		c.Assert(len(completions), check.Equals, 0)
	}
}

func (m *disconnectSuite) TestDisconnectCompletionWorkshopMismatch(c *check.C) {
	plugs := []client.Plug{
		{
			ProjectId: m.prjId,
			Workshop:  "workshop",
			Sdk:       "sdk",
			Name:      "desktop",
			Interface: "desktop",
		},
		{
			ProjectId: m.prjId,
			Workshop:  "another-workshop",
			Sdk:       "sdk",
			Name:      "desktop",
			Interface: "desktop",
		},
	}

	slots := []client.Slot{
		{
			ProjectId: m.prjId,
			Workshop:  "workshop",
			Sdk:       "sdk",
			Name:      "desktop",
			Interface: "desktop",
		},
		{
			ProjectId: m.prjId,
			Workshop:  "another-workshop",
			Sdk:       "sdk",
			Name:      "desktop",
			Interface: "desktop",
		},
	}

	conns := client.Connections{
		Established: []client.Connection{plugSlotToConn(plugs[0], slots[0], true)},
		Plugs:       plugs,
		Slots:       slots,
	}

	m.connectionsRedirectHelper(c, conns, m.prjId, m.prjDir, 4)

	cmd := CmdDisconnect{
		root: &CmdRoot{},
	}

	completions, compDirective := cmd.complete(cmd.Command(), []string{"another-workshop/sdk:desktop"}, "")
	c.Assert(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
	c.Check(len(completions), check.Equals, 0)
}
