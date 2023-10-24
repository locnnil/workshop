package ifacestate_test

import (
	"context"
	"os"
	"path/filepath"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/overlord"
	"github.com/canonical/workshop/internal/overlord/ifacestate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workspacebackend"
	"github.com/spf13/afero"
	"gopkg.in/check.v1"
)

type interfaceManagerSuite struct {
	testutil.BaseTest
	o         *overlord.Overlord
	state     *state.State
	se        *overlord.StateEngine
	ctx       context.Context
	wsbackend workspacebackend.WorkspaceBackend
	prj       *workspacebackend.Project
}

var _ = check.Suite(&interfaceManagerSuite{})

func (s *interfaceManagerSuite) SetUpTest(c *check.C) {
	s.BaseTest.SetUpTest(c)
	var err error

	s.o = overlord.Fake()
	s.state = s.o.State()
	s.se = s.o.StateEngine()
	s.wsbackend = workspacebackend.NewFakeWorkspaceBackend()
	s.ctx = context.WithValue(context.Background(), workspacebackend.ContextUser, "testuser")
	s.prj, _, err = s.wsbackend.CreateOrLoadProject(s.ctx, c.MkDir())
	c.Assert(err, check.IsNil)

	s.BaseTest.AddCleanup(sdk.MockSanitizePlugsSlots(func(snapInfo *sdk.Info) {}))
}

func (s *interfaceManagerSuite) TearDownTest(c *check.C) {
	s.BaseTest.TearDownTest(c)
}

func (s *interfaceManagerSuite) mockWorkspaceWithSDKs(c *check.C, ws string, sdkYamls map[string]string) {
	ctx := context.WithValue(s.ctx, workspacebackend.ContextProjectId, s.prj.ProjectId)

	err := os.WriteFile(filepath.Join(s.prj.Path, ".workspace.ws.yaml"), []byte(`name: ws
base: ubuntu@22.04
sdks:
  consumer:
    channel: latest/stable
  producer:
    channel: latest/stable
`), 0644)
	c.Assert(err, check.IsNil)

	err = s.wsbackend.LaunchWorkspace(ctx, ws, "ubuntu@22.04")
	c.Assert(err, check.IsNil)

	wsfs, err := s.wsbackend.GetWorkspaceFs(ctx, ws)
	c.Assert(err, check.IsNil)
	defer wsfs.Close()

	for name, sdk := range sdkYamls {
		sdkPath := filepath.Join(dirs.WorkspaceSdksDir, name, "current", "meta", "sdk.yaml")
		err = afero.WriteFile(wsfs, sdkPath, []byte(sdk), 0644)
		c.Assert(err, check.IsNil)
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

	s.mockWorkspaceWithSDKs(c, "ws", map[string]string{
		"consumer": consumerYaml,
		"producer": producerYaml,
	})

	s.state.Lock()
	s.state.Set("conns", map[string]interface{}{
		"ws:consumer:plug ws:producer:slot": map[string]interface{}{
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
	cref := &interfaces.ConnRef{PlugRef: interfaces.PlugRef{Workspace: "ws", Sdk: "consumer", Name: "plug"}, SlotRef: interfaces.SlotRef{Workspace: "ws", Sdk: "producer", Name: "slot"}}
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
	s.mockWorkspaceWithSDKs(c, "ws", map[string]string{
		"consumer": consumerYaml,
		"producer": producerYaml,
	})

	s.state.Lock()
	s.state.Set("conns", map[string]interface{}{
		"ws:consumer:plug core:producer:slot": map[string]interface{}{
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
	s.mockWorkspaceWithSDKs(c, "ws", map[string]string{
		"consumer": consumerYaml,
		"producer": producerYaml,
	})

	s.state.Lock()
	s.state.Set("conns", map[string]interface{}{
		"ws:consumer:plug-1 core:producer:slot-1": map[string]interface{}{
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
