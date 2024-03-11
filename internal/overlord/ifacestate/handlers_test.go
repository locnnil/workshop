package ifacestate_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/overlord/ifacestate"
	"github.com/canonical/workshop/internal/overlord/ifacestate/schema"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshopbackend"
	"gopkg.in/check.v1"
	"gopkg.in/tomb.v2"
)

type interfaceHandlersSuite struct {
	interfaceManagerSuite
	mgr                      *ifacestate.InterfaceManager
	restoreInterface         func()
	restoreSecurtityBackends func()
	setup                    sync.Once
}

var _ = check.Suite(&interfaceHandlersSuite{})

func fakeHandler(task *state.Task, _ *tomb.Tomb) error {
	return nil
}

var producer = `name: producer
base: ubuntu@22.04
slots:
  slot:
    interface: mock-network`
var psetup = sdk.Setup{Name: "producer", Channel: "latest/stable"}

var consumer = `name: consumer
base: ubuntu@22.04
plugs:
  plug:
    interface: mock-network
    attribute: one
`

var consumerManyPlugs = `name: consumer
base: ubuntu@22.04
plugs:
  plug:
    interface: mock-network
    attribute: one
  plug2:
    interface: mock-network
    attribute: two
  plug3:
    interface: mock-network
    attribute: three
`

var csetup = sdk.Setup{Name: "consumer", Channel: "latest/stable"}

var consumerNoPlugs = `name: consumer
base: ubuntu@22.04
`

func (s *interfaceHandlersSuite) SetUpTest(c *check.C) {
	s.interfaceManagerSuite.SetUpTest(c)
	s.restoreInterface = builtin.MockInterface(simpleIface{name: "mock-network"})

	s.mgr = ifacestate.New(s.state, s.runner, s.wsbackend)
	c.Assert(s.mgr, check.NotNil)

	s.runner.AddHandler("fake-task", fakeHandler, nil)

	// error-provoking task handler
	erroringHandler := func(task *state.Task, _ *tomb.Tomb) error {
		return errors.New("error-trigger task")
	}
	s.runner.AddHandler("error-trigger", erroringHandler, nil)
	s.restoreSecurtityBackends = ifacestate.MockSecurityBackends([]interfaces.SecurityBackend{s.secBackend})

	s.o.AddManager(s.mgr)
	s.o.AddManager(s.runner)
	err := s.o.StartUp()
	c.Assert(err, check.IsNil)
}

func (s *interfaceHandlersSuite) TearDownTest(c *check.C) {
	s.restoreInterface()
	s.restoreSecurtityBackends()
}

func setWorkshopProject(w string, p *workshopbackend.Project, tasks ...*state.Task) {
	for _, i := range tasks {
		i.Set("workshop", w)
		i.Set("project", *p)
	}
}

type simpleIface struct {
	name string
}

func (si simpleIface) Name() string                                            { return si.name }
func (si simpleIface) AutoConnect(plug *sdk.PlugInfo, slot *sdk.SlotInfo) bool { return true }

func (s *interfaceHandlersSuite) TestAutoconnectPlugSlotPairSuccess(c *check.C) {
	// Setup
	// Create an already installed workshop with a candidate SDK/slot
	repo := s.mgr.Repository()
	s.launchWorkshopWithSDKs(c, "ws-producer", map[sdk.Setup]string{psetup: producer})
	c.Assert(repo.AddSdk(sdk.MockInfo(c, producer, s.prj.ProjectId, "ws-producer", psetup)), check.IsNil)

	// Launch another workshop with a candidate plug
	s.launchWorkshopWithSDKs(c, "ws", map[sdk.Setup]string{csetup: consumer})

	s.secBackend.SetupCallback = func(context context.Context, sdkInfo *sdk.Info, repo *interfaces.Repository) error {
		connections, err := repo.Connected(s.prj.ProjectId, "ws", "consumer", "plug")
		c.Assert(err, check.IsNil)
		connection, err := repo.Connection(connections[0])
		c.Assert(err, check.IsNil)
		c.Assert(connection.Plug.SetAttr("test-dynamic-attr", "new-dynamic-value"), check.IsNil)
		return nil
	}
	defer func() { s.secBackend.SetupCallback = nil }()

	// Execute
	s.state.Lock()
	chg := s.state.NewChange("sample", "...")
	t1 := s.state.NewTask("auto-connect", "...")
	t1.Set("sdk", "consumer")
	setWorkshopProject("ws", s.prj, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	s.state.Unlock()

	s.o.Settle(5 * time.Second)

	s.state.Lock()
	defer s.state.Unlock()
	c.Check(chg.Err(), check.IsNil)

	// Validate
	c.Assert(t1.Status(), check.Equals, state.DoneStatus)
	ref, err := repo.Connected(s.prj.ProjectId, "ws", "consumer", "plug")
	c.Assert(ref, check.HasLen, 1)
	c.Assert(err, check.IsNil)

	ref, err = repo.Connected(s.prj.ProjectId, "ws-producer", "producer", "slot")
	c.Assert(ref, check.HasLen, 1)
	c.Assert(err, check.IsNil)

	var conns map[string]interface{}
	s.state.Get("conns", &conns)
	c.Assert(conns, check.DeepEquals, map[string]interface{}{
		"42424242:ws:consumer:plug 42424242:ws-producer:producer:slot": map[string]interface{}{
			"interface":    "mock-network",
			"auto":         true,
			"plug-static":  map[string]interface{}{"attribute": "one"},
			"plug-dynamic": map[string]interface{}{"test-dynamic-attr": "new-dynamic-value"},
		},
	})

	// ensure that backend profiles were set for both SDKs
	c.Assert(s.secBackend.SetupCalls, check.HasLen, 2)
	c.Assert(s.secBackend.RemoveCalls, check.HasLen, 0)
}

func (s *interfaceHandlersSuite) TestAutoconnectBackendEnsureDisconnectsIfBackendSetupFails(c *check.C) {
	// Setup
	// Create an already installed workshop with a candidate SDK/slot
	repo := s.mgr.Repository()
	s.launchWorkshopWithSDKs(c, "ws-producer", map[sdk.Setup]string{psetup: producer})
	c.Assert(repo.AddSdk(sdk.MockInfo(c, producer, s.prj.ProjectId, "ws-producer", psetup)), check.IsNil)

	s.launchWorkshopWithSDKs(c, "ws-consumer", map[sdk.Setup]string{csetup: consumerManyPlugs})

	s.secBackend.SetupCallback = func(context context.Context, sdkInfo *sdk.Info, repo *interfaces.Repository) error {
		return errors.New("cannot finish backend setup")
	}
	defer func() { s.secBackend.SetupCallback = nil }()

	// Execute
	s.state.Lock()
	chg := s.state.NewChange("sample", "...")
	t1 := s.state.NewTask("auto-connect", "...")
	t1.Set("sdk", "consumer")
	setWorkshopProject("ws-consumer", s.prj, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	s.state.Unlock()

	s.o.Settle(5 * time.Second)

	s.state.Lock()
	defer s.state.Unlock()
	c.Check(chg.Err(), check.NotNil)

	// Validate
	for _, plug := range []string{"plug", "plug2", "plug3"} {
		ref, err := repo.Connected(s.prj.ProjectId, "ws-consumer", "consumer", plug)
		c.Assert(ref, check.HasLen, 0)
		c.Assert(err, check.IsNil)
	}

	ref, err := repo.Connected(s.prj.ProjectId, "ws-producer", "producer", "slot")
	c.Assert(ref, check.HasLen, 0)
	c.Assert(err, check.IsNil)

	var conns map[string]interface{}
	s.state.Get("conns", &conns)
	c.Assert(conns, check.HasLen, 0)
}

func (s *interfaceHandlersSuite) TestAutoconnectUndoAllBackendSetupsIfEitherFailed(c *check.C) {
	// Setup
	// Create an already installed workshop with a candidate SDK/slot
	repo := s.mgr.Repository()
	s.secBackend.SetupCallback = func(context context.Context, sdkInfo *sdk.Info, repo *interfaces.Repository) error {
		if len(s.secBackend.SetupCalls) <= 1 {
			return nil
		}
		return errors.New("cannot finish backend setup")
	}
	defer func() { s.secBackend.SetupCallback = nil }()

	s.launchWorkshopWithSDKs(c, "ws-producer", map[sdk.Setup]string{psetup: producer})
	c.Assert(repo.AddSdk(sdk.MockInfo(c, producer, s.prj.ProjectId, "ws-producer", psetup)), check.IsNil)

	s.launchWorkshopWithSDKs(c, "ws-consumer", map[sdk.Setup]string{csetup: consumerManyPlugs})

	// Execute
	s.state.Lock()
	chg := s.state.NewChange("sample", "...")
	t1 := s.state.NewTask("auto-connect", "...")
	t1.Set("sdk", "consumer")
	setWorkshopProject("ws-consumer", s.prj, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	s.state.Unlock()

	s.o.Settle(5 * time.Second)

	s.state.Lock()
	defer s.state.Unlock()
	c.Check(chg.Err(), check.NotNil)

	// Validate
	c.Assert(s.secBackend.RemoveCalls, check.HasLen, 1)
}

func (s *interfaceHandlersSuite) TestAutoconnectNoConnections(c *check.C) {
	// Setup

	repo := s.mgr.Repository()
	// Launch another workshop with a candidate plug
	s.launchWorkshopWithSDKs(c, "ws", map[sdk.Setup]string{csetup: consumer})

	// Execute
	s.state.Lock()
	chg := s.state.NewChange("sample", "...")
	t1 := s.state.NewTask("auto-connect", "...")
	t1.Set("sdk", "consumer")
	t2 := s.state.NewTask("error-trigger", "...")
	setWorkshopProject("ws", s.prj, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	chg.AddTask(t2)
	s.state.Unlock()

	s.o.Settle(5 * time.Second)

	s.state.Lock()
	defer s.state.Unlock()

	// Validate
	c.Assert(t1.Status(), check.Equals, state.UndoneStatus)
	_, err := repo.Connected(s.prj.ProjectId, "ws", "consumer", "plug")
	c.Assert(err, check.ErrorMatches, "sdk \"consumer\" has no plug or slot named \"plug\"")

	var conns map[string]interface{}
	s.state.Get("conns", &conns)
	c.Assert(conns, check.DeepEquals, map[string]interface{}{})

	// ensure that backend profiles were set for both SDKs
	c.Assert(s.secBackend.SetupCalls, check.HasLen, 0)
	c.Assert(s.secBackend.RemoveCalls, check.HasLen, 0)
}

func (s *interfaceHandlersSuite) TestAutoconnectUndoSuccess(c *check.C) {
	// Setup
	// Create an already installed workshop with a candidate SDK/slot
	repo := s.mgr.Repository()
	s.launchWorkshopWithSDKs(c, "ws-producer", map[sdk.Setup]string{psetup: producer})
	c.Assert(repo.AddSdk(sdk.MockInfo(c, producer, s.prj.ProjectId, "ws-producer", psetup)), check.IsNil)

	// Launch another workshop with a candidate plug
	s.launchWorkshopWithSDKs(c, "ws", map[sdk.Setup]string{csetup: consumer})

	s.state.Lock()
	chg := s.state.NewChange("sample", "...")
	t1 := s.state.NewTask("auto-connect", "...")
	t1.Set("sdk", "consumer")
	t2 := s.state.NewTask("error-trigger", "...")
	setWorkshopProject("ws", s.prj, t1)
	setWorkshopProject("ws", s.prj, t2)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	chg.AddTask(t2)
	s.state.Unlock()

	s.o.Settle(5 * time.Second)

	s.state.Lock()
	defer s.state.Unlock()

	c.Assert(t1.Status(), check.Equals, state.UndoneStatus)
	ref, err := repo.Connected(s.prj.ProjectId, "ws", "consumer", "plug")
	c.Assert(ref, check.HasLen, 0)
	c.Assert(err, check.ErrorMatches, "sdk \"consumer\" has no plug or slot named \"plug\"")

	ref, err = repo.Connected(s.prj.ProjectId, "ws-producer", "producer", "slot")
	c.Assert(ref, check.HasLen, 0)
	c.Assert(err, check.IsNil)

	// ensure that backend profiles were set for both SDKs in Do
	// and unset for producer in Undo
	c.Assert(s.secBackend.SetupCalls, check.HasLen, 2+1)
	// ensure that the backend profile was removed for consumer in Undo
	c.Assert(s.secBackend.RemoveCalls, check.HasLen, 1)
}

func (s *interfaceHandlersSuite) TestAutoconnectRemovesNonExistingConnections(c *check.C) {
	// Setup
	// Create an already installed workshop with a candidate SDK/slot
	repo := s.mgr.Repository()
	s.launchWorkshopWithSDKs(c, "ws-consumer", map[sdk.Setup]string{csetup: consumer})
	c.Assert(repo.AddSdk(sdk.MockInfo(c, producer, s.prj.ProjectId, "ws-consumer", psetup)), check.IsNil)

	s.state.Lock()
	chg := s.state.NewChange("sample", "...")
	t1 := s.state.NewTask("auto-connect", "...")
	t1.Set("sdk", "consumer")
	setWorkshopProject("ws-consumer", s.prj, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	s.state.Unlock()

	s.o.Settle(5 * time.Second)

	s.state.Lock()
	c.Assert(t1.Status(), check.Equals, state.DoneStatus)

	// simulate refresh, which will install the consumer SDK again
	chg = s.state.NewChange("refresh", "...")
	t2 := s.state.NewTask("auto-connect", "...")
	t2.Set("sdk", "consumer")
	chg.AddTask(t2)
	chg.Set("user", "testuser")
	setWorkshopProject("ws-consumer", s.prj, t2)

	// overwrite the SDK meta file so the auto-connect task will
	// now be installing an SDK with no plugs
	// for simplicity of the test it does so into the same workshop
	// but in practice this scenario is possible when the workshop is refreshed
	// and some of the SDKs have their plugs and slots changed.
	fs, err := s.wsbackend.WorkshopFs(s.ctx, "ws-consumer")
	c.Assert(err, check.IsNil)
	s.writeSDKMetaFile(c, fs, csetup.Name, consumerNoPlugs)
	s.state.Unlock()

	s.o.Settle(5 * time.Second)

	s.state.Lock()
	defer s.state.Unlock()

	// Validate
	// Confirm that the previously existing connection of the mock-network
	// interface has now been removed from the connections list.
	c.Assert(t2.Status(), check.Equals, state.DoneStatus)

	var conns map[string]interface{}
	s.state.Get("conns", &conns)
	c.Assert(conns, check.DeepEquals, map[string]interface{}{})

	// Also confirm that the SDK profile does not exist any more as
	// the plug has been removed.
	c.Assert(s.secBackend.SetupCalls, check.HasLen, 2+2)
	c.Assert(s.secBackend.RemoveCalls, check.HasLen, 0)
}

func (s *interfaceHandlersSuite) TestAutoconnectReconnectsExistingConnections(c *check.C) {
	// Setup
	// Create an already installed workshop with a candidate SDK/slot
	repo := s.mgr.Repository()
	s.launchWorkshopWithSDKs(c, "ws-producer", map[sdk.Setup]string{psetup: producer})
	c.Assert(repo.AddSdk(sdk.MockInfo(c, producer, s.prj.ProjectId, "ws-producer", psetup)), check.IsNil)

	// Launch another workshop with a candidate plug
	s.launchWorkshopWithSDKs(c, "ws", map[sdk.Setup]string{csetup: consumer})

	s.state.Lock()
	chg := s.state.NewChange("sample", "...")
	t1 := s.state.NewTask("auto-connect", "...")
	t1.Set("sdk", "consumer")
	setWorkshopProject("ws", s.prj, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	s.state.Unlock()

	s.o.Settle(5 * time.Second)

	s.state.Lock()
	// simulate refresh, which will install the consumer SDK again
	// hence the existing autoconnectios must be restored
	chg = s.state.NewChange("refresh", "...")
	t2 := s.state.NewTask("auto-connect", "...")
	t2.Set("sdk", "consumer")
	chg.AddTask(t2)
	chg.Set("user", "testuser")
	setWorkshopProject("ws", s.prj, t2)

	c.Assert(s.wsbackend.RemoveWorkshop(s.ctx, "ws"), check.IsNil)
	s.launchWorkshopWithSDKs(c, "ws", map[sdk.Setup]string{csetup: consumer})
	s.state.Unlock()

	s.o.Settle(5 * time.Second)

	s.state.Lock()
	defer s.state.Unlock()

	// Validate
	c.Assert(t1.Status(), check.Equals, state.DoneStatus)
	c.Assert(t2.Status(), check.Equals, state.DoneStatus)

	var conns map[string]interface{}
	s.state.Get("conns", &conns)
	c.Assert(conns, check.DeepEquals, map[string]interface{}{
		"42424242:ws:consumer:plug 42424242:ws-producer:producer:slot": map[string]interface{}{
			"interface":   "mock-network",
			"auto":        true,
			"plug-static": map[string]interface{}{"attribute": "one"},
		},
	})

	// ensure that backend profiles were set for both SDKs
	// in the first and the second runs. First, when consumer
	// was installed for the first time and, second, when consumer
	// was refreshed but did not have any changes and had its auto
	// connections restored
	c.Assert(s.secBackend.SetupCalls, check.HasLen, 2+2)
	c.Assert(s.secBackend.RemoveCalls, check.HasLen, 0)
}

func (s *interfaceHandlersSuite) TestAutoconnectFailInstallPolicyCheck(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	var sdkYaml = `
name: consumer
base: ubuntu@22.04
slots:
  slot:
    interface: content
`
	s.launchWorkshopWithSDKs(c, "ws", map[sdk.Setup]string{csetup: sdkYaml})

	t1 := s.state.NewTask("auto-connect", "test")
	t1.Set("sdk", "consumer")

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.prj, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)

	s.state.Unlock()
	s.o.Settle(5 * time.Second)

	s.state.Lock()

	c.Assert(t1.Status(), check.Equals, state.ErrorStatus)
	c.Assert(t1.Log()[0], check.Matches, ".*installation not allowed.*")
}

func (s *interfaceHandlersSuite) TestAutoconnectRemountedPlugs(c *check.C) {
	// Setup
	// Create an already installed workshop with a candidate SDK/slot
	repo := s.mgr.Repository()
	s.launchWorkshopWithSDKs(c, "ws-producer", map[sdk.Setup]string{psetup: producer})
	c.Assert(repo.AddSdk(sdk.MockInfo(c, producer, s.prj.ProjectId, "ws-producer", psetup)), check.IsNil)

	// Launch another workshop with a candidate plug
	s.launchWorkshopWithSDKs(c, "ws", map[sdk.Setup]string{csetup: consumer})

	// Execute
	s.state.Lock()
	chg := s.state.NewChange("sample", "...")
	t := s.state.NewTask("disconnect", "...")
	// see doAutoConnect and doDisconnect handlers for details
	t.Set("plugs-to-remount", map[string]map[string]interface{}{
		"42424242:ws:consumer:plug 42424242:ws-producer:producer:slot": {"source": "/old/source"},
	})
	t.Set("sdk", "consumer")
	t.SetStatus(state.DoneStatus)
	t1 := s.state.NewTask("auto-connect", "...")
	t1.Set("sdk", "consumer")
	setWorkshopProject("ws", s.prj, t, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t)
	chg.AddTask(t1)
	s.state.Unlock()

	s.o.Settle(5 * time.Second)

	s.state.Lock()
	defer s.state.Unlock()
	c.Check(chg.Err(), check.IsNil)

	// Validate
	c.Assert(t1.Status(), check.Equals, state.DoneStatus)
	var conns map[string]interface{}
	s.state.Get("conns", &conns)
	c.Assert(conns, check.DeepEquals, map[string]interface{}{
		"42424242:ws:consumer:plug 42424242:ws-producer:producer:slot": map[string]interface{}{
			"interface":    "mock-network",
			"auto":         true,
			"plug-static":  map[string]interface{}{"attribute": "one"},
			"plug-dynamic": map[string]interface{}{"source": "/old/source"},
		},
	})

	// ensure that backend profiles were set for both SDKs
	c.Assert(s.secBackend.SetupCalls, check.HasLen, 2)
	c.Assert(s.secBackend.RemoveCalls, check.HasLen, 0)
}

func (s *interfaceHandlersSuite) newRemountChange(newSource string) *state.Change {
	s.state.Lock()
	t1 := s.state.NewTask("auto-connect", "test")
	t1.Set("sdk", "consumer")
	t2 := s.state.NewTask("remount", "remount")
	t2.Set("source", newSource)
	t2.Set("plug", interfaces.PlugRef{ProjectId: s.prj.ProjectId, Workshop: "ws-consumer", Sdk: "consumer", Name: "plug"})
	t2.WaitFor(t1)
	setWorkshopProject("ws-consumer", s.prj, t1, t2)

	// Prevent undoing if the remount fails, so we don't have plugs disconnected
	// as we want to test remount in isolation, as if it was a single change.
	connLane := s.state.NewLane()
	t1.JoinLane(connLane)

	chg := s.state.NewChange("sample", "...")
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	chg.AddTask(t2)
	s.state.Unlock()
	return chg
}

func (s *interfaceHandlersSuite) setupPlugConnectionAttribute(c *check.C, repo *interfaces.Repository, oldSource string) {
	connections, err := repo.Connected(s.prj.ProjectId, "ws-consumer", "consumer", "plug")
	c.Assert(err, check.IsNil)
	c.Assert(connections, check.HasLen, 1)

	connRef := connections[0]
	connection, err := repo.Connection(connRef)
	c.Assert(err, check.IsNil)
	c.Assert(connection.Plug.SetAttr("source", oldSource), check.IsNil)
}

func (s *interfaceHandlersSuite) launchRemountWorkshop(c *check.C) {
	// Note: we set the source attribute for the plug in these tests, however,
	// it is a dynamic attribute that will be defined when we install an SDK
	// into a workshop as the default path depends on the username
	var sdkYaml = `
name: consumer
base: ubuntu@22.04
plugs:
    plug:
        interface: content
        target: /home/workshop
`
	s.launchWorkshopWithSDKs(c, "ws-consumer", map[sdk.Setup]string{csetup: sdkYaml})
	c.Assert(s.mgr.Repository().AddSdk(sdk.MockInfo(c, sdkYaml, s.prj.ProjectId, "ws-consumer", csetup)), check.IsNil)
}

func (s *interfaceHandlersSuite) TestRemountSuccessDestExistsAndEmpty(c *check.C) {
	// Setup
	oldSource := c.MkDir()
	newSource := c.MkDir()
	_, err := os.Create(filepath.Join(oldSource, "tempfile"))
	c.Check(err, check.IsNil)

	s.launchRemountWorkshop(c)
	change := s.newRemountChange(newSource)

	var setup sync.Once
	s.secBackend.SetupCallback = func(context context.Context, sdkInfo *sdk.Info, repo *interfaces.Repository) error {
		// Set the plug's source attribute to emulate an existing connection as
		// the remount handler expects that a plug IS connected and HAS a
		// "source" attribute
		setup.Do(func() {
			s.setupPlugConnectionAttribute(c, repo, oldSource)
		})
		return nil
	}
	defer func() { s.secBackend.SetupCallback = nil }()

	// Execute
	s.o.Settle(5 * time.Second)

	// Validate
	s.state.Lock()
	defer s.state.Unlock()
	c.Check(change.Err(), check.IsNil)
	c.Assert(change.Status(), check.Equals, state.DoneStatus)

	repo := s.mgr.Repository()
	ref, err := repo.Connected(s.prj.ProjectId, "ws-consumer", "consumer", "plug")
	c.Assert(ref, check.HasLen, 1)
	c.Assert(err, check.IsNil)

	connection, err := repo.Connection(ref[0])
	c.Assert(err, check.IsNil)
	var remountSource string
	c.Assert(connection.Plug.Attr("source", &remountSource), check.IsNil)
	c.Assert(remountSource, check.Equals, newSource)

	c.Assert(osutil.FileExists(oldSource), check.Equals, false)
	// 2 calls for the autoconnect, one call for the remount
	c.Assert(s.secBackend.SetupCalls, check.HasLen, 2+1)
	c.Assert(s.secBackend.SetupCalls[2].SdkInfo.Name, check.Equals, "consumer")
	c.Assert(s.secBackend.SetupCalls[2].SdkInfo.Workshop, check.Equals, "ws-consumer")

	// ensure the global conns state was updated correctly
	conns, err := ifacestate.GetConns(s.state)
	c.Assert(err, check.IsNil)
	c.Assert(conns[ref[0].ID()], check.DeepEquals, &schema.ConnState{
		Auto:             true,
		Interface:        "content",
		Undesired:        false,
		StaticPlugAttrs:  map[string]interface{}{"target": "/home/workshop"},
		DynamicPlugAttrs: map[string]interface{}{"source": newSource},
		StaticSlotAttrs:  map[string]interface{}{},
		DynamicSlotAttrs: map[string]interface{}{}})
	c.Assert(conns, check.HasLen, 1)
}

func (s *interfaceHandlersSuite) TestRemountSuccessIfNewSourceDoesNotExist(c *check.C) {
	// Setup
	oldSource := c.MkDir()
	newSource := filepath.Join(c.MkDir(), "new")
	s.launchRemountWorkshop(c)
	change := s.newRemountChange(newSource)

	s.secBackend.SetupCallback = func(context context.Context, sdkInfo *sdk.Info, repo *interfaces.Repository) error {
		// Set the plug's source attribute to emulate an existing connection as
		// the remount handler expects that a plug IS connected and HAS a
		// "source" attribute
		s.setup.Do(func() {
			s.setupPlugConnectionAttribute(c, repo, oldSource)
		})
		return nil
	}
	defer func() { s.secBackend.SetupCallback = nil }()

	// Execute
	s.o.Settle(5 * time.Second)

	// Validate
	s.state.Lock()
	defer s.state.Unlock()
	c.Check(change.Err(), check.IsNil)
	c.Assert(change.Status(), check.Equals, state.DoneStatus)

	repo := s.mgr.Repository()
	ref, err := repo.Connected(s.prj.ProjectId, "ws-consumer", "consumer", "plug")
	c.Assert(ref, check.HasLen, 1)
	c.Assert(err, check.IsNil)

	connection, err := repo.Connection(ref[0])
	c.Assert(err, check.IsNil)
	var remountSource string
	c.Assert(connection.Plug.Attr("source", &remountSource), check.IsNil)
	c.Assert(remountSource, check.Equals, newSource)

	c.Assert(osutil.FileExists(oldSource), check.Equals, false)
	// 2 calls for the autoconnect, one call for the remount
	c.Assert(s.secBackend.SetupCalls, check.HasLen, 2+1)
	c.Assert(s.secBackend.SetupCalls[2].SdkInfo.Name, check.Equals, "consumer")
	c.Assert(s.secBackend.SetupCalls[2].SdkInfo.Workshop, check.Equals, "ws-consumer")

	// ensure the global conns state was updated correctly
	conns, err := ifacestate.GetConns(s.state)
	c.Assert(err, check.IsNil)
	c.Assert(conns[ref[0].ID()], check.DeepEquals, &schema.ConnState{
		Auto:             true,
		Interface:        "content",
		Undesired:        false,
		StaticPlugAttrs:  map[string]interface{}{"target": "/home/workshop"},
		DynamicPlugAttrs: map[string]interface{}{"source": newSource},
		StaticSlotAttrs:  map[string]interface{}{},
		DynamicSlotAttrs: map[string]interface{}{}})
	c.Assert(conns, check.HasLen, 1)
}

func (s *interfaceHandlersSuite) TestRemountRenameFails(c *check.C) {
	// Setup
	oldSource := c.MkDir()
	newSource := c.MkDir()
	_, err := os.Create(filepath.Join(newSource, "tempfile"))
	c.Check(err, check.IsNil)
	s.launchRemountWorkshop(c)
	change := s.newRemountChange(newSource)

	var setup sync.Once
	s.secBackend.SetupCallback = func(context context.Context, sdkInfo *sdk.Info, repo *interfaces.Repository) error {
		// Set the plug's source attribute to emulate an existing connection as
		// the remount handler expects that a plug IS connected and HAS a
		// "source" attribute
		setup.Do(func() {
			s.setupPlugConnectionAttribute(c, repo, oldSource)
		})
		return nil
	}
	defer func() { s.secBackend.SetupCallback = nil }()

	// Execute
	s.o.Settle(5 * time.Second)

	// Validate
	s.state.Lock()
	defer s.state.Unlock()
	c.Check(change.Err(), check.ErrorMatches, "(?s).*\\(directory not empty\\)")
	c.Assert(change.Status(), check.Equals, state.ErrorStatus)

	repo := s.mgr.Repository()
	ref, err := repo.Connected(s.prj.ProjectId, "ws-consumer", "consumer", "plug")
	c.Assert(ref, check.HasLen, 1)
	c.Assert(err, check.IsNil)

	connection, err := repo.Connection(ref[0])
	c.Assert(err, check.IsNil)
	var src string
	c.Assert(connection.Plug.Attr("source", &src), check.IsNil)
	c.Assert(src, check.Equals, oldSource)

	c.Assert(osutil.FileExists(oldSource), check.Equals, true)
	c.Assert(osutil.FileExists(newSource), check.Equals, true)
	// 2 calls for the autoconnect, no calls for the remount
	c.Assert(s.secBackend.SetupCalls, check.HasLen, 2)
}

func (s *interfaceHandlersSuite) TestRemountInterfaceBackendSetupFails(c *check.C) {
	// Setup
	oldSource := c.MkDir()
	newSource := c.MkDir()
	s.launchRemountWorkshop(c)
	change := s.newRemountChange(newSource)

	var setup sync.Once
	s.secBackend.SetupCallback = func(context context.Context, sdkInfo *sdk.Info, repo *interfaces.Repository) error {
		// Set the plug's source attribute to emulate an existing connection as
		// the remount handler expects that a plug IS connected and HAS a
		// "source" attribute
		setup.Do(func() {
			s.setupPlugConnectionAttribute(c, repo, oldSource)
		})
		// Emulate the case when remount could not update the LXD profile for
		// the SDK (the first two calls come from the auto-connect task)
		if len(s.secBackend.SetupCalls) == 3 {
			return errors.New("cannot setup LXD profile")
		}
		return nil
	}
	defer func() { s.secBackend.SetupCallback = nil }()

	// Execute
	s.o.Settle(5 * time.Second)

	// Validate
	s.state.Lock()
	defer s.state.Unlock()
	c.Check(change.Err(), check.ErrorMatches, "(?s).*\\(cannot setup LXD profile\\)")
	c.Assert(change.Status(), check.Equals, state.ErrorStatus)

	repo := s.mgr.Repository()
	ref, err := repo.Connected(s.prj.ProjectId, "ws-consumer", "consumer", "plug")
	c.Assert(ref, check.HasLen, 1)
	c.Assert(err, check.IsNil)

	connection, err := repo.Connection(ref[0])
	c.Assert(err, check.IsNil)
	c.Assert(err, check.IsNil)
	var src string
	c.Assert(connection.Plug.Attr("source", &src), check.IsNil)
	c.Assert(src, check.Equals, oldSource)

	c.Assert(osutil.FileExists(oldSource), check.Equals, true)
	c.Assert(osutil.FileExists(newSource), check.Equals, false)
	// 2 calls for the autoconnect, no calls for the remount
	c.Assert(s.secBackend.SetupCalls, check.HasLen, 3)
}

func (s *interfaceHandlersSuite) TestDisconnectSuccess(c *check.C) {
	// Setup
	// Create an already installed workshop with a connected content plug
	repo := s.mgr.Repository()
	s.launchRemountWorkshop(c)

	connRefKey := "42424242:ws-consumer:consumer:plug core:core:core:content"
	s.state.Lock()
	s.state.Set("conns", map[string]interface{}{
		connRefKey: map[string]interface{}{
			"interface":    "content",
			"auto":         true,
			"plug-static":  map[string]interface{}{"attribute": "one"},
			"plug-dynamic": map[string]interface{}{"test-dynamic-attr": "new-dynamic-value"},
		},
	})
	_, err := ifacestate.ReloadConnections(s.mgr, "", "", "")
	c.Assert(err, check.IsNil)
	s.state.Unlock()

	// Execute
	s.state.Lock()
	chg := s.state.NewChange("sample", "...")
	t1 := s.state.NewTask("disconnect", "...")
	t1.Set("sdk", "consumer")
	setWorkshopProject("ws-consumer", s.prj, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	s.state.Unlock()

	s.o.Settle(5 * time.Second)

	s.state.Lock()
	defer s.state.Unlock()
	c.Check(chg.Err(), check.IsNil)

	// Validate
	c.Assert(t1.Status(), check.Equals, state.DoneStatus)
	c.Assert(repo.Plugs(s.prj.ProjectId, "ws-consumer", "consumer"), check.HasLen, 0)
	c.Assert(repo.Slots(s.prj.ProjectId, "ws-consumer", "consumer"), check.HasLen, 0)

	var stateConns map[string]interface{}
	c.Assert(s.state.Get("conns", &stateConns), check.IsNil)
	c.Assert(stateConns, check.HasLen, 0)

	var attrs map[string]interface{}
	c.Assert(t1.Get("plugs-to-remount", &attrs), check.IsNil)
	c.Assert(attrs, check.HasLen, 1)
	c.Assert(attrs[connRefKey], check.DeepEquals, map[string]interface{}{"test-dynamic-attr": "new-dynamic-value"})

	c.Assert(s.secBackend.SetupCalls, check.HasLen, 1)
	c.Assert(s.secBackend.RemoveCalls, check.HasLen, 1)
}
