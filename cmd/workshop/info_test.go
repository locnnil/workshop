package main

import (
	"fmt"
	"net/http"
	"os/user"

	"gopkg.in/check.v1"
)

type workshopInfo struct {
	BaseWorkshopSuite
	prjDir string
	prjId  string
}

var _ = check.Suite(&workshopInfo{})

func (m *workshopInfo) SetUpTest(c *check.C) {
	m.prjDir = c.MkDir()
	m.prjId = "42424242"
	m.BaseWorkshopSuite.SetUpTest(c)
}

var mockWorkshopWithSdks = `{"type":"sync","status-code":200,"status":"OK","result":{
    "name":"ws",
    "base":"ubuntu@22.04",
    "project-id":"42424242",
    "status":"Error",
    "sdks":[{
      "name":"go",
      "version":"1.8.0",
      "channel":"latest/edge",
      "revision":"1",
      "build-time":"2017-02-19T17:23:05.592623Z",
      "install-time":"2017-03-22T09:01:00.0Z"
    },{  
      "name":"sketch",
      "channel":"",
      "revision":"x1",
      "install-time":"2017-03-22T09:01:00.0Z"
    }],
    "notes":["missing-project"],
    "path":"/home/project/.workshop/ws.yaml"
},
"warning-timestamp":"2017-03-22T10:01:00.0Z",
"warning-count":1}`

func (m *workshopInfo) TestWorkshopInfo(c *check.C) {
	cmd := &CmdInfo{root: &CmdRoot{}}
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
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops", m.prjId))
			w.WriteHeader(200)
			fmt.Fprintln(w, mockSingleWorkshopSpecifyStatus("Error"))
		case 3:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops/%s", m.prjId, workshop))
			w.WriteHeader(200)
			fmt.Fprintln(w, mockWorkshopWithSdks)
		default:
			c.Errorf("expected 3 calls, now on %d", n)
		}
	})

	err := cmd.Run(cmd.Command(), nil)
	c.Assert(err, check.IsNil)
	c.Assert(m.stdout.String(), check.Matches, fmt.Sprintf(`name:     ws
base:     ubuntu@22.04
project:  %s
status:   error
notes:    missing-project
sdks:
  go:
    tracking:   latest/edge
    installed:  1.8.0  2017-02-19  \(1\)
  sketch:
    tracking:   ~/.local/share/workshop/project/42424242/sdk/sketch/ws
    installed:  2017-03-22  \(x1\)
`, m.prjDir))
	c.Check(n, check.Equals, 3)
}

var mockWorkshopWithHealth = `{"type":"sync","status-code":200,"status":"OK","result":{
    "name":"ws",
    "base":"ubuntu@22.04",
    "project-id":"42424242",
    "status":"Pending",
    "notes":["workshop-note"],
    "sdks":[{
        "name":"go",
        "version":"1.8.0",
        "channel":"latest/edge",
        "revision":"1",
        "build-time":"2017-02-19T17:23:05.592623Z",
        "install-time":"2017-03-22T09:01:00.0Z",
        "health-check":{"message":"Waiting for all required modules to be installed","code":"try-later"}
    }]
}}`

func (m *workshopInfo) TestWorkshopInfoWithSdkHealthReport(c *check.C) {
	cmd := &CmdInfo{root: &CmdRoot{}}
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
sdks:
  go:
    tracking:   latest/edge
    installed:  1.8.0  2017-02-19  \(1\)
    message:    Waiting for all required modules to be installed
`, m.prjDir))
	c.Check(n, check.Equals, 2)
}

var mockWorkshopWithMounts = `{"type":"sync","status-code":200,"status":"OK","result":{
    "name":"ws",
    "base":"ubuntu@22.04",
    "project-id":"42424242",
    "status":"Ready",
    "sdks":[{
        "name":"go",
        "version":"1.8.0",
        "channel":"latest/edge",
        "revision":"1",
        "build-time":"2017-02-19T17:23:05.592623Z",
        "install-time":"2017-03-22T09:01:00.0Z",
        "mounts":[{
            "host-source":"/home/user/src",
            "workshop-target":"/home/workshop/target", 
            "plug":{
                "project-id":"42ws42ws",
                "workshop":"ws",
                "sdk":"go",
                "plug":"plug-name"
            }
        },{
            "host-source":"%s/.local/share/workshop/project/17942561/mount/ws_go_mod-cache.sdk",
            "workshop-target":"/home/workshop/target", 
            "plug":{
                "project-id":"42ws42ws",
                "workshop":"ws",
                "sdk":"go",
                "plug":"plug-default"
            }
        }]
    }]
}}`

func (m *workshopInfo) TestWorkshopInfoWithSdkMounts(c *check.C) {
	cmd := &CmdInfo{root: &CmdRoot{}}
	workshop := "ws"
	n := 0
	user, err := user.Current()
	c.Assert(err, check.IsNil)
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
			fmt.Fprintln(w, fmt.Sprintf(mockWorkshopWithMounts, user.HomeDir))
		default:
			c.Errorf("expected 2 calls, now on %d", n)
		}
	})

	err = cmd.Run(cmd.Command(), []string{workshop})
	c.Assert(err, check.IsNil)
	c.Assert(m.stdout.String(), check.Matches, fmt.Sprintf(`name:     ws
base:     ubuntu@22.04
project:  %s
status:   ready
notes:    -
sdks:
  go:
    tracking:   latest/edge
    installed:  1.8.0  2017-02-19  \(1\)
    mounts:
      plug-default:
        host-source:      .../17942561/mount/ws_go_mod-cache.sdk
        workshop-target:  /home/workshop/target
      plug-name:
        host-source:      /home/user/src
        workshop-target:  /home/workshop/target
`, m.prjDir))
	c.Check(n, check.Equals, 2)
}

var mockWorkshopWithTunnels = `{"type":"sync","status-code":200,"status":"OK","result":{
    "name":"ws",
    "base":"ubuntu@22.04",
    "project-id":"42424242",
    "status":"Ready",
    "sdks":[{
        "name":"go",
        "version":"1.8.0",
        "channel":"latest/edge",
        "revision":"1",
        "build-time":"2017-02-19T17:23:05.592623Z",
        "install-time":"2017-03-22T09:01:00.0Z",
        "tunnels":{
            "plugs":[{
                "plug":{
                    "project-id":"42ws42ws",
                    "workshop":"ws",
                    "sdk":"go",
                    "plug":"snap-cache"
                },
                "slot":{
                    "project-id":"42ws42ws",
                    "workshop":"ws",
                    "sdk":"system",
                    "slot":"snap-cache"
                },
                "from":{
                    "protocol":"tcp",
                    "host":"0.0.0.0",
                    "port":12345
                },
                "to":{
                    "protocol":"unix",
                    "path":"/run/snap-proxy.socket"
                }
            }],
            "slots":[{
                "plug":{
                    "project-id":"42ws42ws",
                    "workshop":"ws",
                    "sdk":"system",
                    "plug":"gopls"
                },
                "slot":{
                    "project-id":"42ws42ws",
                    "workshop":"ws",
                    "sdk":"go",
                    "slot":"gopls"
                },
                "from":{
                    "protocol":"tcp",
                    "host":"127.0.0.1",
                    "port":60915
                },
                "to":{
                    "protocol":"unix",
                    "path":"/run/user/%s/gopls.socket"
                }
            }]
        }
    }]
}}`

func (m *workshopInfo) TestWorkshopInfoWithSdkTunnels(c *check.C) {
	cmd := &CmdInfo{root: &CmdRoot{}}
	workshop := "ws"
	n := 0
	user, err := user.Current()
	c.Assert(err, check.IsNil)
	m.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		n++
		switch n {
		case 1:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, "/v1/projects")
			r := fmt.Sprintf(`{"type": "sync", "result": {"id":"%s","path":"%s"}}`, m.prjId, m.prjDir)
			_, err = fmt.Fprintln(w, r)
			c.Assert(err, check.IsNil)
		case 2:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, fmt.Sprintf("/v1/projects/%s/workshops/%s", m.prjId, workshop))
			w.WriteHeader(200)
			_, err = fmt.Fprintln(w, fmt.Sprintf(mockWorkshopWithTunnels, user.Uid))
			c.Assert(err, check.IsNil)
		default:
			c.Errorf("expected 2 calls, now on %d", n)
		}
	})

	err = cmd.Run(cmd.Command(), []string{workshop})
	c.Assert(err, check.IsNil)
	c.Assert(m.stdout.String(), check.Matches, fmt.Sprintf(`name:     ws
base:     ubuntu@22.04
project:  %s
status:   ready
notes:    -
sdks:
  system:
    tunnels:
      gopls:
        from:  127.0.0.1:60915/tcp
        to:    /run/user/%s/gopls.socket
  go:
    tracking:   latest/edge
    installed:  1.8.0  2017-02-19  \(1\)
    tunnels:
      snap-cache:
        from:  0.0.0.0:12345/tcp
        to:    /run/snap-proxy.socket
`, m.prjDir, user.Uid))
	c.Check(n, check.Equals, 2)
}
