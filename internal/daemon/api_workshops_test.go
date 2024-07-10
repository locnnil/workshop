package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/overlord/conflict"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
)

var (
	basic = `name: basic
base: ubuntu@22.04
`
	manysdks = `name: manysdks
base: ubuntu@22.04
sdks:
  test-sdk:
    channel: latest/stable
  test-sdk-2:
    channel: latest/stable
`

	somebound = `name: somebound
base: ubuntu@22.04
sdks:
  test-sdk:
    channel: latest/stable
    plugs:
      data:
        bind: test-sdk-2:photos
  test-sdk-2:
    channel: latest/stable
`

	masterunknown = `name: masterunknown
base: ubuntu@22.04
sdks:
  test-sdk:
    channel: latest/stable
    plugs:
      data:
        bind: test-sdk-2:unknown
  test-sdk-2:
    channel: latest/stable
`

	slaveunknown = `name: slaveunknown
base: ubuntu@22.04
sdks:
  test-sdk:
    channel: latest/stable
    plugs:
      unknown:
        bind: test-sdk-2:photos
  test-sdk-2:
    channel: latest/stable
`

	bindincompatible = `name: bindincompatible
base: ubuntu@22.04
sdks:
  test-sdk:
    channel: latest/stable
    plugs:
      data:
        bind: test-sdk-2:gpu
  test-sdk-2:
    channel: latest/stable
`

	testsdk = `
name: test-sdk
base: ubuntu@20.04
title: title
summary: summary
description: SDK
plugs:
  data:
    interface: content
    target: /opt/data
  ssh-agent:
    interface: test
`

	testsdk2 = `
name: test-sdk-2
base: ubuntu@20.04
title: title
summary: summary
description: SDK
plugs:
  photos:
    interface: content
    target: /opt/data2
  gpu:
    interface: gpu
`
)

var testsdks = map[string]string{
	"test-sdk":   testsdk,
	"test-sdk-2": testsdk2,
}

func (s *apiSuite) launchWorkshop(c *check.C, name, yaml string, sdks map[string]string) {
	s.createWFile(c, name, yaml)

	defer s.store.SetActionCallback(storeAction)()
	defer s.mockWorkshopFs(c, sdks)()

	reqbuf := bytes.NewBufferString(fmt.Sprintf(`{"names":["%s"],"action":"launch"}`, name))
	s.vars = map[string]string{"id": s.project.ProjectId}
	req, err := s.createProjectsRequest("POST", "/v1/projects/"+s.project.ProjectId+"/workshops", reqbuf)
	c.Assert(err, check.IsNil)

	rsp := v1PostProjectWorkshop(apiCmd("/v1/projects/{id}/workshops"), req, nil).(*resp)
	st := s.d.state
	st.Lock()
	change := st.Change(rsp.Change)
	st.Unlock()
	<-change.Ready()

	c.Assert(change.Err(), check.IsNil)
}

func (s *apiSuite) TestGetWorkshops(c *check.C) {
	// Setup
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

	s.launchWorkshop(c, "manysdks", manysdks, testsdks)
	s.launchWorkshop(c, "basic", basic, map[string]string{})

	projectsCmd := apiCmd("/v1/projects/{id}/workshops")
	s.vars = map[string]string{"id": s.project.ProjectId}
	req, err := s.createProjectsRequest("GET", "/v1/projects/"+s.project.ProjectId+"/workshops", nil)
	c.Assert(err, check.IsNil)

	// Execute
	rsp := v1GetProjectWorkshops(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	_, err = rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	// for DeepEqual to work correctly
	t1, t2 := s.installTime, s.installTime
	c.Check(rsp.Result, testutil.DeepUnsortedMatches, []*WorkshopInfo{
		{
			Name:      "manysdks",
			Base:      "ubuntu@22.04",
			ProjectId: s.project.ProjectId,
			Status:    "Ready",
			Content: []*SdkInfo{
				{
					Name:        "test-sdk",
					Channel:     "latest/stable",
					Revision:    "0",
					InstallTime: &t1,
				},
				{
					Name:        "test-sdk-2",
					Channel:     "latest/stable",
					Revision:    "0",
					InstallTime: &t2,
				},
			},
			Notes: nil,
		}, {
			Name:      "basic",
			Base:      "ubuntu@22.04",
			ProjectId: s.project.ProjectId,
			Status:    "Ready",
			Notes:     nil,
		},
	})
}

func (s *apiSuite) TestGetWorkshopInfo(c *check.C) {
	// Setup (create a running workshop)
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

	s.launchWorkshop(c, "manysdks", manysdks, testsdks)

	// Get Workshop info
	projectsCmd := apiCmd("/v1/projects/{id}/workshops/{name}")
	s.vars = map[string]string{"id": s.project.ProjectId, "name": "manysdks"}
	req, err := s.createProjectsRequest("GET", "/v1/projects/"+s.project.ProjectId+"/workshops/manysdks", nil)

	rsp := v1GetProjectWorkshop(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	_, err = rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	// for DeepEqual to work correctly
	t1, t2 := s.installTime, s.installTime
	c.Check(rsp.Result, check.DeepEquals, &WorkshopInfo{
		Name:      "manysdks",
		Base:      "ubuntu@22.04",
		ProjectId: s.project.ProjectId,
		Status:    "Ready",
		Notes:     nil,
		Content: []*SdkInfo{
			{
				Name:        "test-sdk",
				Channel:     "latest/stable",
				Revision:    "0",
				InstallTime: &t1,
				Mounts: []*Mount{
					{
						Source: sdk.DefaultContentSource(s.userhome, s.project.ProjectId, "manysdks", "test-sdk", "data"),
						Target: "/opt/data",
						Plug: interfaces.PlugRef{
							ProjectId: s.project.ProjectId,
							Workshop:  "manysdks",
							Sdk:       "test-sdk",
							Name:      "data",
						},
					},
				},
			},
			{
				Name:        "test-sdk-2",
				Channel:     "latest/stable",
				Revision:    "0",
				InstallTime: &t2,
				Mounts: []*Mount{
					{
						Source: sdk.DefaultContentSource(s.userhome, s.project.ProjectId, "manysdks", "test-sdk-2", "photos"),
						Target: "/opt/data2",
						Plug: interfaces.PlugRef{
							ProjectId: s.project.ProjectId,
							Workshop:  "manysdks",
							Sdk:       "test-sdk-2",
							Name:      "photos",
						},
					},
				},
			},
		},
	})
}

func (s *apiSuite) TestGetWorkshopInfoSomePlugsBound(c *check.C) {
	// Setup (create a running workshop)
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

	s.launchWorkshop(c, "somebound", somebound, testsdks)

	// Get Workshop info
	projectsCmd := apiCmd("/v1/projects/{id}/workshops/{name}")
	s.vars = map[string]string{"id": s.project.ProjectId, "name": "somebound"}
	req, err := s.createProjectsRequest("GET", "/v1/projects/"+s.project.ProjectId+"/workshops/somebound", nil)

	rsp := v1GetProjectWorkshop(projectsCmd, req, nil).(*resp)

	// Verify
	c.Assert(rsp.Type, check.Equals, ResponseTypeSync)
	c.Assert(rsp.Status, check.Equals, http.StatusOK)

	_, err = rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
	// for DeepEqual to work correctly
	t1, t2 := s.installTime, s.installTime
	c.Check(rsp.Result, check.DeepEquals, &WorkshopInfo{
		Name:      "somebound",
		Base:      "ubuntu@22.04",
		ProjectId: s.project.ProjectId,
		Status:    "Ready",
		Notes:     nil,
		Content: []*SdkInfo{
			{
				Name:        "test-sdk",
				Channel:     "latest/stable",
				Revision:    "0",
				InstallTime: &t1,
				Mounts: []*Mount{
					{
						Source: sdk.DefaultContentSource(s.userhome, s.project.ProjectId, "somebound", "test-sdk-2", "photos"),
						Target: "/opt/data2",
						Plug: interfaces.PlugRef{
							ProjectId: s.project.ProjectId,
							Workshop:  "somebound",
							Sdk:       "test-sdk",
							Name:      "data",
						},
					},
				},
			},
			{
				Name:        "test-sdk-2",
				Channel:     "latest/stable",
				Revision:    "0",
				InstallTime: &t2,
				Mounts: []*Mount{
					{
						Source: sdk.DefaultContentSource(s.userhome, s.project.ProjectId, "somebound", "test-sdk-2", "photos"),
						Target: "/opt/data2",
						Plug: interfaces.PlugRef{
							ProjectId: s.project.ProjectId,
							Workshop:  "somebound",
							Sdk:       "test-sdk-2",
							Name:      "photos",
						},
					},
				},
			},
		},
	})
}

type expectedResp struct {
	Type    ResponseType
	Status  int
	Message string
	Kind    string
	Summary string
}

func (s *apiSuite) runActionTest(c *check.C, buffers []*bytes.Buffer, expected []*expectedResp) {
	s.daemon(c)
	defer s.store.SetActionCallback(storeAction)()

	s.vars = map[string]string{"id": s.project.ProjectId}
	requests := []*http.Request{}

	for _, i := range buffers {
		req, err := s.createProjectsRequest("POST", "/v1/projects/"+s.project.ProjectId+"/workshops", i)
		c.Assert(err, check.IsNil)
		requests = append(requests, req)
	}

	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

	for num, req := range requests {
		// Execute
		rsp := v1PostProjectWorkshop(apiCmd("/v1/projects/{id}/workshops"), req, nil).(*resp)

		st := s.d.state
		st.Lock()
		change := st.Change(rsp.Change)
		st.Unlock()

		// Verify
		c.Check(rsp.Type, check.Equals, expected[num].Type)
		c.Check(rsp.Status, check.Equals, expected[num].Status, check.Commentf("case: %v", num))

		if rsp.Type == ResponseTypeError {
			c.Check(rsp.Result.(*errorResult).Message, check.Equals, expected[num].Message)
		}

		if rsp.Type == ResponseTypeAsync {
			ticker := time.NewTicker(100 * time.Millisecond)
		End:
			for {
				select {
				case <-change.Ready():
					break End
				case <-ticker.C:
					st.Lock()
					status := change.Status()
					st.Unlock()
					if status == state.WaitStatus {
						break End
					}
				}
			}
			c.Check(change.Kind(), check.Equals, expected[num].Kind)
			c.Check(change.Summary(), check.Equals, expected[num].Summary)
		}
	}
}

func (s *apiSuite) createWFile(c *check.C, name, yaml string) {
	err := os.WriteFile(filepath.Join(s.project.Path, fmt.Sprintf(`.workshop.%s.yaml`, name)),
		[]byte(yaml), 0644)
	c.Assert(err, check.IsNil)
}

func storeAction(ctx context.Context, currentSdks map[string]*sdk.Info, actions []sdk.SdkAction) ([]sdk.SdkResult, error) {
	var result = []sdk.SdkResult{}
	for _, act := range actions {
		info, err := sdk.ReadSdkInfo([]byte(testsdks[act.Name]), act.ProjectId, act.Workshop)
		if err != nil {
			return nil, err
		}
		info.Channel = act.Channel
		result = append(result, sdk.SdkResult{Info: info})
	}
	return result, nil
}

func (s *apiSuite) mockWorkshopFs(c *check.C, sdks map[string]string) func() {
	fs := workshop.NewFakeWorkshopFs()

	for n, s := range sdks {
		metadir := filepath.Join(sdk.SdkCurrentPath(n), "meta")
		err := fs.MkdirAll(metadir, 0655)
		c.Assert(err, check.IsNil)

		file, err := fs.OpenFile(filepath.Join(metadir, "sdk.yaml"), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
		c.Assert(err, check.IsNil)

		_, err = file.Write([]byte(s))
		c.Assert(err, check.IsNil)
	}

	s.b.WorkshopFsCallback = func(ctx context.Context, name string) (workshop.WorkshopFs, error) {
		wp, ok := s.b.Workshops[s.project.ProjectId][name]
		c.Assert(ok, check.Equals, true)
		wp.WorkshopFilesystem = fs
		return fs, nil
	}
	return func() { s.b.WorkshopFsCallback = nil }
}

func (s *apiSuite) TestLaunchWorkshopBasic(c *check.C) {
	// Setup
	s.createWFile(c, "basic", basic)

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["basic", "basic", "basic"],"action":"launch"}`),
		bytes.NewBufferString(`{"names":[],"action":"launch"}`),
		bytes.NewBufferString(`{"names":["basic"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "basic" workshop`,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "cannot launch: at least one workshop name must be provided",
		},
		{
			Type:    ResponseTypeError,
			Message: `cannot launch: "basic" already exists`,
			Status:  http.StatusBadRequest,
		},
	}

	s.runActionTest(c, requests, expected)

	_, err := s.b.Workshop(s.ctx, "basic")
	c.Assert(err, check.IsNil)
	c.Assert(s.b.AssignProfileCalls, check.HasLen, 0)
	repo := s.d.overlord.InterfaceManager().Repository()
	c.Assert(repo.Slots(s.project.ProjectId, "basic", "agent"), check.HasLen, 3)
}

func (s *apiSuite) TestWorkshopLaunchPlugBindsSuccess(c *check.C) {
	// Setup
	s.createWFile(c, "somebound", somebound)
	defer s.mockWorkshopFs(c, testsdks)()

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["somebound"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "somebound" workshop`,
		},
	}

	s.runActionTest(c, requests, expected)
	_, err := s.b.Workshop(s.ctx, "somebound")
	c.Assert(err, check.IsNil)

	repo := s.d.overlord.InterfaceManager().Repository()
	conns, err := repo.Connected(s.project.ProjectId, "somebound", "test-sdk", "data")
	c.Assert(err, check.IsNil)
	c.Assert(conns, check.HasLen, 1)

	connection, err := repo.Connection(conns[0])
	c.Assert(err, check.IsNil)
	_, bound := connection.CheckBound()
	c.Assert(bound, check.Equals, true)
}

func (s *apiSuite) TestWorkshopLaunchBindPlugNoMasterPlug(c *check.C) {
	// Setup

	s.createWFile(c, "masterunknown", masterunknown)

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["masterunknown"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `cannot bind: SDK "test-sdk-2" does not have a plug "unknown"`,
		},
	}

	s.runActionTest(c, requests, expected)
}

func (s *apiSuite) TestWorkshopLaunchBindPlugNoSlavePlug(c *check.C) {
	// Setup

	s.createWFile(c, "slaveunknown", slaveunknown)

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["slaveunknown"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `cannot bind: SDK "test-sdk" does not have a plug "unknown"`,
		},
	}

	s.runActionTest(c, requests, expected)
}

func (s *apiSuite) TestWorkshopLaunchBindPlugIncompatibleIface(c *check.C) {
	// Setup
	s.createWFile(c, "bindincompatible", bindincompatible)

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["bindincompatible"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `cannot bind: test-sdk-2:gpu and test-sdk:data must be of the same interface`,
		},
	}

	s.runActionTest(c, requests, expected)
}

func (s *apiSuite) TestRefreshWorkshopSuccess(c *check.C) {
	// Setup
	s.createWFile(c, "basic", basic)

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["basic"],"action":"launch"}`),
		bytes.NewBufferString(`{"names":["basic"],"action":"refresh","options": {"refresh-mode":"transactional"}}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "basic" workshop`,
		},
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "refresh",
			Summary: `Refresh "basic" workshop`,
		},
	}

	s.runActionTest(c, requests, expected)
}

func (s *apiSuite) TestRefreshWorkshopIncorrectInput(c *check.C) {
	// Setup
	requests := []*bytes.Buffer{
		// try continue without starting wait-on-error
		bytes.NewBufferString(`{"names":["basic"],"action":"refresh", "options": {"refresh-mode":"continue"}}`),

		// unknown refresh option
		bytes.NewBufferString(`{"names":["basic"],"action":"refresh", "options": {"refresh-mode":"unknown"}}`),

		// a workshop name is a must
		bytes.NewBufferString(`{"names":[],"action":"refresh"}`),

		// non-transactional refresh is only supported for a single workshop
		bytes.NewBufferString(`{"names":["basic", "basic1"],"action":"refresh","options": {"refresh-mode":"wait-on-error"}}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "cannot continue, no refresh in progress",
		}, {
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `cannot refresh: refresh mode must be any of: "transactional", "wait-on-error", "continue", "abort"`,
		}, {
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "cannot refresh: at least one workshop name must be provided",
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "wait-on-error is not supported for multiple workshops",
		},
	}

	s.runActionTest(c, requests, expected)
}

func (s *apiSuite) TestRefreshWorkshopContinueSuccess(c *check.C) {
	s.createWFile(c, "basic", basic)

	var errOnce sync.Once
	s.b.RemoveProfileCallback = func(ctx context.Context, workshop, profile string) error {
		var err error
		errOnce.Do(func() { err = errors.New("cannot remove profile") })
		return err
	}
	defer func() { s.b.RemoveProfileCallback = nil }()

	// Setup
	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["basic"],"action":"launch"}`),
		// start - continue (success) - continue (fail, already finished)
		bytes.NewBufferString(`{"names":["basic"],"action":"refresh","options": {"refresh-mode":"wait-on-error"}}`),
		bytes.NewBufferString(`{"names":["basic"],"action":"refresh","options": {"refresh-mode":"continue"}}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "basic" workshop`,
		},
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "refresh",
			Summary: `Refresh "basic" workshop`,
		},
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "refresh",
			Summary: `Refresh "basic" workshop`,
		},
	}
	s.runActionTest(c, requests, expected)

	st := s.d.state
	st.Lock()
	defer st.Unlock()
	// no refresh in progress after continue was successful
	err := conflict.CheckChangeConflict(st, s.project.ProjectId, "basic", "")
	c.Assert(err, check.IsNil)
}

func (s *apiSuite) TestRefreshWorkshopNoRefreshInProgress(c *check.C) {
	// Setup
	s.createWFile(c, "basic", basic)

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["basic"],"action":"launch"}`),
		bytes.NewBufferString(`{"names":["basic"],"action":"refresh","options": {"refresh-mode":"continue"}}`),
		bytes.NewBufferString(`{"names":["basic"],"action":"refresh","options": {"refresh-mode":"abort"}}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "basic" workshop`,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "cannot continue, no refresh in progress",
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: "cannot abort, no refresh in progress",
		},
	}

	s.runActionTest(c, requests, expected)
}

func (s *apiSuite) TestRefreshWorkshopRefreshAbort(c *check.C) {
	// Setup
	s.createWFile(c, "basic", basic)

	var errOnce sync.Once
	s.b.RemoveProfileCallback = func(ctx context.Context, workshop, profile string) error {
		var err error
		errOnce.Do(func() { err = errors.New("cannot remove profile") })
		return err
	}
	defer func() { s.b.RemoveProfileCallback = nil }()

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["basic"],"action":"launch"}`),
		// start - abort (both success)
		bytes.NewBufferString(`{"names":["basic"],"action":"refresh","options": {"refresh-mode":"wait-on-error"}}`),
		bytes.NewBufferString(`{"names":["basic"],"action":"refresh","options": {"refresh-mode":"abort"}}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "basic" workshop`,
		},
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "refresh",
			Summary: `Refresh "basic" workshop`,
		},
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "refresh",
			Summary: `Refresh "basic" workshop`,
		},
	}

	s.runActionTest(c, requests, expected)

	st := s.d.state
	st.Lock()
	defer st.Unlock()
	// no refresh in progress after continue was successful
	err := conflict.CheckChangeConflict(st, s.project.ProjectId, "basic", "")
	c.Assert(err, check.IsNil)
}

func (s *apiSuite) TestStartWorkshop(c *check.C) {
	// Setup
	s.createWFile(c, "basic", basic)
	// Setup
	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["basic"],"action":"launch"}`),

		bytes.NewBufferString(`{"names":["basic"],"action":"stop"}`),
		//
		bytes.NewBufferString(`{"names":["basic"],"action":"start"}`),
		// a second attempt to start the workshop that is already in Started
		bytes.NewBufferString(`{"names":["basic"],"action":"start"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "basic" workshop`,
		},
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "stop",
			Summary: `Stop "basic" workshop`,
		},
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "start",
			Summary: `Start "basic" workshop`,
		},
		{
			Type:    ResponseTypeError,
			Status:  http.StatusBadRequest,
			Message: `cannot start: "basic" status is "Ready", must be one of: "Stopped"`,
		},
	}

	s.runActionTest(c, requests, expected)

	wp, err := s.b.Workshop(s.ctx, "basic")
	c.Assert(err, check.IsNil)
	c.Assert(wp.Running, check.Equals, true)
}

func (s *apiSuite) TestStopWorkshop(c *check.C) {
	// Setup
	s.createWFile(c, "basic", basic)

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["basic"],"action":"launch"}`),

		bytes.NewBufferString(`{"names":["basic"],"action":"stop"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "basic" workshop`,
		},
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "stop",
			Summary: `Stop "basic" workshop`,
		},
	}

	s.runActionTest(c, requests, expected)

	wp, err := s.b.Workshop(s.ctx, "basic")
	c.Assert(err, check.IsNil)
	c.Assert(wp.Running, check.Equals, false)
}

func (s *apiSuite) TestRemoveWorkshopSuccess(c *check.C) {
	// Setup

	s.createWFile(c, "basic", basic)

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["basic"],"action":"launch"}`),
		bytes.NewBufferString(`{"names":["basic"],"action":"remove"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "basic" workshop`,
		},
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "remove",
			Summary: `Remove "basic" workshop`,
		},
	}

	s.runActionTest(c, requests, expected)

	_, err := s.b.Workshop(s.ctx, "basic")
	c.Check(err, testutil.ErrorIs, workshop.ErrWorkshopNotFound)
	c.Check(s.b.RemoveProfileCalls, check.HasLen, 1)
}
