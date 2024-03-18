// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package client_test

import (
	"net/url"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/client"
)

func (cs *clientSuite) TestClientConnectionsCallsEndpoint(c *check.C) {
	_, _ = cs.cli.Connections(nil)
	c.Check(cs.req.Method, check.Equals, "GET")
	c.Check(cs.req.URL.Path, check.Equals, "/v1/connections")
}

func (cs *clientSuite) TestClientConnectionsDefault(c *check.C) {
	cs.rsp = `{
		"type": "sync",
		"result": {
			"established": [
				{
					"slot": {"project-id":"b8639dea", "workshop": "keyboard-lights", "sdk": "lights",  "slot": "capslock-led"},
					"plug": {"project-id":"b8639dea", "workshop": "canonical-pi2", "sdk": "pi", "plug": "pin-13"},
					"interface": "bool-file"
                }
			],
			"plugs": [
				{
					"project-id": "b8639dea",
					"workshop": "canonical-pi2",
					"sdk": "pi",
					"plug": "pin-13",
					"interface": "bool-file",
					"label": "Pin 13",
					"connections": [
						{"project-id":"b8639dea","workshop": "keyboard-lights", "sdk":"lights", "slot": "capslock-led"}
					]
				}
			],
			"slots": [
				{
					"project-id":"b8639dea",
					"workshop": "keyboard-lights",
					"sdk": "lights",
					"slot": "capslock-led",
					"interface": "bool-file",
					"label": "Capslock indicator LED",
					"connections": [
						{"project-id":"b8639dea","workshop": "canonical-pi2", "sdk":"pi", "plug": "pin-13"}
					]
				}
			]
		}
	}`
	conns, err := cs.cli.Connections(&client.ConnectionOptions{ProjectId: "b8639dea"})
	c.Assert(err, check.IsNil)
	c.Check(cs.req.URL.Path, check.Equals, "/v1/connections")
	c.Check(conns, check.DeepEquals, client.Connections{
		Established: []client.Connection{
			{
				Plug:      client.PlugRef{ProjectId: "b8639dea", Workshop: "canonical-pi2", Sdk: "pi", Name: "pin-13"},
				Slot:      client.SlotRef{ProjectId: "b8639dea", Workshop: "keyboard-lights", Sdk: "lights", Name: "capslock-led"},
				Interface: "bool-file",
			},
		},
		Plugs: []client.Plug{
			{
				ProjectId: "b8639dea",
				Workshop:  "canonical-pi2",
				Sdk:       "pi",
				Name:      "pin-13",
				Interface: "bool-file",
				Label:     "Pin 13",
				Connections: []client.SlotRef{
					{
						ProjectId: "b8639dea",
						Workshop:  "keyboard-lights",
						Sdk:       "lights",
						Name:      "capslock-led",
					},
				},
			},
		},
		Slots: []client.Slot{
			{
				ProjectId: "b8639dea",
				Workshop:  "keyboard-lights",
				Sdk:       "lights",
				Name:      "capslock-led",
				Interface: "bool-file",
				Label:     "Capslock indicator LED",
				Connections: []client.PlugRef{
					{
						ProjectId: "b8639dea",
						Workshop:  "canonical-pi2",
						Sdk:       "pi",
						Name:      "pin-13",
					},
				},
			},
		},
	})
}

func (cs *clientSuite) TestClientConnectionsAll(c *check.C) {
	cs.rsp = `{
		"type": "sync",
		"result": {
			"established": [
				{
					"slot": {"project-id":"b8639dea", "workshop": "keyboard-lights", "sdk": "lights",  "slot": "capslock-led"},
					"plug": {"project-id":"b8639dea", "workshop": "canonical-pi2", "sdk": "pi", "plug": "pin-13"},
					"interface": "bool-file"
                }
			],
			"undesired": [
				{
					"slot": {"project-id":"b8639dea", "workshop": "keyboard-lights", "sdk": "lights",  "slot": "numlock-led"},
					"plug": {"project-id":"b8639dea", "workshop": "canonical-pi2", "sdk": "pi", "plug": "pin-14"},
					"interface": "bool-file",
					"manual": true
                }
			],
			"plugs": [
				{
					"project-id": "b8639dea",
					"workshop": "canonical-pi2",
					"sdk": "pi",
					"plug": "pin-13",
					"interface": "bool-file",
					"label": "Pin 13",
					"connections": [
						{"project-id":"b8639dea","workshop": "keyboard-lights", "sdk":"lights", "slot": "capslock-led"}
					]
				},
				{
					"project-id": "b8639dea",
					"workshop": "canonical-pi2",
					"sdk": "pi",
					"plug": "pin-14",
					"interface": "bool-file",
					"label": "Pin 14"
				}
			],
			"slots": [
				{
					"project-id":"b8639dea",
					"workshop": "keyboard-lights",
					"sdk": "lights",
					"slot": "capslock-led",
					"interface": "bool-file",
					"label": "Capslock indicator LED",
					"connections": [
						{"project-id":"b8639dea","workshop": "canonical-pi2", "sdk":"pi", "plug": "pin-13"}
					]
				},
				{
					"project-id":"b8639dea",
					"workshop": "keyboard-lights",
					"sdk": "lights",
					"slot": "numlock-led",
					"interface": "bool-file",
					"label": "Numlock LED"
				}
			]
		}
	}`
	conns, err := cs.cli.Connections(&client.ConnectionOptions{ProjectId: "b8639dea", All: true})
	c.Assert(err, check.IsNil)
	c.Check(cs.req.URL.Path, check.Equals, "/v1/connections")
	c.Check(cs.req.URL.RawQuery, check.Equals, "project-id=b8639dea&select=all")
	c.Check(conns, check.DeepEquals, client.Connections{
		Established: []client.Connection{
			{
				Plug:      client.PlugRef{ProjectId: "b8639dea", Workshop: "canonical-pi2", Sdk: "pi", Name: "pin-13"},
				Slot:      client.SlotRef{ProjectId: "b8639dea", Workshop: "keyboard-lights", Sdk: "lights", Name: "capslock-led"},
				Interface: "bool-file",
			},
		},
		Undesired: []client.Connection{
			{
				Plug:      client.PlugRef{ProjectId: "b8639dea", Workshop: "canonical-pi2", Sdk: "pi", Name: "pin-14"},
				Slot:      client.SlotRef{ProjectId: "b8639dea", Workshop: "keyboard-lights", Sdk: "lights", Name: "numlock-led"},
				Interface: "bool-file",
				Manual:    true,
			},
		},
		Plugs: []client.Plug{
			{
				ProjectId: "b8639dea",
				Workshop:  "canonical-pi2",
				Sdk:       "pi",
				Name:      "pin-13",
				Interface: "bool-file",
				Label:     "Pin 13",
				Connections: []client.SlotRef{
					{
						ProjectId: "b8639dea",
						Workshop:  "keyboard-lights",
						Sdk:       "lights",
						Name:      "capslock-led",
					},
				},
			},
			{
				ProjectId: "b8639dea",
				Workshop:  "canonical-pi2",
				Sdk:       "pi",
				Name:      "pin-14",
				Interface: "bool-file",
				Label:     "Pin 14",
			},
		},
		Slots: []client.Slot{
			{
				ProjectId: "b8639dea",
				Workshop:  "keyboard-lights",
				Sdk:       "lights",
				Name:      "capslock-led",
				Interface: "bool-file",
				Label:     "Capslock indicator LED",
				Connections: []client.PlugRef{
					{
						ProjectId: "b8639dea",
						Workshop:  "canonical-pi2",
						Sdk:       "pi",
						Name:      "pin-13",
					},
				},
			},
			{
				ProjectId: "b8639dea",
				Workshop:  "keyboard-lights",
				Sdk:       "lights",
				Name:      "numlock-led",
				Interface: "bool-file",
				Label:     "Numlock LED",
			},
		},
	})
}

func (cs *clientSuite) TestClientConnectionsFilter(c *check.C) {
	cs.rsp = `{
		"type": "sync",
		"result": {
			"established": [],
			"plugs": [],
			"slots": []
		}
	}`

	_, err := cs.cli.Connections(&client.ConnectionOptions{All: true})
	c.Assert(err, check.IsNil)
	c.Check(cs.req.URL.Path, check.Equals, "/v1/connections")
	c.Check(cs.req.URL.RawQuery, check.Equals, "select=all")

	_, err = cs.cli.Connections(&client.ConnectionOptions{Workshop: "foo"})
	c.Assert(err, check.IsNil)
	c.Check(cs.req.URL.Path, check.Equals, "/v1/connections")
	c.Check(cs.req.URL.RawQuery, check.Equals, "workshop=foo")

	_, err = cs.cli.Connections(&client.ConnectionOptions{Interface: "test"})
	c.Assert(err, check.IsNil)
	c.Check(cs.req.URL.Path, check.Equals, "/v1/connections")
	c.Check(cs.req.URL.RawQuery, check.Equals, "interface=test")

	_, err = cs.cli.Connections(&client.ConnectionOptions{ProjectId: "b8639dea", All: true, Workshop: "foo", Interface: "test"})
	c.Assert(err, check.IsNil)
	query := cs.req.URL.Query()
	c.Check(query, check.DeepEquals, url.Values{
		"project-id": []string{"b8639dea"},
		"select":     []string{"all"},
		"interface":  []string{"test"},
		"workshop":   []string{"foo"},
	})
}
