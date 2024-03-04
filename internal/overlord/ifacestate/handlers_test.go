package ifacestate_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/overlord/ifacestate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
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

func (s *interfaceHandlersSuite) TestDoAutoconnectFailInstallPolicyCheck(c *check.C) {
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

func (s *interfaceHandlersSuite) newRemountChange(newSource string) *state.Change {
	s.state.Lock()
	t1 := s.state.NewTask("auto-connect", "test")
	t1.Set("sdk", "consumer")
	t2 := s.state.NewTask("remount", "remount")
	t2.Set("remount-source", newSource)
	t2.Set("remount-plug", interfaces.PlugRef{ProjectId: s.prj.ProjectId, Workshop: "ws-consumer", Sdk: "consumer", Name: "plug"})
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

func (s *interfaceHandlersSuite) launchRemountWorkshop(c *check.C, oldSource string) {
	// Note: we set the source attribute for the plug in these tests, however,
	// it is a dynamic attribute that will be defined when we install an SDK
	// into a workshop as the default path depends on the username
	var sdkYaml = fmt.Sprintf(`
name: consumer
base: ubuntu@22.04
plugs:
    plug:
        interface: content
        target: /home/workshop
        source: %s
`, oldSource)
	s.launchWorkshopWithSDKs(c, "ws-consumer", map[sdk.Setup]string{csetup: sdkYaml})
}

func (s *interfaceHandlersSuite) TestRemountSuccessDestExistsAndEmpty(c *check.C) {
	// Setup
	oldSource := c.MkDir()
	newSource := c.MkDir()
	_, err := os.Create(filepath.Join(oldSource, "tempfile"))
	c.Check(err, check.IsNil)

	s.launchRemountWorkshop(c, oldSource)
	change := s.newRemountChange(newSource)

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
	c.Assert(connection.Plug.Attr("remount-source", &remountSource), check.IsNil)
	c.Assert(remountSource, check.Equals, remountSource)

	c.Assert(osutil.FileExists(oldSource), check.Equals, false)
	// 2 calls for the autoconnect, one call for the remount
	c.Assert(s.secBackend.SetupCalls, check.HasLen, 2+1)
	c.Assert(s.secBackend.SetupCalls[2].SdkInfo.Name, check.Equals, "consumer")
	c.Assert(s.secBackend.SetupCalls[2].SdkInfo.Workshop, check.Equals, "ws-consumer")
}

func (s *interfaceHandlersSuite) TestRemountSuccessDestDoesNotExist(c *check.C) {
	// Setup
	oldSource := c.MkDir()
	newSource := filepath.Join(c.MkDir(), "new")
	s.launchRemountWorkshop(c, oldSource)
	change := s.newRemountChange(newSource)

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
	c.Assert(connection.Plug.Attr("remount-source", &remountSource), check.IsNil)
	c.Assert(remountSource, check.Equals, remountSource)

	c.Assert(osutil.FileExists(oldSource), check.Equals, false)
	// 2 calls for the autoconnect, one call for the remount
	c.Assert(s.secBackend.SetupCalls, check.HasLen, 2+1)
	c.Assert(s.secBackend.SetupCalls[2].SdkInfo.Name, check.Equals, "consumer")
	c.Assert(s.secBackend.SetupCalls[2].SdkInfo.Workshop, check.Equals, "ws-consumer")
}

func (s *interfaceHandlersSuite) TestRemountRenameFails(c *check.C) {
	// Setup
	oldSource := c.MkDir()
	newSource := c.MkDir()
	_, err := os.Create(filepath.Join(newSource, "tempfile"))
	c.Check(err, check.IsNil)
	s.launchRemountWorkshop(c, oldSource)
	change := s.newRemountChange(newSource)

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
	c.Assert(connection.Plug.Attr("remount-source", string("")), testutil.ErrorIs, sdk.AttributeNotFoundError{})

	c.Assert(osutil.FileExists(oldSource), check.Equals, true)
	c.Assert(osutil.FileExists(newSource), check.Equals, true)
	// 2 calls for the autoconnect, no calls for the remount
	c.Assert(s.secBackend.SetupCalls, check.HasLen, 2)
}

func (s *interfaceHandlersSuite) TestRemountInterfaceBackendSetupFails(c *check.C) {
	// Setup
	oldSource := c.MkDir()
	newSource := c.MkDir()
	s.launchRemountWorkshop(c, oldSource)
	change := s.newRemountChange(newSource)

	s.secBackend.SetupCallback = func(context context.Context, sdkInfo *sdk.Info, repo *interfaces.Repository) error {
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
	c.Assert(connection.Plug.Attr("remount-source", string("")), testutil.ErrorIs, sdk.AttributeNotFoundError{})

	c.Assert(osutil.FileExists(oldSource), check.Equals, true)
	c.Assert(osutil.FileExists(newSource), check.Equals, false)
	// 2 calls for the autoconnect, no calls for the remount
	c.Assert(s.secBackend.SetupCalls, check.HasLen, 3)
}
