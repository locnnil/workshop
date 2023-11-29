package ifacestate_test

import (
	"errors"
	"time"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/overlord/ifacestate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
	"golang.org/x/exp/maps"
	"gopkg.in/check.v1"
	"gopkg.in/tomb.v2"
)

type interfaceHandlersSuite struct {
	interfaceManagerSuite
	mgr                      *ifacestate.InterfaceManager
	restoreInterface         func()
	restoreSecurtityBackends func()
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
			"interface":   "mock-network",
			"auto":        true,
			"plug-static": map[string]interface{}{"attribute": "one"},
		},
	})

	// ensure that backend profiles were set for both SDKs
	c.Assert(s.secBackend.SetupCalls, check.HasLen, 2)
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

func (s *interfaceHandlersSuite) TestAutoconnectRemovesNonexistingConnections(c *check.C) {
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
	// simulate refresh, which will install the consumer SDK but now, the
	// connection must be gone as consumer does not have any plugs in this
	// revision
	chg = s.state.NewChange("refresh", "...")
	t2 := s.state.NewTask("auto-connect", "...")
	t2.Set("sdk", "consumer")
	chg.AddTask(t2)
	chg.Set("user", "testuser")
	setWorkshopProject("ws", s.prj, t2)

	c.Assert(s.wsbackend.RemoveWorkshop(s.ctx, "ws"), check.IsNil)
	s.launchWorkshopWithSDKs(c, "ws", map[sdk.Setup]string{csetup: consumerNoPlugs})
	s.state.Unlock()

	s.o.Settle(5 * time.Second)

	s.state.Lock()
	defer s.state.Unlock()

	// Validate
	c.Assert(t1.Status(), check.Equals, state.DoneStatus)
	c.Assert(t2.Status(), check.Equals, state.DoneStatus)

	conns, err := ifacestate.GetConns(s.state)
	c.Assert(err, check.IsNil)
	c.Assert(maps.Keys(conns), check.Not(testutil.Contains), "42424242:ws:consumer:plug 42424242:ws-producer:producer:slot")

	// ensure that backend profiles were set for both SDKs
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
