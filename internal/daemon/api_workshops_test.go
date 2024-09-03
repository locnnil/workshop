package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	basic_refreshed = `name: basic
base: ubuntu@22.04
sdks:
  test-sdk:
    channel: latest/stable
  test-sdk-2:
    channel: latest/stable
`

	manysdks = `name: manysdks
base: ubuntu@22.04
sdks:
  test-sdk:
    channel: latest/stable
  test-sdk-2:
    channel: latest/stable
`
	manysdks_refreshed = `name: manysdks
base: ubuntu@22.04
sdks:
  test-sdk:
    channel: latest/stable
  test-sdk-2:
    channel: latest/stable
connections:
  - plug: test-sdk:data-non-existent
    slot: host:content
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
      unknown-data:
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

	workshopplug = `name: workshopplug
base: ubuntu@22.04
sdks:
  host:
    slots:
      training-slot:
        interface: content
        source: .
  test-sdk:
    channel: latest/stable
    plugs:
      training-plug:
        interface: content
        target: /opt
  test-sdk-2:
    channel: latest/stable
connections:
  - plug: test-sdk:training-plug
    slot: host:training-slot
`

	workshopplugbound = `name: workshopplugbound
base: ubuntu@22.04
sdks:
  host:
    slots:
      training-slot:
        interface: content
        source: .
  test-sdk:
    channel: latest/stable
    plugs:
      training-plug:
        interface: content
        target: /opt
      data:
        bind: test-sdk:training-plug
  test-sdk-2:
    channel: latest/stable
connections:
  - plug: test-sdk:training-plug
    slot: host:training-slot
`

	workshopslot = `name: workshopslot
base: ubuntu@22.04
sdks:
  host:
    slots:
      training:
        interface: content
        source: .
  test-sdk:
    channel: latest/stable
  test-sdk-2:
    channel: latest/stable
`

	workshopconns = `name: workshopconns
base: ubuntu@22.04
sdks:
  host:
    slots:
      training:
        interface: content
        source: .
  test-sdk:
    channel: latest/stable
  test-sdk-2:
    channel: latest/stable
connections:
  - plug: test-sdk:data
    slot: host:training
`

	workshopconns_refreshed = `name: workshopconns
base: ubuntu@22.04
sdks:
  test-sdk:
    channel: latest/stable
  test-sdk-2:
    channel: latest/stable
`

	workshopbrokenconn = `name: workshopbrokenconn
base: ubuntu@22.04
sdks:
  test-sdk:
    channel: latest/stable
  test-sdk-2:
    channel: latest/stable
connections:
  - plug: test-sdk:data-unknown-plug
    slot: host:content
`

	connsplugbound = `name: connsplugbound
base: ubuntu@22.04
sdks:
  host:
    slots:
      training:
        interface: content
        source: .
      photos:
        interface: content
        source: .
  test-sdk:
    channel: latest/stable
  test-sdk-2:
    channel: latest/stable
    plugs:
      photos: 
        bind: test-sdk:data
connections:
  - plug: test-sdk:data
    slot: host:training
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
	defer s.mockDoInstallSdk(c, name, sdks)()

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
						Source: sdk.SdkContentSource(s.userhome, s.project.ProjectId, "manysdks", "test-sdk", "data"),
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
						Source: sdk.SdkContentSource(s.userhome, s.project.ProjectId, "manysdks", "test-sdk-2", "photos"),
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
						Source: sdk.SdkContentSource(s.userhome, s.project.ProjectId, "somebound", "test-sdk-2", "photos"),
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
						Source: sdk.SdkContentSource(s.userhome, s.project.ProjectId, "somebound", "test-sdk-2", "photos"),
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
	Type      ResponseType
	Status    int
	Message   string
	Kind      string
	Summary   string
	ChangeErr string // an error that happens during the change execution
}

func (s *apiSuite) runActionTest(c *check.C, buffers []*bytes.Buffer, expected []*expectedResp) {
	defer s.store.SetActionCallback(storeAction)()

	s.vars = map[string]string{"id": s.project.ProjectId}
	requests := []*http.Request{}

	for _, i := range buffers {
		req, err := s.createProjectsRequest("POST", "/v1/projects/"+s.project.ProjectId+"/workshops", i)
		c.Assert(err, check.IsNil)
		requests = append(requests, req)
	}

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
						// some tests (refresh continue/abort) leave the change
						// in the wait state and this is expected.
						break End
					}
				}
			}
			c.Check(change.Kind(), check.Equals, expected[num].Kind)
			c.Check(change.Summary(), check.Equals, expected[num].Summary)
			st.Lock()
			if expected[num].ChangeErr != "" {
				c.Check(change.Err(), check.ErrorMatches, expected[num].ChangeErr)
			} else {
				c.Assert(change.Err(), check.IsNil)
			}
			st.Unlock()
		}
	}
}

func (s *apiSuite) createWFile(c *check.C, name, yaml string) {
	err := os.WriteFile(filepath.Join(s.project.Path, fmt.Sprintf(`.workshop.%s.yaml`, name)),
		[]byte(yaml), 0644)
	c.Assert(err, check.IsNil)
}

func storeAction(ctx context.Context, actions []sdk.SdkAction) ([]sdk.SdkResult, error) {
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

func (s *apiSuite) mockDoInstallSdk(c *check.C, ws string, sdks map[string]string) func() {
	s.b.ExecCallback = func(ctx context.Context, name string, args *workshop.Execution) (workshop.ExecContext, error) {
		// check if the command is to install an SDK
		if args.Command[0] != "tar" {
			return workshop.ExecContext{WaitExecution: func(ctx context.Context) error { return nil }}, nil
		}

		fs, err := s.b.WorkshopFs(s.ctx, ws)
		c.Check(err, check.IsNil)
		defer fs.Close()
		// Get the SDK name from the exec command (we don't have a separate
		// method to install an SDK now).
		sdkname, found := strings.CutSuffix(filepath.Base(args.Command[3]), "_0.sdk")
		c.Check(found, check.Equals, true)
		metadir := filepath.Join(sdk.SdkCurrentPath(sdkname), "meta")
		err = fs.MkdirAll(metadir, 0655)
		c.Check(err, check.IsNil)

		file, err := fs.OpenFile(filepath.Join(metadir, "sdk.yaml"), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
		c.Check(err, check.IsNil)
		defer file.Close()

		syaml, exists := sdks[sdkname]
		c.Check(exists, check.Equals, true)
		_, err = file.Write([]byte(syaml))
		c.Check(err, check.IsNil)

		for _, hook := range []string{"setup-base", "save-state", "restore-state"} {
			setuphook, err := fs.OpenFile(sdk.SdkHookPath(sdkname, hook), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0744)
			c.Check(err, check.IsNil)
			defer setuphook.Close()
		}
		return workshop.ExecContext{WaitExecution: func(ctx context.Context) error { return nil }}, nil
	}
	return func() { s.b.ExecCallback = nil }
}

func (s *apiSuite) TestLaunchWorkshopBasic(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

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
	c.Assert(repo.Slots(s.project.ProjectId, "basic", "host"), check.HasLen, 3)
}

func (s *apiSuite) TestLaunchWorkshopWithSlotOK(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
	// Setup
	s.createWFile(c, "workshopslot", workshopslot)
	defer s.mockDoInstallSdk(c, "workshopslot", testsdks)()

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["workshopslot"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "workshopslot" workshop`,
		},
	}

	s.runActionTest(c, requests, expected)

	_, err := s.b.Workshop(s.ctx, "workshopslot")
	c.Assert(err, check.IsNil)

	repo := s.d.overlord.InterfaceManager().Repository()
	c.Assert(repo.Slot(s.project.ProjectId, "workshopslot", "host", "training"), check.Not(check.IsNil))
}

func (s *apiSuite) TestLaunchWorkshopFailed(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
	// Setup
	s.createWFile(c, "manysdks", manysdks)
	defer s.mockDoInstallSdk(c, "manysdks", testsdks)()

	s.b.AssignProfileCallback = func(ctx context.Context, workshop string, profile workshop.SdkProfile) error {
		return fmt.Errorf(`cannot assign profile to %q`, workshop)
	}
	defer func() { s.b.AssignProfileCallback = nil }()

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["manysdks"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:      ResponseTypeAsync,
			Status:    http.StatusAccepted,
			Kind:      "launch",
			Summary:   `Launch "manysdks" workshop`,
			ChangeErr: `(?s).*cannot assign profile to "manysdks".*`,
		},
	}

	s.runActionTest(c, requests, expected)

	_, err := s.b.Workshop(s.ctx, "manysdks")
	c.Assert(err, testutil.ErrorIs, workshop.ErrWorkshopNotFound)

	repo := s.d.overlord.InterfaceManager().Repository()
	c.Assert(repo.Slots(s.project.ProjectId, "manysdks", "host"), check.HasLen, 0)
	c.Assert(repo.Plugs(s.project.ProjectId, "manysdks", "host"), check.HasLen, 0)

	c.Assert(repo.Slots(s.project.ProjectId, "manysdks", "test-sdk"), check.HasLen, 0)
	c.Assert(repo.Plugs(s.project.ProjectId, "manysdks", "test-sdk"), check.HasLen, 0)

	c.Assert(repo.Slots(s.project.ProjectId, "manysdks", "test-sdk-2"), check.HasLen, 0)
	c.Assert(repo.Plugs(s.project.ProjectId, "manysdks", "test-sdk-2"), check.HasLen, 0)
}

func (s *apiSuite) TestLaunchWorkshopPlugBindsSuccess(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
	// Setup
	s.createWFile(c, "somebound", somebound)
	defer s.mockDoInstallSdk(c, "somebound", testsdks)()

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

func (s *apiSuite) TestLaunchWorkshopBindPlugNoMasterPlug(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
	// Setup
	s.createWFile(c, "masterunknown", masterunknown)
	defer s.mockDoInstallSdk(c, "masterunknown", testsdks)()

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["masterunknown"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:      ResponseTypeAsync,
			Status:    http.StatusAccepted,
			Kind:      "launch",
			Summary:   `Launch "masterunknown" workshop`,
			ChangeErr: `(?s).*SDK masterunknown/test-sdk has no "unknown-data" plug.*`,
		},
	}

	s.runActionTest(c, requests, expected)
}

func (s *apiSuite) TestLaunchWorkshopBindPlugNoSlavePlug(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
	// Setup
	s.createWFile(c, "slaveunknown", slaveunknown)
	defer s.mockDoInstallSdk(c, "slaveunknown", testsdks)()

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["slaveunknown"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:      ResponseTypeAsync,
			Status:    http.StatusAccepted,
			Kind:      "launch",
			Summary:   `Launch "slaveunknown" workshop`,
			ChangeErr: `(?s).*SDK slaveunknown/test-sdk has no "unknown" plug.*`,
		},
	}

	s.runActionTest(c, requests, expected)
}

func (s *apiSuite) TestLaunchWorkshopBindPlugIncompatibleIface(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
	// Setup
	s.createWFile(c, "bindincompatible", bindincompatible)
	defer s.mockDoInstallSdk(c, "bindincompatible", testsdks)()

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["bindincompatible"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:      ResponseTypeAsync,
			Status:    http.StatusAccepted,
			Kind:      "launch",
			Summary:   `Launch "bindincompatible" workshop`,
			ChangeErr: `(?s).*cannot bind bindincompatible/test-sdk:data \("content" interface\) to bindincompatible/test-sdk-2:gpu \("gpu" interface\).*`,
		},
	}

	s.runActionTest(c, requests, expected)
}

func (s *apiSuite) TestLaunchWorkshopWithPlugOK(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

	// Setup
	s.createWFile(c, "workshopplug", workshopplug)
	defer s.mockDoInstallSdk(c, "workshopplug", testsdks)()

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["workshopplug"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "workshopplug" workshop`,
		},
	}

	s.runActionTest(c, requests, expected)

	repo := s.d.overlord.InterfaceManager().Repository()
	c.Assert(repo.Plug(s.project.ProjectId, "workshopplug", "test-sdk", "training-plug"), check.NotNil)
	conns, err := repo.Connected(s.project.ProjectId, "workshopplug", "test-sdk", "training-plug")
	c.Assert(err, check.IsNil)
	c.Assert(conns, check.HasLen, 1)
	c.Assert(conns[0].ID(), check.Equals, fmt.Sprintf(`%s/workshopplug/test-sdk:training-plug %s/workshopplug/host:training-slot`, s.project.ProjectId, s.project.ProjectId))
}

func (s *apiSuite) TestLaunchWorkshopPlugAddedAndBound(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

	// Setup
	s.createWFile(c, "workshopplugbound", workshopplugbound)
	defer s.mockDoInstallSdk(c, "workshopplugbound", testsdks)()

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["workshopplugbound"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "workshopplugbound" workshop`,
		},
	}

	s.runActionTest(c, requests, expected)

	repo := s.d.overlord.InterfaceManager().Repository()
	c.Assert(repo.Plug(s.project.ProjectId, "workshopplugbound", "test-sdk", "training-plug"), check.NotNil)
	conns, err := repo.Connected(s.project.ProjectId, "workshopplugbound", "test-sdk", "training-plug")
	c.Assert(err, check.IsNil)
	c.Assert(conns, check.HasLen, 1)
	c.Assert(conns[0].ID(), check.Equals, fmt.Sprintf(`%s/workshopplugbound/test-sdk:training-plug %s/workshopplugbound/host:training-slot`, s.project.ProjectId, s.project.ProjectId))

	conns, err = repo.Connected(s.project.ProjectId, "workshopplugbound", "test-sdk", "data")
	c.Assert(err, check.IsNil)
	c.Assert(conns, check.HasLen, 1)
	c.Assert(conns[0].ID(), check.Equals, fmt.Sprintf(`%s/workshopplugbound/test-sdk:data %s/workshopplugbound/host:training-slot`, s.project.ProjectId, s.project.ProjectId))
}

func (s *apiSuite) TestWorkshopConnectionsOK(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
	// Setup
	s.createWFile(c, "workshopconns", workshopconns)
	defer s.mockDoInstallSdk(c, "workshopconns", testsdks)()

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["workshopconns"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "workshopconns" workshop`,
		},
	}

	s.runActionTest(c, requests, expected)

	_, err := s.b.Workshop(s.ctx, "workshopconns")
	c.Assert(err, check.IsNil)

	repo := s.d.overlord.InterfaceManager().Repository()
	c.Assert(repo.Slot(s.project.ProjectId, "workshopconns", "host", "training"), check.Not(check.IsNil))

	conns, err := repo.Connections(s.project.ProjectId, "workshopconns", "test-sdk")
	c.Assert(err, check.IsNil)
	c.Assert(conns, testutil.DeepUnsortedMatches, []*interfaces.ConnRef{
		{
			PlugRef: interfaces.PlugRef{ProjectId: s.project.ProjectId, Workshop: "workshopconns", Sdk: "test-sdk", Name: "data"},
			SlotRef: interfaces.SlotRef{ProjectId: s.project.ProjectId, Workshop: "workshopconns", Sdk: "host", Name: "training"},
		},
	})

	conns, err = repo.Connections(s.project.ProjectId, "workshopconns", "test-sdk-2")
	c.Assert(err, check.IsNil)
	c.Assert(conns, testutil.DeepUnsortedMatches, []*interfaces.ConnRef{
		{
			PlugRef: interfaces.PlugRef{ProjectId: s.project.ProjectId, Workshop: "workshopconns", Sdk: "test-sdk-2", Name: "photos"},
			SlotRef: interfaces.SlotRef{ProjectId: s.project.ProjectId, Workshop: "workshopconns", Sdk: "host", Name: "content"},
		}, {
			PlugRef: interfaces.PlugRef{ProjectId: s.project.ProjectId, Workshop: "workshopconns", Sdk: "test-sdk-2", Name: "gpu"},
			SlotRef: interfaces.SlotRef{ProjectId: s.project.ProjectId, Workshop: "workshopconns", Sdk: "host", Name: "gpu"},
		},
	})
}

func (s *apiSuite) TestWorkshopConnectionsUnknownPlug(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
	// Setup
	s.createWFile(c, "workshopbrokenconn", workshopbrokenconn)
	defer s.mockDoInstallSdk(c, "workshopbrokenconn", testsdks)()

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["workshopbrokenconn"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:      ResponseTypeAsync,
			Status:    http.StatusAccepted,
			Kind:      "launch",
			Summary:   `Launch "workshopbrokenconn" workshop`,
			ChangeErr: `(?s).*SDK "workshopbrokenconn/test-sdk" has no plug named "data-unknown-plug".*`,
		},
	}

	s.runActionTest(c, requests, expected)

	repo := s.d.overlord.InterfaceManager().Repository()
	conns, err := repo.Connections(s.project.ProjectId, "workshopbrokenconn", "test-sdk")
	c.Assert(err, check.IsNil)
	c.Assert(conns, check.HasLen, 0)
	conns, err = repo.Connections(s.project.ProjectId, "workshopbrokenconn", "test-sdk-2")
	c.Assert(err, check.IsNil)
	c.Assert(conns, check.HasLen, 0)
}

func (s *apiSuite) TestWorkshopConnectionsPlugIsBoundTo(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
	// Setup
	s.createWFile(c, "connsplugbound", connsplugbound)
	defer s.mockDoInstallSdk(c, "connsplugbound", testsdks)()

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["connsplugbound"],"action":"launch"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "connsplugbound" workshop`,
		},
	}

	s.runActionTest(c, requests, expected)

	repo := s.d.overlord.InterfaceManager().Repository()
	conns, err := repo.Connected(s.project.ProjectId, "connsplugbound", "test-sdk", "data")
	c.Assert(err, check.IsNil)
	c.Assert(conns, check.HasLen, 1)
	c.Assert(conns[0].SlotRef.Name, check.Equals, "training")

	conns, err = repo.Connected(s.project.ProjectId, "connsplugbound", "test-sdk-2", "photos")
	c.Assert(err, check.IsNil)
	c.Assert(conns, check.HasLen, 1)
	c.Assert(conns[0].SlotRef.Name, check.Equals, "training")

	connection, err := repo.Connection(conns[0])
	c.Assert(err, check.IsNil)
	_, bound := connection.CheckBound()
	c.Assert(bound, check.Equals, true)
}

func (s *apiSuite) TestRefreshWorkshopSuccess(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
	// Setup
	s.createWFile(c, "basic", basic)

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["basic"],"action":"launch"}`),
	}
	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "basic" workshop`,
		},
	}
	s.runActionTest(c, requests, expected)

	s.createWFile(c, "basic", basic_refreshed)
	defer s.mockDoInstallSdk(c, "basic", testsdks)()

	requests = []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["basic"],"action":"refresh","options": {"refresh-mode":"transactional"}}`),
	}
	expected = []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "refresh",
			Summary: `Refresh "basic" workshop`,
		},
	}

	s.runActionTest(c, requests, expected)

	repo := s.d.overlord.InterfaceManager().Repository()

	conns, err := repo.Connections(s.project.ProjectId, "basic", "test-sdk")
	c.Assert(err, check.IsNil)
	c.Assert(conns, testutil.DeepUnsortedMatches, []*interfaces.ConnRef{
		{
			PlugRef: interfaces.PlugRef{ProjectId: s.project.ProjectId, Workshop: "basic", Sdk: "test-sdk", Name: "data"},
			SlotRef: interfaces.SlotRef{ProjectId: s.project.ProjectId, Workshop: "basic", Sdk: "host", Name: "content"},
		},
	})

	conns, err = repo.Connections(s.project.ProjectId, "basic", "test-sdk-2")
	c.Assert(err, check.IsNil)
	c.Assert(conns, testutil.DeepUnsortedMatches, []*interfaces.ConnRef{
		{
			PlugRef: interfaces.PlugRef{ProjectId: s.project.ProjectId, Workshop: "basic", Sdk: "test-sdk-2", Name: "photos"},
			SlotRef: interfaces.SlotRef{ProjectId: s.project.ProjectId, Workshop: "basic", Sdk: "host", Name: "content"},
		}, {
			PlugRef: interfaces.PlugRef{ProjectId: s.project.ProjectId, Workshop: "basic", Sdk: "test-sdk-2", Name: "gpu"},
			SlotRef: interfaces.SlotRef{ProjectId: s.project.ProjectId, Workshop: "basic", Sdk: "host", Name: "gpu"},
		},
	})
}

func (s *apiSuite) TestRefreshWorkshopReturnsPreviousWorkshopIfFailed(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
	// Setup
	s.createWFile(c, "manysdks", manysdks)
	defer s.mockDoInstallSdk(c, "manysdks", testsdks)()

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["manysdks"],"action":"launch"}`)}

	expected := []*expectedResp{{
		Type:    ResponseTypeAsync,
		Status:  http.StatusAccepted,
		Kind:    "launch",
		Summary: `Launch "manysdks" workshop`,
	}}
	s.runActionTest(c, requests, expected)

	// Setup "refresh" with a new workshop that will trigger an error
	s.createWFile(c, "manysdks", manysdks_refreshed)
	requests = []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["manysdks"],"action":"refresh","options": {"refresh-mode":"transactional"}}`)}
	expected = []*expectedResp{{
		Type:      ResponseTypeAsync,
		Status:    http.StatusAccepted,
		Kind:      "refresh",
		Summary:   `Refresh "manysdks" workshop`,
		ChangeErr: `(?s).*SDK "manysdks/test-sdk" has no plug named "data-non-existent".*`,
	}}
	s.runActionTest(c, requests, expected)

	wp, err := s.b.Workshop(s.ctx, "manysdks")
	c.Assert(err, check.IsNil)

	content, err := wp.ContentInfo(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(content, check.HasLen, 2)

	repo := s.d.overlord.InterfaceManager().Repository()
	conns, err := repo.Connections(s.project.ProjectId, "manysdks", "test-sdk")
	c.Assert(err, check.IsNil)

	c.Assert(conns, testutil.DeepUnsortedMatches, []*interfaces.ConnRef{
		{
			PlugRef: interfaces.PlugRef{ProjectId: s.project.ProjectId, Workshop: "manysdks", Sdk: "test-sdk", Name: "data"},
			SlotRef: interfaces.SlotRef{ProjectId: s.project.ProjectId, Workshop: "manysdks", Sdk: "host", Name: "content"},
		},
	})

	conns, err = repo.Connections(s.project.ProjectId, "manysdks", "test-sdk-2")
	c.Assert(err, check.IsNil)
	c.Assert(conns, testutil.DeepUnsortedMatches, []*interfaces.ConnRef{
		{
			PlugRef: interfaces.PlugRef{ProjectId: s.project.ProjectId, Workshop: "manysdks", Sdk: "test-sdk-2", Name: "photos"},
			SlotRef: interfaces.SlotRef{ProjectId: s.project.ProjectId, Workshop: "manysdks", Sdk: "host", Name: "content"},
		},
		{
			PlugRef: interfaces.PlugRef{ProjectId: s.project.ProjectId, Workshop: "manysdks", Sdk: "test-sdk-2", Name: "gpu"},
			SlotRef: interfaces.SlotRef{ProjectId: s.project.ProjectId, Workshop: "manysdks", Sdk: "host", Name: "gpu"},
		},
	})
}

func (s *apiSuite) TestRefreshWorkshopIncorrectInput(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
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
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
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
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
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
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
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
			Type:      ResponseTypeAsync,
			Status:    http.StatusAccepted,
			Kind:      "refresh",
			Summary:   `Refresh "basic" workshop`,
			ChangeErr: `(?s).*cannot remove profile.*`,
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

func (s *apiSuite) TestRefreshWorkshopRestoreUserDefinedConnsIfFailed(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

	// Setup "launch"
	s.createWFile(c, "workshopconns", workshopconns)
	defer s.mockDoInstallSdk(c, "workshopconns", testsdks)()

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["workshopconns"],"action":"launch"}`),
	}

	expected := []*expectedResp{{
		Type:    ResponseTypeAsync,
		Status:  http.StatusAccepted,
		Kind:    "launch",
		Summary: `Launch "workshopconns" workshop`,
	}}
	s.runActionTest(c, requests, expected)

	// Validate
	wp, err := s.b.Workshop(s.ctx, "workshopconns")
	c.Assert(err, check.IsNil)

	// Setup "refresh"
	s.createWFile(c, "workshopconns", workshopconns_refreshed)
	requests = []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["workshopconns"],"action":"refresh","options": {"refresh-mode":"transactional"}}`),
	}
	expected = []*expectedResp{{
		Type:      ResponseTypeAsync,
		Status:    http.StatusAccepted,
		Kind:      "refresh",
		Summary:   `Refresh "workshopconns" workshop`,
		ChangeErr: `(?s)*.cannot assign profile to "workshopconns".*`,
	}}
	var errOnce sync.Once
	s.b.AssignProfileCallback = func(ctx context.Context, workshop string, profile workshop.SdkProfile) error {
		var err error
		// trigger the refresh failure
		errOnce.Do(func() {
			err = fmt.Errorf(`cannot assign profile to %q`, workshop)
		})
		return err
	}
	defer func() { s.b.AssignProfileCallback = nil }()

	s.runActionTest(c, requests, expected)

	content, err := wp.ContentInfo(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(content, check.HasLen, 2)

	repo := s.d.overlord.InterfaceManager().Repository()
	conns, err := repo.Connections(s.project.ProjectId, "workshopconns", "test-sdk")
	c.Assert(err, check.IsNil)

	c.Assert(conns, testutil.DeepUnsortedMatches, []*interfaces.ConnRef{
		{
			PlugRef: interfaces.PlugRef{ProjectId: s.project.ProjectId, Workshop: "workshopconns", Sdk: "test-sdk", Name: "data"},
			SlotRef: interfaces.SlotRef{ProjectId: s.project.ProjectId, Workshop: "workshopconns", Sdk: "host", Name: "training"},
		},
	})

	conns, err = repo.Connections(s.project.ProjectId, "workshopconns", "test-sdk-2")
	c.Assert(err, check.IsNil)
	c.Assert(conns, testutil.DeepUnsortedMatches, []*interfaces.ConnRef{
		{
			PlugRef: interfaces.PlugRef{ProjectId: s.project.ProjectId, Workshop: "workshopconns", Sdk: "test-sdk-2", Name: "photos"},
			SlotRef: interfaces.SlotRef{ProjectId: s.project.ProjectId, Workshop: "workshopconns", Sdk: "host", Name: "content"},
		},
		{
			PlugRef: interfaces.PlugRef{ProjectId: s.project.ProjectId, Workshop: "workshopconns", Sdk: "test-sdk-2", Name: "gpu"},
			SlotRef: interfaces.SlotRef{ProjectId: s.project.ProjectId, Workshop: "workshopconns", Sdk: "host", Name: "gpu"},
		},
	})
}

func (s *apiSuite) TestStartWorkshop(c *check.C) {
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
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
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()
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
	s.daemon(c)
	s.d.Overlord().Loop()
	defer s.d.Overlord().Stop()

	// Setup
	s.createWFile(c, "workshopconns", workshopconns)
	defer s.mockDoInstallSdk(c, "workshopconns", testsdks)()

	requests := []*bytes.Buffer{
		bytes.NewBufferString(`{"names":["workshopconns"],"action":"launch"}`),
		bytes.NewBufferString(`{"names":["workshopconns"],"action":"remove"}`),
	}

	expected := []*expectedResp{
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "launch",
			Summary: `Launch "workshopconns" workshop`,
		},
		{
			Type:    ResponseTypeAsync,
			Status:  http.StatusAccepted,
			Kind:    "remove",
			Summary: `Remove "workshopconns" workshop`,
		},
	}

	s.runActionTest(c, requests, expected)

	_, err := s.b.Workshop(s.ctx, "workshopconns")
	c.Check(err, testutil.ErrorIs, workshop.ErrWorkshopNotFound)
	c.Check(s.b.RemoveProfileCalls, check.HasLen, 3)
}
