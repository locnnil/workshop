package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/client"
)

func (s *apiSuite) runMountTest(c *check.C, wp string, buffers []*bytes.Buffer, expected []*expectedResp) {
	s.vars = map[string]string{"id": s.project.ProjectId, "name": wp}
	projectsCmd := apiCmd("/v1/projects/{id}/workshops/{name}/mounts")
	requests := []*http.Request{}

	for _, i := range buffers {
		req, err := s.createProjectsRequest("POST", "/v1/projects/"+s.project.ProjectId+"/workshops/"+wp+"/mounts", i)
		c.Assert(err, check.IsNil)
		requests = append(requests, req)
	}

	for num, req := range requests {
		// Execute
		rsp := v1PostWorkshopMount(projectsCmd, req, nil).(*resp)

		// Verify
		c.Check(rsp.Type, check.Equals, expected[num].Type)
		c.Check(rsp.Status, check.Equals, expected[num].Status, check.Commentf("case: %v: %v", num, rsp))
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
			if expected[num].ChangeErr != "" {
				c.Assert(change.Err(), check.ErrorMatches, expected[num].ChangeErr)
			} else {
				c.Assert(change.Err(), check.IsNil)
			}
		}
	}
}

func (s *apiSuite) TestWorkshopRemountSuccess(c *check.C) {
	// Setup
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

	s.launchWorkshop(c, "manysdks", manysdks, testsdks)

	requests := []*bytes.Buffer{
		bytes.NewBufferString(fmt.Sprintf(`{"action":"remount","plug":{"sdk":"test-sdk","plug":"data"},"source":%q}`, c.MkDir())),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "remount",
			Summary: `Remount manysdks/test-sdk:data`,
		},
	}

	s.runMountTest(c, "manysdks", requests, expected)
}

func (s *apiSuite) TestWorkshopRemountWorkshopSlot(c *check.C) {
	// Setup
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

	s.launchWorkshop(c, "workshopslot", workshopslot, testsdks)

	actions := []*client.InterfaceAction{
		{
			Action: "disconnect",
			Plugs:  []client.Plug{{ProjectId: s.project.ProjectId, Workshop: "workshopslot", Sdk: "test-sdk", Name: "data"}},
			Slots:  []client.Slot{{ProjectId: s.project.ProjectId, Workshop: "workshopslot", Sdk: "host", Name: "content"}},
		},
		{
			Action: "connect",
			Plugs:  []client.Plug{{ProjectId: s.project.ProjectId, Workshop: "workshopslot", Sdk: "test-sdk", Name: "data"}},
			Slots:  []client.Slot{{ProjectId: s.project.ProjectId, Workshop: "workshopslot", Sdk: "host", Name: "training"}},
		},
	}

	// content the content interface plug to the slot provided in the workshop.
	for _, act := range actions {
		text, err := json.Marshal(act)
		c.Assert(err, check.IsNil)
		buf := bytes.NewBuffer(text)
		cmd := apiCmd("/v1/connections")
		req, err := http.NewRequest("POST", cmd.Path, buf)
		c.Assert(err, check.IsNil)
		rec := httptest.NewRecorder()
		v1PostConnections(cmd, req.WithContext(s.ctx), nil).ServeHTTP(rec, req)
		c.Check(rec.Code, check.Equals, 202)
		var body map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &body)
		c.Check(err, check.IsNil)
		id := body["change"].(string)
		st := s.d.Overlord().State()
		st.Lock()
		chg := st.Change(id)
		st.Unlock()
		c.Assert(chg, check.NotNil)

		<-chg.Ready()

		st.Lock()
		err = chg.Err()
		st.Unlock()
		c.Assert(err, check.IsNil)
	}

	requests := []*bytes.Buffer{
		bytes.NewBufferString(fmt.Sprintf(`{"action":"remount","plug":{"sdk":"test-sdk","plug":"data"},"source":%q}`, c.MkDir())),
	}

	expected := []*expectedResp{
		{
			Type:      ResponseTypeAsync,
			Status:    http.StatusAccepted,
			Kind:      "remount",
			Summary:   `Remount workshopslot/test-sdk:data`,
			ChangeErr: `(?s).*cannot change attribute \"source\" as it was statically specified in the "host" sdk details.*`,
		},
	}

	s.runMountTest(c, "workshopslot", requests, expected)
}

func (s *apiSuite) TestWorkshopRemountBoundPlugSuccess(c *check.C) {
	// Setup
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

	s.launchWorkshop(c, "manysdks", manysdks, testsdks)

	// Setup
	src := c.MkDir()
	requests := []*bytes.Buffer{
		bytes.NewBufferString(fmt.Sprintf(`{"action":"remount","plug":{"sdk":"test-sdk","plug":"data"},"source":%q}`, src)),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "remount",
			Summary: `Remount manysdks/test-sdk:data`,
		},
	}

	s.runMountTest(c, "manysdks", requests, expected)
	repo := s.d.overlord.InterfaceManager().Repository()
	ref, err := repo.Connected(s.project.ProjectId, "manysdks", "test-sdk", "data")
	c.Assert(err, check.IsNil)
	conn, err := repo.Connection(ref[0])
	c.Assert(err, check.IsNil)
	c.Assert(conn.Slot.DynamicAttrs(), check.DeepEquals, map[string]interface{}{"source": src})
}

func (s *apiSuite) TestWorkshopRemountPlugDisconnected(c *check.C) {
	// Setup
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

	s.launchWorkshop(c, "manysdks", manysdks, testsdks)

	_, err := s.d.overlord.InterfaceManager().Repository().DisconnectSdk(s.project.ProjectId, "manysdks", "test-sdk")
	c.Check(err, check.IsNil)

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"action":"remount","plug":{"sdk":"test-sdk","plug":"data"},"source":"/srv/data"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `"manysdks/test-sdk:data" must be connected for remount`,
		},
	}

	s.runMountTest(c, "manysdks", requests, expected)
}

func (s *apiSuite) TestWorkshopRemountInvalidSetup(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

	s.launchWorkshop(c, "manysdks", manysdks, testsdks)

	// Setup
	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"action":"mount","plug":{"sdk":"test-sdk","plug":"data"},"source":"/srv/data"}`),
		bytes.NewBufferString(`{"action":"remount","plug":{"sdk":"test-sdk","plug":"data","source":"/srv/data"}`),
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

	s.runMountTest(c, "manysdks", requests, expected)
}

func (s *apiSuite) TestWorkshopRemountInvalidInterface(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

	s.launchWorkshop(c, "manysdks", manysdks, testsdks)

	// Setup
	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"action":"remount","plug":{"sdk":"test-sdk-2","plug":"gpu"},"source":"/srv/data"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `remount requires a content interface plug (provided plug is of "gpu" interface)`,
		},
	}

	s.runMountTest(c, "manysdks", requests, expected)
}

func (s *apiSuite) TestWorkshopRemountStaticSlotSourceFails(c *check.C) {
	// Setup
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

	s.launchWorkshop(c, "workshopconns", workshopconns, testsdks)

	requests := []*bytes.Buffer{
		bytes.NewBufferString(fmt.Sprintf(`{"action":"remount","plug":{"sdk":"test-sdk","plug":"data"},"source":%q}`, c.MkDir())),
	}

	expected := []*expectedResp{
		{
			Type:      ResponseTypeAsync,
			Status:    http.StatusAccepted,
			Kind:      "remount",
			Summary:   `Remount workshopconns/test-sdk:data`,
			ChangeErr: `(?s).*cannot change attribute \"source\" as it was statically specified in the \"host\" sdk details.*`,
		},
	}

	s.runMountTest(c, "workshopconns", requests, expected)
}
