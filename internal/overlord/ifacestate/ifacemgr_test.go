package ifacestate_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/overlord"
	"github.com/canonical/workshop/internal/overlord/ifacestate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
	"github.com/spf13/afero"
	"gopkg.in/check.v1"
)

type interfaceManagerSuite struct {
	testutil.BaseTest
	o         *overlord.Overlord
	state     *state.State
	se        *overlord.StateEngine
	ctx       context.Context
	wsbackend workshopbackend.WorkshopBackend
	prj       *workshopbackend.Project
}

var _ = check.Suite(&interfaceManagerSuite{})

func (s *interfaceManagerSuite) SetUpTest(c *check.C) {
	s.BaseTest.SetUpTest(c)
	var err error

	s.o = overlord.Fake()
	s.state = s.o.State()
	s.se = s.o.StateEngine()
	s.wsbackend = workshopbackend.NewFakeWorkshopBackend()
	s.ctx = context.WithValue(context.Background(), workshopbackend.ContextUser, "testuser")
	s.prj, _, err = s.wsbackend.CreateOrLoadProject(s.ctx, c.MkDir())
	c.Assert(err, check.IsNil)

	s.BaseTest.AddCleanup(sdk.MockSanitizePlugsSlots(func(snapInfo *sdk.Info) {}))
}

func (s *interfaceManagerSuite) TearDownTest(c *check.C) {
	s.BaseTest.TearDownTest(c)
}

func (s *interfaceManagerSuite) mockWorkshopWithSDKs(c *check.C, ws string, sdkYamls map[string]string) {
	ctx := context.WithValue(s.ctx, workshopbackend.ContextProjectId, s.prj.ProjectId)

	err := os.WriteFile(filepath.Join(s.prj.Path, ".workshop.ws.yaml"), []byte(`name: ws
base: ubuntu@22.04
sdks:
  consumer:
    channel: latest/stable
  producer:
    channel: latest/stable
`), 0644)
	c.Assert(err, check.IsNil)

	err = s.wsbackend.LaunchWorkshop(ctx, ws, "ubuntu@22.04")
	c.Assert(err, check.IsNil)

	wsfs, err := s.wsbackend.WorkshopFs(ctx, ws)
	c.Assert(err, check.IsNil)
	defer wsfs.Close()

	for name, sdk := range sdkYamls {
		sdkPath := filepath.Join(dirs.WorkshopSdksDir, name, "current", "meta", "sdk.yaml")
		err = afero.WriteFile(wsfs, sdkPath, []byte(sdk), 0644)
		c.Assert(err, check.IsNil)
	}
}

func (s *interfaceManagerSuite) TestManagerAddImplicitSlots(c *check.C) {
	mgr := ifacestate.New(s.state, s.o.TaskRunner(), s.wsbackend)
	err := mgr.StartUp()
	c.Assert(err, check.IsNil)

	repo := mgr.Repository()

	for _, iface := range builtin.Interfaces() {
		si := interfaces.StaticInfoOf(iface)
		if si.ImplicitOnCore {
			slots := repo.AllSlots(iface.Name())
			c.Assert(slots, check.HasLen, 1)
			c.Assert(slots[0].Sdk.Type, check.Equals, sdk.Core)
		}
	}
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
	var producerYaml = `
name: producer
base: ubuntu@22.04
type: core
slots:
 slot:
  interface: content
  attr: slot-value
`

	s.mockWorkshopWithSDKs(c, "ws", map[string]string{
		"consumer": consumerYaml,
		"producer": producerYaml,
	})

	s.state.Lock()
	key := fmt.Sprintf("%s:ws:consumer:plug %s:ws:producer:slot", s.prj.ProjectId, s.prj.ProjectId)
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
		SlotRef: interfaces.SlotRef{ProjectId: s.prj.ProjectId, Workshop: "ws", Sdk: "producer", Name: "slot"}}
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
	s.mockWorkshopWithSDKs(c, "ws", map[string]string{
		"consumer": consumerYaml,
		"producer": producerYaml,
	})

	s.state.Lock()
	key := fmt.Sprintf("%s:ws:consumer:plug %s:core:producer:slot", s.prj.ProjectId, s.prj.ProjectId)
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
	s.mockWorkshopWithSDKs(c, "ws", map[string]string{
		"consumer": consumerYaml,
		"producer": producerYaml,
	})

	s.state.Lock()
	key := fmt.Sprintf("%s:ws:consumer:plug-1 %s:core:producer:slot-1", s.prj.ProjectId, s.prj.ProjectId)

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
