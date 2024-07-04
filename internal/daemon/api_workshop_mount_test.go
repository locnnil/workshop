package daemon

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
)

var wp = `name: %s
base: ubuntu@20.04
sdks:
  test-sdk:
    channel: latest/stable
`

var workshopBind = `name: %s
base: ubuntu@20.04
sdks:
  test-sdk:
    channel: latest/stable
    plugs:
      test-plug:
        bind: test-sdk:test-plug2
`

func (s *apiSuite) launchWorkshopWithPlug(c *check.C, ctx context.Context, name string, wsyaml string) *workshop.Workshop {
	b := s.d.overlord.WorkshopBackend()
	err := os.WriteFile(filepath.Join(s.project.Path, fmt.Sprintf(`.workshop.%s.yaml`, name)), []byte(fmt.Sprintf(wsyaml, name)), 0644)
	wf, err := s.project.Workshop(name)
	c.Assert(err, check.IsNil)
	err = b.LaunchWorkshop(ctx, wf)
	c.Assert(err, check.IsNil)
	ws, err := b.Workshop(ctx, name)
	c.Assert(err, check.IsNil)

	sdkInfo := &sdk.Info{ProjectId: s.project.ProjectId, Workshop: name, Name: "test-sdk"}
	plug := &sdk.PlugInfo{
		Sdk:       sdkInfo,
		Name:      "test-plug",
		Interface: "content",
	}
	plug2 := &sdk.PlugInfo{
		Sdk:       sdkInfo,
		Name:      "test-plug2",
		Interface: "content",
	}
	slot := &sdk.SlotInfo{
		Sdk:       sdkInfo,
		Name:      "test-slot",
		Interface: "content",
	}

	repo := s.d.overlord.InterfaceManager().Repository()
	c.Assert(repo.AddPlug(plug), check.IsNil)
	c.Assert(repo.AddPlug(plug2), check.IsNil)
	c.Assert(repo.AddSlot(slot), check.IsNil)
	_, err = repo.Connect(interfaces.NewConnRef(plug, slot), nil, nil, nil, nil, nil)
	_, err = repo.Connect(interfaces.NewConnRef(plug2, slot), nil, nil, nil, nil, nil)

	c.Assert(err, check.IsNil)

	return ws
}

func (s *apiSuite) runMountTest(c *check.C, buffers []*bytes.Buffer, expected []*expectedResp) {
	s.vars = map[string]string{"id": s.project.ProjectId, "name": "ws"}
	projectsCmd := apiCmd("/v1/projects/{id}/workshops/{name}/mounts")
	requests := []*http.Request{}

	for _, i := range buffers {
		req, err := s.createProjectsRequest("POST", "/v1/projects/"+s.project.ProjectId+"/workshops/ws/mounts", i)
		c.Assert(err, check.IsNil)
		requests = append(requests, req)
	}

	s.d.overlord.Loop()
	defer s.d.overlord.Stop()

	for num, req := range requests {
		// Execute
		rsp := v1PostWorkshopMount(projectsCmd, req, nil).(*resp)

		// Verify
		c.Check(rsp.Type, check.Equals, expected[num].Type)
		c.Assert(rsp.Status, check.Equals, expected[num].Status, check.Commentf("case: %v", num))
		if rsp.Type == ResponseTypeError {
			c.Assert(rsp.Result.(*errorResult).Message, check.Equals, expected[num].Message)
		}

		if rsp.Type == ResponseTypeAsync {
			st := s.d.state
			st.Lock()
			change := s.d.state.Change(rsp.Change)
			st.Unlock()
			c.Assert(change, check.NotNil)
			c.Assert(change.Kind(), check.Equals, expected[num].Kind)
			c.Assert(change.Summary(), check.Equals, expected[num].Summary)
			<-change.Ready()
			c.Assert(change.Err(), check.IsNil)
		}
	}
}

func (s *apiSuite) TestWorkshopRemountSuccess(c *check.C) {
	s.daemon(c)

	s.launchWorkshopWithPlug(c, s.ctx, "ws", wp)

	// Setup
	requests := []*bytes.Buffer{
		bytes.NewBufferString(fmt.Sprintf(`{"action":"remount","plug":{"sdk":"test-sdk","plug":"test-plug"},"source":%q}`, c.MkDir())),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "remount",
			Summary: `Remount ws/test-sdk:test-plug`,
		},
	}

	s.runMountTest(c, requests, expected)
}

func (s *apiSuite) TestWorkshopRemountBoundPlugSuccess(c *check.C) {
	s.daemon(c)
	s.launchWorkshopWithPlug(c, s.ctx, "ws", workshopBind)

	// Setup
	src := c.MkDir()
	requests := []*bytes.Buffer{
		bytes.NewBufferString(fmt.Sprintf(`{"action":"remount","plug":{"sdk":"test-sdk","plug":"test-plug"},"source":%q}`, src)),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "remount",
			Summary: `Remount ws/test-sdk:test-plug`,
		},
	}

	s.runMountTest(c, requests, expected)
	repo := s.d.overlord.InterfaceManager().Repository()
	ref, err := repo.Connected(s.project.ProjectId, "ws", "test-sdk", "test-plug2")
	c.Assert(err, check.IsNil)
	conn, err := repo.Connection(ref[0])
	c.Assert(err, check.IsNil)
	c.Assert(conn.Plug.DynamicAttrs(), check.DeepEquals, map[string]interface{}{"source": src})
}

func (s *apiSuite) TestWorkshopRemountPlugDisconnected(c *check.C) {
	// Setup
	s.daemon(c)
	s.launchWorkshopWithPlug(c, s.ctx, "ws", wp)
	_, err := s.d.overlord.InterfaceManager().Repository().DisconnectSdk(s.project.ProjectId, "ws", "test-sdk")
	c.Check(err, check.IsNil)

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"action":"remount","plug":{"sdk":"test-sdk","plug":"test-plug"},"source":"/srv/data"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `"ws/test-sdk:test-plug" must be connected for remount`,
		},
	}

	s.runMountTest(c, requests, expected)
}

func (s *apiSuite) TestWorkshopRemountInvalidSetup(c *check.C) {
	s.daemon(c)
	s.launchWorkshop(s.ctx, "ws", c)

	// Setup
	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"action":"mount","plug":{"sdk":"test-sdk","plug":"test-plug"},"source":"/srv/data"}`),
		bytes.NewBufferString(`{"action":"remount","plug":{"sdk":"test-sdk","plug":"test-plug","source":"/srv/data"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `unknown action "mount"`,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `cannot decode data from request body: unexpected EOF`,
		},
	}

	s.runMountTest(c, requests, expected)
}

func (s *apiSuite) TestWorkshopRemountInvalidInterface(c *check.C) {
	s.daemon(c)
	s.launchWorkshop(s.ctx, "ws", c)

	sdkInfo := &sdk.Info{ProjectId: s.project.ProjectId, Workshop: "ws", Name: "test-sdk"}
	plug := &sdk.PlugInfo{
		Sdk:       sdkInfo,
		Name:      "test-plug",
		Interface: "gpu",
	}
	slot := &sdk.SlotInfo{
		Sdk:       sdkInfo,
		Name:      "test-slot",
		Interface: "gpu",
	}

	repo := s.d.overlord.InterfaceManager().Repository()
	c.Assert(repo.AddPlug(plug), check.IsNil)
	c.Assert(repo.AddSlot(slot), check.IsNil)
	_, err := repo.Connect(interfaces.NewConnRef(plug, slot), nil, nil, nil, nil, nil)
	c.Assert(err, check.IsNil)

	// Setup
	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"action":"remount","plug":{"sdk":"test-sdk","plug":"test-plug"},"source":"/srv/data"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `remount requires a content interface plug (provided plug is of "gpu" interface)`,
		},
	}

	soon := 0
	s.runMountTest(c, requests, expected)
	c.Assert(soon, check.Equals, 0)
}
