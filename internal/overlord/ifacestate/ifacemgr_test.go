package ifacestate_test

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/interfaces/ifacetest"
	"github.com/canonical/workshop/internal/overlord"
	"github.com/canonical/workshop/internal/overlord/ifacestate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
	"github.com/spf13/afero"
	"golang.org/x/exp/maps"
	"gopkg.in/check.v1"
)

type interfaceManagerSuite struct {
	testutil.BaseTest
	o          *overlord.Overlord
	state      *state.State
	se         *overlord.StateEngine
	runner     *state.TaskRunner
	ctx        context.Context
	wsbackend  workshopbackend.WorkshopBackend
	prj        *workshopbackend.Project
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

	s.restoreProjectId = testutil.FakeFunc(func() (string, error) { return "42424242", nil }, &workshopbackend.NewProjectId)

	s.wsbackend = workshopbackend.NewFakeWorkshopBackend()
	s.ctx = context.WithValue(context.Background(), workshopbackend.ContextUser, "testuser")
	s.prj, _, err = s.wsbackend.CreateOrLoadProject(s.ctx, c.MkDir())
	c.Assert(err, check.IsNil)
	s.ctx = context.WithValue(s.ctx, workshopbackend.ContextProjectId, s.prj.ProjectId)

	s.BaseTest.AddCleanup(sdk.MockSanitizePlugsSlots(func(sdkInfo *sdk.Info) {}))
}

func (s *interfaceManagerSuite) TearDownTest(c *check.C) {
	s.restoreProjectId()
	s.BaseTest.TearDownTest(c)
}

func (s *interfaceManagerSuite) writeSDKMetaFile(c *check.C, fs workshopbackend.WorkshopFs, name, yaml string) {
	sdkPath := filepath.Join(dirs.WorkshopSdksDir, name, "current", "meta", "sdk.yaml")
	err := afero.WriteFile(fs, sdkPath, []byte(yaml), 0644)
	c.Assert(err, check.IsNil)
}

func (s *interfaceManagerSuite) launchWorkshopWithSDKs(c *check.C, ws string, sdkYamls map[sdk.Setup]string) {
	ctx := context.WithValue(s.ctx, workshopbackend.ContextProjectId, s.prj.ProjectId)

	t, err := template.New("workshop").Parse(fmt.Sprintf(workshopTemplate, ws))
	c.Assert(err, check.IsNil)

	var workshopFile = bytes.NewBuffer([]byte{})
	t.Execute(workshopFile, maps.Keys(sdkYamls))

	err = os.WriteFile(filepath.Join(s.prj.Path, fmt.Sprintf(".workshop.%s.yaml", ws)), workshopFile.Bytes(), 0644)
	c.Assert(err, check.IsNil)

	err = s.wsbackend.LaunchWorkshop(ctx, ws, "ubuntu@22.04")
	c.Assert(err, check.IsNil)

	wsfs, err := s.wsbackend.WorkshopFs(ctx, ws)
	c.Assert(err, check.IsNil)
	defer wsfs.Close()

	for sdk, yaml := range sdkYamls {
		s.writeSDKMetaFile(c, wsfs, sdk.Name, yaml)
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

	s.launchWorkshopWithSDKs(c, "ws", map[sdk.Setup]string{
		{Name: "consumer", Channel: "latest/stable"}: consumerYaml,
		{Name: "producer", Channel: "latest/stable"}: producerYaml,
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
	s.launchWorkshopWithSDKs(c, "ws", map[sdk.Setup]string{
		{Name: "consumer", Channel: "latest/stable"}: consumerYaml,
		{Name: "producer", Channel: "latest/stable"}: producerYaml,
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
	s.launchWorkshopWithSDKs(c, "ws", map[sdk.Setup]string{
		{Name: "consumer", Channel: "latest/stable"}: consumerYaml,
		{Name: "producer", Channel: "latest/stable"}: producerYaml,
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
