package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/client"
)

func (s *sdkSuite) TestInfo(c *check.C) {
	d1 := time.Date(2024, 11, 25, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2024, 11, 20, 0, 0, 0, 0, time.UTC)

	home, _ := os.UserHomeDir()
	nav := filepath.Join(home, "work", "nav2")
	lerobot := filepath.Join(home, "work", "lerobot")

	resp := client.SdkFullInfo{
		Name:        "openvino",
		Summary:     "ROS2 development environment",
		Description: "Longer description\ncan be multiline.",
		Installed: []client.SdkInstalled{
			{
				ProjectPath: nav,
				Workshop:    "ci",
				Channel:     "latest/stable",
				SdkVolume: client.SdkVolume{
					Revision:  "85",
					BuildTime: &d1,
					Size:      109 * 1024 * 1024,
				},
			},
			{
				ProjectPath: lerobot,
				Workshop:    "dev",
				Channel:     "latest/edge",
				SdkVolume: client.SdkVolume{
					Revision:  "82",
					BuildTime: &d2,
					Size:      102 * 1024 * 1024,
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.Assert(r.Method, check.Equals, "GET")
		c.Assert(r.URL.Path, check.Equals, "/v1/sdks/openvino")
		body := map[string]interface{}{
			"type":   "sync",
			"result": resp,
		}
		encoder := json.NewEncoder(w)
		c.Assert(encoder.Encode(body), check.IsNil)
	}))
	defer srv.Close()

	ClientConfig.BaseURL = srv.URL

	cmd := (&CmdRoot{}).Command()
	cmd.SetArgs([]string{"info", "openvino"})
	c.Assert(cmd.Execute(), check.IsNil)

	want := ""
	want += "name: openvino\n"
	want += "summary: ROS2 development environment\n"
	want += "description: |\n"
	want += "  Longer description\n"
	want += "  can be multiline.\n"
	want += "installed:\n"
	want += "  " + contractHome(nav) + ":     ci   latest/stable  2024-11-25  (85)  109MB\n"
	want += "  " + contractHome(lerobot) + ":  dev  latest/edge    2024-11-20  (82)  102MB\n"

	c.Check(s.Stdout(), check.Equals, want)
}
