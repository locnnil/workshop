package ifacestate_test

import (
	"errors"

	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/overlord/ifacestate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshopbackend"
	"gopkg.in/check.v1"
	"gopkg.in/tomb.v2"
)

type interfaceHandlersSuite struct {
	interfaceManagerSuite
	mgr              *ifacestate.InterfaceManager
	restoreInterface func()
}

var _ = check.Suite(&interfaceHandlersSuite{})

func fakeHandler(task *state.Task, _ *tomb.Tomb) error {
	return nil
}

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

	s.se.AddManager(s.mgr)
	s.se.AddManager(s.runner)
	err := s.se.StartUp()
	c.Assert(err, check.IsNil)
}

func (s *interfaceHandlersSuite) TearDownTest(c *check.C) {
	s.restoreInterface()
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
	var producer = `name: producer
base: ubuntu@22.04
slots:
  slot:
    interface: mock-network
`

	err := s.mgr.Repository().AddSdk(sdk.MockInfo(c, producer, s.prj.ProjectId, "ws",
		sdk.Setup{Name: "producer", Channel: "latest/stable"}))
	c.Assert(err, check.IsNil)

	var consumer = `name: consumer
base: ubuntu@22.04
plugs:
  plug:
    interface: mock-network
`

	csetup := sdk.Setup{Name: "consumer", Channel: "latest/stable"}
	s.launchWorkshopWithSDKs(c, "ws", map[sdk.Setup]string{
		csetup: consumer,
	})

	s.state.Lock()
	chg := s.state.NewChange("sample", "...")
	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", csetup)
	t1 := s.state.NewTask("auto-connect", "...")
	t1.Set("sdk-retrieve-task", t.ID())

	setWorkshopProject("ws", s.prj, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t)
	chg.AddTask(t1)
	s.state.Unlock()

	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()
	c.Check(chg.Err(), check.Equals, nil)
}

func (s *interfaceHandlersSuite) TestAutoconnectUndoSuccess(c *check.C) {
	var producer = `name: producer
base: ubuntu@22.04
slots:
  slot:
    interface: mock-network`

	psetup := sdk.Setup{Name: "producer", Channel: "latest/stable"}
	s.launchWorkshopWithSDKs(c, "ws-producer", map[sdk.Setup]string{
		psetup: producer,
	})

	err := s.mgr.Repository().AddSdk(sdk.MockInfo(c, producer, s.prj.ProjectId, "ws-producer",
		sdk.Setup{Name: "producer", Channel: "latest/stable"}))
	c.Assert(err, check.IsNil)

	var consumer = `name: consumer
base: ubuntu@22.04
plugs:
  plug:
    interface: mock-network
`

	csetup := sdk.Setup{Name: "consumer", Channel: "latest/stable"}
	s.launchWorkshopWithSDKs(c, "ws", map[sdk.Setup]string{
		csetup: consumer,
	})

	s.state.Lock()
	chg := s.state.NewChange("sample", "...")
	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", csetup)
	t1 := s.state.NewTask("auto-connect", "...")
	t1.Set("sdk-retrieve-task", t.ID())
	t2 := s.state.NewTask("error-trigger", "...")

	setWorkshopProject("ws", s.prj, t1)
	setWorkshopProject("ws", s.prj, t2)
	chg.Set("user", "testuser")
	chg.AddTask(t)
	chg.AddTask(t1)
	chg.AddTask(t2)
	s.state.Unlock()

	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	ref, err := s.mgr.Repository().Connected(s.prj.ProjectId, "ws", "consumer", "plug")
	c.Assert(ref, check.HasLen, 0)
	c.Assert(err, check.ErrorMatches, "sdk \"consumer\" has no plug or slot named \"plug\"")

	ref, err = s.mgr.Repository().Connected(s.prj.ProjectId, "ws-producer", "producer", "slot")
	c.Assert(ref, check.HasLen, 0)
	c.Assert(err, check.IsNil)

	c.Assert(s.wsbackend.RemoveProfile(s.ctx, "ws", "consumer"), check.ErrorMatches, "profile not found")
}
