package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/spf13/cobra"
	"golang.org/x/exp/rand"
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/testutil"
)

type BaseWorkshopSuite struct {
	testutil.BaseTest
	stdin  *bytes.Buffer
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func TestMain(t *testing.T) { check.TestingT(t) }

func (s *BaseWorkshopSuite) SetUpTest(c *check.C) {
	dirs.SetRootDir(c.MkDir())

	path := os.Getenv("PATH")
	s.AddCleanup(func() {
		os.Setenv("PATH", path)
	})

	s.stdin = bytes.NewBuffer(nil)
	s.stdout = bytes.NewBuffer(nil)
	s.stderr = bytes.NewBuffer(nil)

	Stdin = s.stdin
	Stdout = s.stdout
	Stderr = s.stderr
}

func (s *BaseWorkshopSuite) TearDownTest(c *check.C) {
	Stdin = os.Stdin
	Stdout = os.Stdout
	Stderr = os.Stderr
}

func (s *BaseWorkshopSuite) RedirectClientToTestServer(handler func(http.ResponseWriter, *http.Request)) {
	server := httptest.NewServer(http.HandlerFunc(handler))
	s.BaseTest.AddCleanup(func() { server.Close() })
	ClientConfig.BaseURL = server.URL
	s.BaseTest.AddCleanup(func() { ClientConfig.BaseURL = "" })
}

func (s *BaseWorkshopSuite) ResetStdStreams() {
	s.stdin.Reset()
	s.stdout.Reset()
	s.stderr.Reset()
}

func (s *BaseWorkshopSuite) Stdout() string {
	return s.stdout.String()
}

func (s *BaseWorkshopSuite) Stderr() string {
	return s.stderr.String()
}

func (s *BaseWorkshopSuite) TestWorkshopInfoList(c *check.C) {
	prjId := "42424242"
	prjDir := c.MkDir()

	status := []string{"Ready", "Pending", "Stopped", "Error"}
	expected := make(map[string][]string)

	var wsInfo []*client.WorkshopInfo
	for i := range 20 {
		index := rand.Intn(len(status) - 1)
		info := &client.WorkshopInfo{
			ProjectId: "42424242",
			Name:      "test" + strconv.Itoa(i),
			Status:    status[index],
		}
		wsInfo = append(wsInfo, info)
		switch index {
		case 0:
			expected["Ready"] = append(expected["Ready"], info.Name)
		case 1:
			expected["Pending"] = append(expected["Pending"], info.Name)
		case 2:
			expected["Stopped"] = append(expected["Stopped"], info.Name)
		case 3:
			expected["Error"] = append(expected["Error"], info.Name)
		}
	}

	w := client.Workshops{
		Workshops: wsInfo,
	}

	cmd := &CmdRoot{}

	s.listRedirectHelper(c, w, prjId, prjDir, len(status)*2)

	for _, st := range status {
		result, compDirective := cmd.completeWorkshopName([]string{st})(cmd.Command(prjDir), nil, "")
		c.Check(result, check.DeepEquals, expected[st])
		c.Check(compDirective, check.Equals, cobra.ShellCompDirectiveNoFileComp)
	}
}

func (m *BaseWorkshopSuite) listRedirectHelper(c *check.C, w client.Workshops, prjId, prjDir string, expected int) {
	workshops, err := json.Marshal(w)
	c.Assert(err, check.IsNil)

	n := 0
	m.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		n++
		switch {
		case n%2 != 0 && n <= expected:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, "/v1/projects")
			r := fmt.Sprintf(`{"type": "sync", "result": {"id":"%s","path":"%s"}}`, prjId, prjDir)
			fmt.Fprintln(w, r)
		case n%2 == 0 && n <= expected:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, "/v1/projects/42424242/workshops")
			rsp := fmt.Sprintf(`{"type": "sync", "result": %s}`, workshops)
			fmt.Fprintln(w, rsp)
		}
	})
}

func (m *BaseWorkshopSuite) connectionsRedirectHelper(c *check.C, conns client.Connections, prjId, prjDir string, expected int) {
	connections, err := json.Marshal(conns)
	c.Assert(err, check.IsNil)

	n := 0
	m.RedirectClientToTestServer(func(w http.ResponseWriter, r *http.Request) {
		n++
		switch {
		case n%2 != 0 && n <= expected:
			c.Check(r.Method, check.Equals, "POST")
			c.Assert(r.URL.Path, check.Equals, "/v1/projects")
			r := fmt.Sprintf(`{"type": "sync", "result": {"id":"%s","path":"%s"}}`, prjId, prjDir)
			fmt.Fprintln(w, r)
		case n%2 == 0 && n <= expected:
			c.Check(r.Method, check.Equals, "GET")
			c.Assert(r.URL.Path, check.Equals, "/v1/connections")
			r := fmt.Sprintf(`{"type": "sync", "result": %s}`, connections)
			fmt.Fprintln(w, r)
		default:
			c.Errorf("expected %d calls, now on %d", expected, n)
		}
	})
}

// EncodeResponseBody writes JSON-serialized body to the response writer.
func EncodeResponseBody(c *check.C, w http.ResponseWriter, body interface{}) {
	encoder := json.NewEncoder(w)
	err := encoder.Encode(body)
	c.Assert(err, check.IsNil)
}

// DecodedRequestBody returns the JSON-decoded body of the request.
func DecodedRequestBody(c *check.C, r *http.Request) map[string]interface{} {
	var body map[string]interface{}
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	err := decoder.Decode(&body)
	c.Assert(err, check.IsNil)
	return body
}

func testPlugsSlots(projectId string) ([]client.Plug, []client.Slot) {
	plugs := []client.Plug{
		{
			ProjectId: projectId,
			Workshop:  "workshop",
			Sdk:       "sdk",
			Name:      "desktop",
			Interface: "desktop",
		},
		{
			ProjectId: projectId,
			Workshop:  "workshop",
			Sdk:       "sdk",
			Name:      "ssh-agent",
			Interface: "ssh-agent",
		},
		{
			ProjectId: projectId,
			Workshop:  "workshop",
			Sdk:       "sdk",
			Name:      "mount",
			Interface: "mount",
		},
		{
			ProjectId: projectId,
			Workshop:  "another-workshop",
			Sdk:       "sdk",
			Name:      "desktop",
			Interface: "desktop",
		},
	}

	slots := []client.Slot{
		{
			ProjectId: projectId,
			Workshop:  "workshop",
			Sdk:       "sdk",
			Name:      "desktop",
			Interface: "desktop",
		},
		{
			ProjectId: projectId,
			Workshop:  "workshop",
			Sdk:       "sdk",
			Name:      "ssh-agent",
			Interface: "ssh-agent",
		},
		{
			ProjectId: projectId,
			Workshop:  "workshop",
			Sdk:       "sdk",
			Name:      "mount",
			Interface: "mount",
		},
		{
			ProjectId: projectId,
			Workshop:  "another-workshop",
			Sdk:       "sdk",
			Name:      "desktop",
			Interface: "desktop",
		},
	}
	return plugs, slots
}

func plugSlotToConn(plug client.Plug, slot client.Slot, manual bool) client.Connection {
	return client.Connection{
		Plug: client.PlugRef{
			ProjectId: plug.ProjectId,
			Workshop:  plug.Workshop,
			Sdk:       plug.Sdk,
			Name:      plug.Name,
		},
		Slot: client.SlotRef{
			ProjectId: slot.ProjectId,
			Workshop:  slot.Workshop,
			Sdk:       slot.Sdk,
			Name:      slot.Name,
		},
		Interface: plug.Interface,
		Manual:    manual,
	}
}

func mockSingleWorkshopSpecifyStatus(status string) string {
	return fmt.Sprintf(`{"type":"sync","status-code":200,"status":"OK","result":{
      "workshops":[{
          "name":"ws",
          "base":"ubuntu@22.04",
          "project-id":"42424242",
          "status":%q,
          "notes":["missing-project"
          ]}
      ]}
  }`, status,
	)
}
