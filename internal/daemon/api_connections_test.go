// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2019-2020 Canonical Ltd
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

package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/interfaces/ifacetest"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/x-go/strutil"
)

// Tests for GET /v1/connections

const (
	consumerYaml = `
name: consumer
base: ubuntu@22.04
plugs:
 plug:
  interface: test
  key: value
  label: label
`
	producerYaml = `
name: producer
base: ubuntu@22.04
slots:
 slot:
  interface: test
  key: value
  label: label
`
)

func (s *apiSuite) mockInstalledSDK(c *check.C, yaml string, workshop string) {
	info := sdk.MockInfo(c, yaml, s.project.ProjectId, workshop)
	c.Assert(s.d.overlord.InterfaceManager().Repository().AddSdk(info), check.IsNil)
	err := os.WriteFile(filepath.Join(s.project.Path, fmt.Sprintf(`.workshop.%s.yaml`, workshop)), []byte(fmt.Sprintf(`name: %s
base: ubuntu@20.04
sdks:
  %s:
    channel: latest/stable
`, workshop, info.Name)), 0644)
	c.Assert(err, check.IsNil)
	c.Assert(s.b.LaunchWorkshop(s.ctx, workshop, "ubuntu@20.04"), check.IsNil)
}

func (s *apiSuite) testConnectionsConnected(c *check.C, d *Daemon, query string, connsState map[string]interface{}, repoConnected []string, expected map[string]interface{}) {
	repo := d.Overlord().InterfaceManager().Repository()
	for crefStr, cstate := range connsState {
		// if repoConnected is defined, then given connection must be on
		// list, otherwise it's not going to be connected in the repository
		// to simulate missing plugs/slots.
		if repoConnected != nil && !strutil.ListContains(repoConnected, crefStr) {
			continue
		}
		cref, err := interfaces.ParseConnRef(crefStr)
		c.Assert(err, check.IsNil)
		conn := cstate.(map[string]interface{})
		if undesiredRaw, ok := conn["undesired"]; ok {
			undesired, ok := undesiredRaw.(bool)
			c.Assert(ok, check.Equals, true, check.Commentf("unexpected value for key 'undesired': %v", cstate))
			if undesired {
				// do not add connections that are undesired
				continue
			}
		}
		staticPlugAttrs, _ := conn["plug-static"].(map[string]interface{})
		dynamicPlugAttrs, _ := conn["plug-dynamic"].(map[string]interface{})
		staticSlotAttrs, _ := conn["slot-static"].(map[string]interface{})
		dynamicSlotAttrs, _ := conn["slot-dynamic"].(map[string]interface{})
		_, err = repo.Connect(cref, staticPlugAttrs, dynamicPlugAttrs, staticSlotAttrs, dynamicSlotAttrs, nil)
		c.Assert(err, check.IsNil)
	}

	st := d.Overlord().State()
	st.Lock()
	st.Set("conns", connsState)
	st.Unlock()

	s.testConnections(c, query, expected)
}

func (s *apiSuite) testConnections(c *check.C, query string, expected map[string]interface{}) {
	cmd := apiCmd("/v1/connections")
	req, err := s.createProjectsRequest("GET", query, nil)
	c.Assert(err, check.IsNil)
	rec := httptest.NewRecorder()
	v1GetConnections(cmd, req, nil).ServeHTTP(rec, req)
	c.Check(rec.Code, check.Equals, 200)
	var body map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &body)
	c.Check(err, check.IsNil)
	c.Check(body, check.DeepEquals, expected)
}

func (s *apiSuite) TestConnectionsUnhappy(c *check.C) {
	s.daemon(c)
	cmd := apiCmd("/v1/connections")
	req, err := s.createProjectsRequest("GET", "/v2/connections?project-id=b8639dea&select=not-found", nil)
	c.Assert(err, check.IsNil)
	rec := httptest.NewRecorder()
	v1GetConnections(cmd, req, nil).ServeHTTP(rec, req)
	c.Check(rec.Code, check.Equals, 400)
	var body map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &body)
	c.Check(err, check.IsNil)
	c.Check(body, check.DeepEquals, map[string]interface{}{
		"result": map[string]interface{}{
			"message": "unsupported select qualifier"},
		"status":      "Bad Request",
		"status-code": 400.0,
		"type":        "error",
	})
}

func (s *apiSuite) TestConnectionsEmpty(c *check.C) {
	s.daemon(c)
	s.testConnections(c, "/v2/connections?project-id=b8639dea", map[string]interface{}{
		"result": map[string]interface{}{
			"established": []interface{}{},
			"plugs":       []interface{}{},
			"slots":       []interface{}{},
		},
		"status":      "OK",
		"status-code": 200.0,
		"type":        "sync",
	})
	s.testConnections(c, "/v2/connections?project-id=b8639dea&select=all", map[string]interface{}{
		"result": map[string]interface{}{
			"established": []interface{}{},
			"plugs":       []interface{}{},
			"slots":       []interface{}{},
		},
		"status":      "OK",
		"status-code": 200.0,
		"type":        "sync",
	})
}

func (s *apiSuite) TestConnectionsNotFound(c *check.C) {
	s.daemon(c)
	cmd := apiCmd("/v1/connections")
	req, err := s.createProjectsRequest("GET", "/v2/connections?project-id=b8639dea&workshop=not-found", nil)
	c.Assert(err, check.IsNil)
	rec := httptest.NewRecorder()
	v1GetConnections(cmd, req, nil).ServeHTTP(rec, req)
	c.Check(rec.Code, check.Equals, 404)
	var body map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &body)
	c.Check(err, check.IsNil)
	c.Check(body, check.DeepEquals, map[string]interface{}{
		"result": map[string]interface{}{
			"message": `cannot access workshop: workshop not found`,
		},
		"status":      "Not Found",
		"status-code": 404.0,
		"type":        "error",
	})
}

func (s *apiSuite) TestConnectionsUnconnected(c *check.C) {
	restore := builtin.MockInterface(&ifacetest.TestInterface{InterfaceName: "test"})
	defer restore()

	s.daemon(c)

	s.mockInstalledSDK(c, consumerYaml, "ws-consumer")
	s.mockInstalledSDK(c, producerYaml, "ws-producer")

	s.testConnections(c, "/v2/connections?project-id=b8639dea&select=all", map[string]interface{}{
		"result": map[string]interface{}{
			"established": []interface{}{},
			"plugs": []interface{}{
				map[string]interface{}{
					"project-id": s.project.ProjectId,
					"workshop":   "ws-consumer",
					"sdk":        "consumer",
					"plug":       "plug",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
				},
			},
			"slots": []interface{}{
				map[string]interface{}{
					"project-id": s.project.ProjectId,
					"workshop":   "ws-producer",
					"sdk":        "producer",
					"slot":       "slot",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
				},
			},
		},
		"status":      "OK",
		"status-code": 200.0,
		"type":        "sync",
	})
}

func (s *apiSuite) TestConnectionsByWorkshopName(c *check.C) {
	restore := builtin.MockInterface(&ifacetest.TestInterface{InterfaceName: "test"})
	defer restore()

	d := s.daemon(c)

	s.mockInstalledSDK(c, consumerYaml, "consumer-ws")
	s.mockInstalledSDK(c, producerYaml, "producer-ws")

	s.testConnections(c, "/v2/connections?project-id=b8639dea&select=all&workshop=producer-ws", map[string]interface{}{
		"result": map[string]interface{}{
			"established": []interface{}{},
			"slots": []interface{}{
				map[string]interface{}{
					"project-id": s.project.ProjectId,
					"workshop":   "producer-ws",
					"sdk":        "producer",
					"slot":       "slot",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
				},
			},
			"plugs": []interface{}{},
		},
		"status":      "OK",
		"status-code": 200.0,
		"type":        "sync",
	})

	s.testConnections(c, "/v2/connections?project-id=b8639dea&select=all&workshop=consumer-ws", map[string]interface{}{
		"result": map[string]interface{}{
			"established": []interface{}{},
			"plugs": []interface{}{
				map[string]interface{}{
					"project-id": s.project.ProjectId,
					"workshop":   "consumer-ws",
					"sdk":        "consumer",
					"plug":       "plug",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
				},
			},
			"slots": []interface{}{},
		},
		"status":      "OK",
		"status-code": 200.0,
		"type":        "sync",
	})

	s.testConnectionsConnected(c, d, "/v2/connections?project-id=b8639dea&workshop=producer-ws", map[string]interface{}{
		"b8639dea:consumer-ws:consumer:plug b8639dea:producer-ws:producer:slot": map[string]interface{}{
			"interface": "test",
		},
	}, nil, map[string]interface{}{
		"result": map[string]interface{}{
			"plugs": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "consumer-ws",
					"sdk":        "consumer",
					"plug":       "plug",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
					"connections": []interface{}{
						map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
					},
				},
			},
			"slots": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "producer-ws",
					"sdk":        "producer",
					"slot":       "slot",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
					"connections": []interface{}{
						map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws", "sdk": "consumer", "plug": "plug"},
					},
				},
			},
			"established": []interface{}{
				map[string]interface{}{
					"plug":      map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws", "sdk": "consumer", "plug": "plug"},
					"slot":      map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
					"manual":    true,
					"interface": "test",
				},
			},
		},
		"status":      "OK",
		"status-code": 200.0,
		"type":        "sync",
	})
}

func (s *apiSuite) TestConnectionsMissingPlugSlotFilteredOut(c *check.C) {
	restore := builtin.MockInterface(&ifacetest.TestInterface{InterfaceName: "test"})
	defer restore()

	d := s.daemon(c)

	s.mockInstalledSDK(c, consumerYaml, "consumer-ws")
	s.mockInstalledSDK(c, producerYaml, "producer-ws")

	for _, missingPlugOrSlot := range []string{"b8639dea:consumer-ws:consumer:plug2 b8639dea:producer-ws:producer:slot", "b8639dea:consumer-ws:consumer:plug b8639dea:producer-ws:producer:slot2"} {
		s.testConnectionsConnected(c, d, "/v2/connections?project-id=b8639dea&workshop=producer-ws", map[string]interface{}{
			"b8639dea:consumer-ws:consumer:plug b8639dea:producer-ws:producer:slot": map[string]interface{}{
				"interface": "test",
			},
			missingPlugOrSlot: map[string]interface{}{
				"interface": "test",
			},
		},
			[]string{"b8639dea:consumer-ws:consumer:plug b8639dea:producer-ws:producer:slot"},
			map[string]interface{}{
				"result": map[string]interface{}{
					"plugs": []interface{}{
						map[string]interface{}{
							"project-id": "b8639dea",
							"workshop":   "consumer-ws",
							"sdk":        "consumer",
							"plug":       "plug",
							"interface":  "test",
							"attrs":      map[string]interface{}{"key": "value"},
							"label":      "label",
							"connections": []interface{}{
								map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
							},
						},
					},
					"slots": []interface{}{
						map[string]interface{}{
							"project-id": "b8639dea",
							"workshop":   "producer-ws",
							"sdk":        "producer",
							"slot":       "slot",
							"interface":  "test",
							"attrs":      map[string]interface{}{"key": "value"},
							"label":      "label",
							"connections": []interface{}{
								map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws", "sdk": "consumer", "plug": "plug"},
							},
						},
					},
					"established": []interface{}{
						map[string]interface{}{
							"plug":      map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws", "sdk": "consumer", "plug": "plug"},
							"slot":      map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
							"manual":    true,
							"interface": "test",
						},
					},
				},
				"status":      "OK",
				"status-code": 200.0,
				"type":        "sync",
			})
	}
}

func (s *apiSuite) TestConnectionsByIfaceName(c *check.C) {
	restore := builtin.MockInterface(&ifacetest.TestInterface{InterfaceName: "test"})
	defer restore()
	restore = builtin.MockInterface(&ifacetest.TestInterface{InterfaceName: "different"})
	defer restore()

	d := s.daemon(c)

	s.mockInstalledSDK(c, consumerYaml, "consumer-ws")
	s.mockInstalledSDK(c, producerYaml, "producer-ws")
	var differentProducerYaml = `
name: different-producer
base: ubuntu@22.04
slots:
 slot:
  interface: different
  key: value
  label: label
`
	var differentConsumerYaml = `
name: different-consumer
base: ubuntu@22.04
plugs:
 plug:
  interface: different
  key: value
  label: label
`
	s.mockInstalledSDK(c, differentConsumerYaml, "consumer-diff-ws")
	s.mockInstalledSDK(c, differentProducerYaml, "producer-diff-ws")

	s.testConnections(c, "/v2/connections?project-id=b8639dea&select=all&interface=test", map[string]interface{}{
		"result": map[string]interface{}{
			"established": []interface{}{},
			"plugs": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "consumer-ws",
					"sdk":        "consumer",
					"plug":       "plug",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
				},
			},
			"slots": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "producer-ws",
					"sdk":        "producer",
					"slot":       "slot",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
				},
			},
		},
		"status":      "OK",
		"status-code": 200.0,
		"type":        "sync",
	})
	s.testConnections(c, "/v2/connections?project-id=b8639dea&select=all&interface=different", map[string]interface{}{
		"result": map[string]interface{}{
			"established": []interface{}{},
			"plugs": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "consumer-diff-ws",
					"sdk":        "different-consumer",
					"plug":       "plug",
					"interface":  "different",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
				},
			},
			"slots": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "producer-diff-ws",
					"sdk":        "different-producer",
					"slot":       "slot",
					"interface":  "different",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
				},
			},
		},
		"status":      "OK",
		"status-code": 200.0,
		"type":        "sync",
	})

	// modifies state internally
	s.testConnectionsConnected(c, d, "/v2/connections?project-id=b8639dea&interfaces=test", map[string]interface{}{
		"b8639dea:consumer-ws:consumer:plug b8639dea:producer-ws:producer:slot": map[string]interface{}{
			"interface": "test",
		},
	}, nil, map[string]interface{}{
		"result": map[string]interface{}{
			"plugs": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "consumer-ws",
					"sdk":        "consumer",
					"plug":       "plug",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
					"connections": []interface{}{
						map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
					},
				},
			},
			"slots": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "producer-ws",
					"sdk":        "producer",
					"slot":       "slot",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
					"connections": []interface{}{
						map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws", "sdk": "consumer", "plug": "plug"},
					},
				},
			},
			"established": []interface{}{
				map[string]interface{}{
					"plug":      map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws", "sdk": "consumer", "plug": "plug"},
					"slot":      map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
					"manual":    true,
					"interface": "test",
				},
			},
		},
		"status":      "OK",
		"status-code": 200.0,
		"type":        "sync",
	})
	// use state modified by previous call
	s.testConnections(c, "/v2/connections?project-id=b8639dea&interface=different", map[string]interface{}{
		"result": map[string]interface{}{
			"established": []interface{}{},
			"slots":       []interface{}{},
			"plugs":       []interface{}{},
		},
		"status":      "OK",
		"status-code": 200.0,
		"type":        "sync",
	})
}

func (s *apiSuite) TestConnectionsDefaultManual(c *check.C) {
	restore := builtin.MockInterface(&ifacetest.TestInterface{InterfaceName: "test"})
	defer restore()

	d := s.daemon(c)

	s.mockInstalledSDK(c, consumerYaml, "consumer-ws")
	s.mockInstalledSDK(c, producerYaml, "producer-ws")

	s.testConnectionsConnected(c, d, "/v2/connections?project-id=b8639dea", map[string]interface{}{
		"b8639dea:consumer-ws:consumer:plug b8639dea:producer-ws:producer:slot": map[string]interface{}{
			"interface": "test",
		},
	}, nil, map[string]interface{}{
		"result": map[string]interface{}{
			"plugs": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "consumer-ws",
					"sdk":        "consumer",
					"plug":       "plug",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
					"connections": []interface{}{
						map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
					},
				},
			},
			"slots": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "producer-ws",
					"sdk":        "producer",
					"slot":       "slot",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
					"connections": []interface{}{
						map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws", "sdk": "consumer", "plug": "plug"},
					},
				},
			},
			"established": []interface{}{
				map[string]interface{}{
					"plug":      map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws", "sdk": "consumer", "plug": "plug"},
					"slot":      map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
					"manual":    true,
					"interface": "test",
				},
			},
		},
		"status":      "OK",
		"status-code": 200.0,
		"type":        "sync",
	})
}

func (s *apiSuite) TestConnectionsDefaultAuto(c *check.C) {
	restore := builtin.MockInterface(&ifacetest.TestInterface{InterfaceName: "test"})
	defer restore()

	d := s.daemon(c)

	s.mockInstalledSDK(c, consumerYaml, "consumer-ws")
	s.mockInstalledSDK(c, producerYaml, "producer-ws")

	s.testConnectionsConnected(c, d, "/v2/connections?project-id=b8639dea", map[string]interface{}{
		"b8639dea:consumer-ws:consumer:plug b8639dea:producer-ws:producer:slot": map[string]interface{}{
			"interface": "test",
			"auto":      true,
			"plug-static": map[string]interface{}{
				"key": "value",
			},
			"plug-dynamic": map[string]interface{}{
				"foo-plug-dynamic": "bar-dynamic",
			},
			"slot-static": map[string]interface{}{
				"key": "value",
			},
			"slot-dynamic": map[string]interface{}{
				"foo-slot-dynamic": "bar-dynamic",
			},
		},
	}, nil, map[string]interface{}{
		"result": map[string]interface{}{
			"plugs": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "consumer-ws",
					"sdk":        "consumer",
					"plug":       "plug",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
					"connections": []interface{}{
						map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
					},
				},
			},
			"slots": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "producer-ws",
					"sdk":        "producer",
					"slot":       "slot",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
					"connections": []interface{}{
						map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws", "sdk": "consumer", "plug": "plug"},
					},
				},
			},
			"established": []interface{}{
				map[string]interface{}{
					"plug":      map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws", "sdk": "consumer", "plug": "plug"},
					"slot":      map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
					"interface": "test",
					"plug-attrs": map[string]interface{}{
						"key":              "value",
						"foo-plug-dynamic": "bar-dynamic",
					},
					"slot-attrs": map[string]interface{}{
						"key":              "value",
						"foo-slot-dynamic": "bar-dynamic",
					},
				},
			},
		},
		"status":      "OK",
		"status-code": 200.0,
		"type":        "sync",
	})
}

func (s *apiSuite) TestConnectionsAll(c *check.C) {
	restore := builtin.MockInterface(&ifacetest.TestInterface{InterfaceName: "test"})
	defer restore()

	d := s.daemon(c)

	s.mockInstalledSDK(c, consumerYaml, "consumer-ws")
	s.mockInstalledSDK(c, producerYaml, "producer-ws")

	s.testConnectionsConnected(c, d, "/v2/connections?project-id=b8639dea&select=all", map[string]interface{}{
		"b8639dea:consumer-ws:consumer:plug b8639dea:producer-ws:producer:slot": map[string]interface{}{
			"interface": "test",
			"auto":      true,
			"undesired": true,
		},
	}, nil, map[string]interface{}{
		"result": map[string]interface{}{
			"established": []interface{}{},
			"plugs": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "consumer-ws",
					"sdk":        "consumer",
					"plug":       "plug",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
				},
			},
			"slots": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "producer-ws",
					"sdk":        "producer",
					"slot":       "slot",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
				},
			},
			"undesired": []interface{}{
				map[string]interface{}{
					"plug":      map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws", "sdk": "consumer", "plug": "plug"},
					"slot":      map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
					"manual":    true,
					"interface": "test",
				},
			},
		},
		"status":      "OK",
		"status-code": 200.0,
		"type":        "sync",
	})
}

func (s *apiSuite) TestConnectionsOnlyConnected(c *check.C) {
	restore := builtin.MockInterface(&ifacetest.TestInterface{InterfaceName: "test"})
	defer restore()

	d := s.daemon(c)

	s.mockInstalledSDK(c, consumerYaml, "consumer-ws")
	s.mockInstalledSDK(c, producerYaml, "producer-ws")

	s.testConnectionsConnected(c, d, "/v2/connections?project-id=b8639dea", map[string]interface{}{
		"b8639dea:consumer-ws:consumer:plug b8639dea:producer-ws:producer:slot": map[string]interface{}{
			"interface": "test",
			"auto":      true,
			"undesired": true,
		},
	}, nil, map[string]interface{}{
		"result": map[string]interface{}{
			"established": []interface{}{},
			"plugs":       []interface{}{},
			"slots":       []interface{}{},
		},
		"status":      "OK",
		"status-code": 200.0,
		"type":        "sync",
	})
}

func (s *apiSuite) TestConnectionsSorted(c *check.C) {
	restore := builtin.MockInterface(&ifacetest.TestInterface{InterfaceName: "test"})
	defer restore()

	d := s.daemon(c)

	var anotherConsumerYaml = `
name: another-consumer-%s
base: ubuntu@22.04
plugs:
 plug:
  interface: test
  key: value
  label: label
`
	var anotherProducerYaml = `
name: another-producer
base: ubuntu@22.04
slots:
 slot:
  interface: test
  key: value
  label: label
`
	s.mockInstalledSDK(c, consumerYaml, "consumer-ws")
	s.mockInstalledSDK(c, fmt.Sprintf(anotherConsumerYaml, "def"), "consumer-ws-def")
	s.mockInstalledSDK(c, fmt.Sprintf(anotherConsumerYaml, "abc"), "consumer-ws-abc")

	s.mockInstalledSDK(c, producerYaml, "producer-ws")
	s.mockInstalledSDK(c, anotherProducerYaml, "another-producer-ws")

	s.testConnectionsConnected(c, d, "/v2/connections?project-id=b8639dea", map[string]interface{}{
		"b8639dea:consumer-ws:consumer:plug b8639dea:producer-ws:producer:slot": map[string]interface{}{
			"interface": "test",
			"auto":      true,
		},
		"b8639dea:consumer-ws-def:another-consumer-def:plug b8639dea:producer-ws:producer:slot": map[string]interface{}{
			"interface": "test",
			"auto":      true,
		},
		"b8639dea:consumer-ws-abc:another-consumer-abc:plug b8639dea:producer-ws:producer:slot": map[string]interface{}{
			"interface": "test",
			"auto":      true,
		},
		"b8639dea:consumer-ws-def:another-consumer-def:plug b8639dea:another-producer-ws:another-producer:slot": map[string]interface{}{
			"interface": "test",
			"auto":      true,
		},
	}, nil, map[string]interface{}{
		"result": map[string]interface{}{
			"plugs": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "consumer-ws",
					"sdk":        "consumer",
					"plug":       "plug",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
					"connections": []interface{}{
						map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
					},
				},
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "consumer-ws-abc",
					"sdk":        "another-consumer-abc",
					"plug":       "plug",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
					"connections": []interface{}{
						map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
					},
				},
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "consumer-ws-def",
					"sdk":        "another-consumer-def",
					"plug":       "plug",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
					"connections": []interface{}{
						map[string]interface{}{"project-id": "b8639dea", "workshop": "another-producer-ws", "sdk": "another-producer", "slot": "slot"},
						map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
					},
				},
			},
			"slots": []interface{}{
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "another-producer-ws",
					"sdk":        "another-producer",
					"slot":       "slot",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
					"connections": []interface{}{
						map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws-def", "sdk": "another-consumer-def", "plug": "plug"},
					},
				},
				map[string]interface{}{
					"project-id": "b8639dea",
					"workshop":   "producer-ws",
					"sdk":        "producer",
					"slot":       "slot",
					"interface":  "test",
					"attrs":      map[string]interface{}{"key": "value"},
					"label":      "label",
					"connections": []interface{}{
						map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws", "sdk": "consumer", "plug": "plug"},
						map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws-abc", "sdk": "another-consumer-abc", "plug": "plug"},
						map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws-def", "sdk": "another-consumer-def", "plug": "plug"},
					},
				},
			},
			"established": []interface{}{
				map[string]interface{}{
					"plug":      map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws", "sdk": "consumer", "plug": "plug"},
					"slot":      map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
					"interface": "test",
				},
				map[string]interface{}{
					"plug":      map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws-abc", "sdk": "another-consumer-abc", "plug": "plug"},
					"slot":      map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
					"interface": "test",
				},
				map[string]interface{}{
					"plug":      map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws-def", "sdk": "another-consumer-def", "plug": "plug"},
					"slot":      map[string]interface{}{"project-id": "b8639dea", "workshop": "another-producer-ws", "sdk": "another-producer", "slot": "slot"},
					"interface": "test",
				},
				map[string]interface{}{
					"plug":      map[string]interface{}{"project-id": "b8639dea", "workshop": "consumer-ws-def", "sdk": "another-consumer-def", "plug": "plug"},
					"slot":      map[string]interface{}{"project-id": "b8639dea", "workshop": "producer-ws", "sdk": "producer", "slot": "slot"},
					"interface": "test",
				},
			},
		},
		"status":      "OK",
		"status-code": 200.0,
		"type":        "sync",
	})
}

// Tests for POST /v1/connections

func (s *apiSuite) testDisconnect(c *check.C, plugWorkshop, plugSdk, plugName, slotWorkshop, slotSdk, slotName string) {
	restore := builtin.MockInterface(&ifacetest.TestInterface{InterfaceName: "test"})
	defer restore()

	d := s.daemon(c)

	s.mockInstalledSDK(c, consumerYaml, "consumer-ws")
	s.mockInstalledSDK(c, producerYaml, "producer-ws")

	repo := d.Overlord().InterfaceManager().Repository()
	connRef := &interfaces.ConnRef{
		PlugRef: interfaces.PlugRef{ProjectId: "b8639dea", Workshop: "consumer-ws", Sdk: "consumer", Name: "plug"},
		SlotRef: interfaces.SlotRef{ProjectId: "b8639dea", Workshop: "producer-ws", Sdk: "producer", Name: "slot"},
	}
	_, err := repo.Connect(connRef, nil, nil, nil, nil, nil)
	c.Assert(err, check.IsNil)

	st := d.Overlord().State()
	st.Lock()
	st.Set("conns", map[string]interface{}{
		"b8639dea:consumer-ws:consumer:plug b8639dea:producer-ws:producer:slot": map[string]interface{}{
			"interface": "test",
		},
	})
	st.Unlock()

	d.Overlord().Loop()
	defer d.Overlord().Stop()

	action := &client.InterfaceAction{
		Action: "disconnect",
		Plugs:  []client.Plug{{ProjectId: "b8639dea", Workshop: plugWorkshop, Sdk: plugSdk, Name: plugName}},
		Slots:  []client.Slot{{ProjectId: "b8639dea", Workshop: slotWorkshop, Sdk: slotSdk, Name: slotName}},
	}
	text, err := json.Marshal(action)
	c.Assert(err, check.IsNil)
	buf := bytes.NewBuffer(text)
	req, err := http.NewRequest("POST", "/v1/connections", buf)
	c.Assert(err, check.IsNil)
	rec := httptest.NewRecorder()
	cmd := apiCmd("/v1/connections")
	v1PostConnections(cmd, req.WithContext(s.ctx), nil).ServeHTTP(rec, req)
	c.Check(rec.Code, check.Equals, 202)
	var body map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &body)
	c.Check(err, check.IsNil)
	id := body["change"].(string)

	st.Lock()
	chg := st.Change(id)
	st.Unlock()
	c.Assert(chg, check.NotNil)

	<-chg.Ready()

	st.Lock()
	err = chg.Err()
	st.Unlock()
	c.Assert(err, check.IsNil)

	ifaces := repo.Interfaces()
	c.Assert(ifaces.Connections, check.HasLen, 0)
}

// func (s *apiSuite) TestDisconnectPlugSuccess(c *check.C) {
// 	s.testDisconnect(c, "consumer-ws", "consumer", "plug", "producer-ws", "producer", "slot")
// }
