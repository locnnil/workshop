package ifacestate_test

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"path/filepath"

	"github.com/spf13/afero"
	"golang.org/x/exp/maps"
	"gopkg.in/check.v1"
	"gopkg.in/yaml.v3"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/ifacetest"
	"github.com/canonical/workshop/internal/overlord"
	"github.com/canonical/workshop/internal/overlord/ifacestate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	"github.com/canonical/workshop/internal/workshop/fakebackend"
)

type interfaceManagerSuite struct {
	testutil.BaseTest
	o          *overlord.Overlord
	state      *state.State
	se         *overlord.StateEngine
	runner     *state.TaskRunner
	ctx        context.Context
	wsbackend  workshop.Backend
	prj        *workshop.Project
	secBackend *ifacetest.TestSecurityBackend

	restoreProjectId func()
}

var _ = check.Suite(&interfaceManagerSuite{})

func (s *interfaceManagerSuite) SetUpTest(c *check.C) {
	s.BaseTest.SetUpTest(c)
	var err error

	s.o = overlord.Fake()
	s.state = s.o.State()

	s.se = s.o.StateEngine()
	s.runner = state.NewTaskRunner(s.state)
	s.secBackend = &ifacetest.TestSecurityBackend{}

	s.restoreProjectId = testutil.FakeFunc(func() (string, error) { return "42424242", nil }, &workshop.NewProjectId)

	s.wsbackend = fakebackend.New()
	s.ctx = context.WithValue(context.Background(), workshop.ContextUser, "testuser")
	s.prj, _, err = s.wsbackend.CreateOrLoadProject(s.ctx, c.MkDir())
	c.Assert(err, check.IsNil)
	s.ctx = context.WithValue(s.ctx, workshop.ContextProjectId, s.prj.ProjectId)

	s.BaseTest.AddCleanup(sdk.MockSanitizePlugsSlots(func(sdkInfo *sdk.Info) {}))
}

func (s *interfaceManagerSuite) TearDownTest(c *check.C) {
	s.restoreProjectId()
	s.BaseTest.TearDownTest(c)
}

func (s *interfaceManagerSuite) writeSDKMetaFile(c *check.C, fs workshop.WorkshopFs, name, yaml string) {
	sdkPath := filepath.Join(dirs.WorkshopSdksDir, name, "current", "meta", "sdk.yaml")
	c.Assert(afero.WriteFile(fs, sdkPath, []byte(yaml), 0644), check.IsNil)
}

func (s *interfaceManagerSuite) launchWorkshop(c *check.C, ws string, sdkYamls map[sdk.Setup]string) (*workshop.Workshop, error) {
	ctx := context.WithValue(s.ctx, workshop.ContextProjectId, s.prj.ProjectId)

	t, err := template.New("workshop").Parse(fmt.Sprintf(workshopTemplate, ws))
	c.Assert(err, check.IsNil)

	var workshopFile = bytes.NewBuffer([]byte{})
	t.Execute(workshopFile, maps.Keys(sdkYamls))

	var wf workshop.File
	err = yaml.Unmarshal(workshopFile.Bytes(), &wf)
	c.Assert(err, check.IsNil)

	err = s.wsbackend.LaunchWorkshop(ctx, &wf)
	c.Assert(err, check.IsNil)

	wsfs, err := s.wsbackend.WorkshopFs(ctx, ws)
	c.Assert(err, check.IsNil)
	defer wsfs.Close()

	var hostYaml = `name: host
base: ubuntu@22.04
type: host
slots:
  slot:
    interface: content
    attr: slot-value
`
	c.Assert(wsfs.MkdirAll(filepath.Join(dirs.WorkshopSdksDir, "host", "current", "meta", "sdk.yaml"), 0655), check.IsNil)
	s.writeSDKMetaFile(c, wsfs, "host", hostYaml)
	for sdk, yaml := range sdkYamls {
		s.writeSDKMetaFile(c, wsfs, sdk.Name, yaml)
	}

	w, err := s.wsbackend.Workshop(ctx, ws)
	c.Assert(err, check.IsNil)

	for s := range sdkYamls {
		if err = w.LinkSdk(ctx, s); err != nil {
			c.Assert(err, check.IsNil)
		}
	}

	return w, nil
}

func (s *interfaceManagerSuite) TestManagerReloadsConnections(c *check.C) {
	var consumerYaml = `
name: consumer
base: ubuntu@22.04
plugs:
 plug:
  interface: content
  attr: plug-value
`

	s.launchWorkshop(c, "ws", map[sdk.Setup]string{
		{Name: "consumer", Channel: "latest/stable"}: consumerYaml,
	})

	s.state.Lock()
	key := fmt.Sprintf("%s/ws/consumer:plug %s/ws/host:slot", s.prj.ProjectId, s.prj.ProjectId)
	s.state.Set("conns", map[string]interface{}{
		key: map[string]interface{}{
			"interface": "content",
			"plug-static": map[string]interface{}{
				"content": "foo",
				"attr":    "stored-value",
			},
			"slot-static": map[string]interface{}{
				"content": "foo",
				"attr":    "stored-value",
			},
		},
	})
	s.state.Unlock()

	mgr := ifacestate.New(s.state, s.o.TaskRunner(), s.wsbackend)
	err := mgr.StartUp()
	c.Assert(err, check.IsNil)

	repo := mgr.Repository()

	ifaces := repo.Interfaces()
	c.Assert(ifaces.Connections, check.HasLen, 1)
	cref := &interfaces.ConnRef{
		PlugRef: interfaces.PlugRef{ProjectId: s.prj.ProjectId, Workshop: "ws", Sdk: "consumer", Name: "plug"},
		SlotRef: interfaces.SlotRef{ProjectId: s.prj.ProjectId, Workshop: "ws", Sdk: "host", Name: "slot"}}
	c.Check(ifaces.Connections, check.DeepEquals, []*interfaces.ConnRef{cref})

	conn, err := repo.Connection(cref)
	c.Assert(err, check.IsNil)
	c.Assert(conn.Plug.Name(), check.Equals, "plug")
	c.Assert(conn.Plug.StaticAttrs(), check.DeepEquals, map[string]interface{}{
		"content": "foo",
		"attr":    "stored-value",
	})
	c.Assert(conn.Slot.Name(), check.Equals, "slot")
	c.Assert(conn.Slot.StaticAttrs(), check.DeepEquals, map[string]interface{}{
		"content": "foo",
		"attr":    "stored-value",
	})
}

func (s *interfaceManagerSuite) TestManagerDoesntReloadUndesiredAutoconnections(c *check.C) {
	var consumerYaml = `
name: consumer
base: ubuntu@22.04
plugs:
 plug:
  interface: content
  attr1: value1
 otherplug:
  interface: content
`

	var producerYaml = `
name: producer
base: ubuntu@22.04
slots:
 slot:
  interface: content
  attr2: value2
`
	s.launchWorkshop(c, "ws", map[sdk.Setup]string{
		{Name: "consumer", Channel: "latest/stable"}: consumerYaml,
		{Name: "producer", Channel: "latest/stable"}: producerYaml,
	})

	s.state.Lock()
	key := fmt.Sprintf("%s/ws/consumer:plug %s/core/producer:slot", s.prj.ProjectId, s.prj.ProjectId)
	s.state.Set("conns", map[string]interface{}{
		key: map[string]interface{}{
			"interface": "test",
			"auto":      true,
			"undesired": true,
		},
	})
	s.state.Unlock()

	mgr := ifacestate.New(s.state, s.o.TaskRunner(), s.wsbackend)
	err := mgr.StartUp()
	c.Assert(err, check.IsNil)

	c.Assert(mgr.Repository().Interfaces().Connections, check.HasLen, 0)
}

func (s *interfaceManagerSuite) TestManagerRemovesNonexistingAutoConnectionss(c *check.C) {
	var consumerYaml = `
name: consumer
base: ubuntu@22.04
plugs:
 plug:
  interface: content
  attr1: value1
 otherplug:
  interface: content
`

	var producerYaml = `
name: producer
base: ubuntu@22.04
slots:
 slot:
  interface: content
  attr2: value2
`
	s.launchWorkshop(c, "ws", map[sdk.Setup]string{
		{Name: "consumer", Channel: "latest/stable"}: consumerYaml,
		{Name: "producer", Channel: "latest/stable"}: producerYaml,
	})

	s.state.Lock()
	key := fmt.Sprintf("%s/ws/consumer:plug-1 %s/core/producer:slot-1", s.prj.ProjectId, s.prj.ProjectId)

	s.state.Set("conns", map[string]interface{}{
		key: map[string]interface{}{
			"interface": "test",
			"auto":      true,
		},
	})
	s.state.Unlock()

	mgr := ifacestate.New(s.state, s.o.TaskRunner(), s.wsbackend)
	err := mgr.StartUp()
	c.Assert(err, check.IsNil)

	c.Assert(mgr.Repository().Interfaces().Connections, check.HasLen, 0)
}

func (s *interfaceManagerSuite) TestConnectionStatesAutoManual(c *check.C) {
	var isAuto, isUndesired bool = true, false
	s.testConnectionStates(c, isAuto, isUndesired, map[string]ifacestate.ConnectionState{
		"pid/ws/consumer:plug pid/ws/producer:slot": {
			Interface: "test",
			Auto:      true,
			StaticPlugAttrs: map[string]interface{}{
				"attr1": "value1",
			},
			DynamicPlugAttrs: map[string]interface{}{
				"dynamic-number": int64(7),
			},
			StaticSlotAttrs: map[string]interface{}{
				"attr2": "value2",
			},
			DynamicSlotAttrs: map[string]interface{}{
				"other-number": int64(9),
			},
		}})
}

func (s *interfaceManagerSuite) TestConnectionStatesUndesired(c *check.C) {
	var isAuto, isUndesired bool = true, true
	s.testConnectionStates(c, isAuto, isUndesired, map[string]ifacestate.ConnectionState{
		"pid/ws/consumer:plug pid/ws/producer:slot": {
			Interface: "test",
			Auto:      true,
			Undesired: true,
			StaticPlugAttrs: map[string]interface{}{
				"attr1": "value1",
			},
			DynamicPlugAttrs: map[string]interface{}{
				"dynamic-number": int64(7),
			},
			StaticSlotAttrs: map[string]interface{}{
				"attr2": "value2",
			},
			DynamicSlotAttrs: map[string]interface{}{
				"other-number": int64(9),
			},
		}})
}

func (s *interfaceManagerSuite) testConnectionStates(c *check.C, auto, undesired bool, expected map[string]ifacestate.ConnectionState) {
	consumer := sdk.MockInfo(c, `
name: consumer
base: ubuntu@22.04
plugs:
    plug:
        interface: test
        attr1: value1
`, "pid", "ws")

	producer := sdk.MockInfo(c, `
name: producer
base: ubuntu@22.04
slots:
    slot:
        interface: test
        attr2: value2
`, "pid", "ws")
	mgr := ifacestate.New(s.state, s.o.TaskRunner(), s.wsbackend)
	err := mgr.StartUp()
	c.Assert(err, check.IsNil)

	conns, err := mgr.ConnectionStates()
	c.Assert(err, check.IsNil)
	c.Check(conns, check.HasLen, 0)

	st := s.state
	st.Lock()
	sc, err := ifacestate.GetConns(st)
	c.Assert(err, check.IsNil)

	slot := producer.Slots["slot"]
	c.Assert(slot, check.NotNil)
	plug := consumer.Plugs["plug"]
	c.Assert(plug, check.NotNil)
	dynamicPlugAttrs := map[string]interface{}{"dynamic-number": 7}
	dynamicSlotAttrs := map[string]interface{}{"other-number": 9}
	// create connection in conns state
	conn := &interfaces.Connection{
		Plug: interfaces.NewConnectedPlug(plug, nil, dynamicPlugAttrs),
		Slot: interfaces.NewConnectedSlot(slot, nil, dynamicSlotAttrs),
	}
	ifacestate.UpdateConnectionInConnState(sc, conn, auto, undesired)
	ifacestate.SetConns(st, sc)
	st.Unlock()

	conns, err = mgr.ConnectionStates()
	c.Assert(err, check.IsNil)
	c.Assert(conns, check.HasLen, 1)
	c.Check(conns, check.DeepEquals, expected)
}
