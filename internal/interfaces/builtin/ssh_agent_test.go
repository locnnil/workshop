package builtin_test

import (
	"os/user"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/builtin"
	"github.com/canonical/workshop/internal/interfaces/lxd_device"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
)

type sshAgentSuite struct {
	iface          interfaces.Interface
	projectId      string
	restoreUserEnv func()
}

var _ = check.Suite(&sshAgentSuite{
	iface: builtin.MustInterface("ssh-agent"),
})

func (s *sshAgentSuite) SetUpTest(c *check.C) {
	s.projectId = "42424242"
	testuser.HomeDir = c.MkDir()
	s.restoreUserEnv = osutil.FakeUserAndEnv(func(name string) (*user.User, map[string]string, error) {
		return &testuser, map[string]string{"SSH_AUTH_SOCK": "/tmp/dir/ssh"}, nil
	})
}

func (s *sshAgentSuite) TearDownTest(c *check.C) {
	s.restoreUserEnv()
}

func (s *sshAgentSuite) TestName(c *check.C) {
	c.Assert(s.iface.Name(), check.Equals, "ssh-agent")
}

func (s *sshAgentSuite) TestInterfaces(c *check.C) {
	c.Check(builtin.Interfaces(), testutil.DeepContains, s.iface)
}

func (s *sshAgentSuite) TestSshAgentInterface(c *check.C) {
	plug := builtin.MockPlug(c, `name: consumer
base: ubuntu@22.04
plugs:
  ssh-agent:
    interface: ssh-agent
`, s.projectId, "ws", "consumer", "ssh-agent")
	connectedPlug := interfaces.NewConnectedPlug(plug, nil, nil)

	slot := builtin.MockSlot(c, `name: producer
base: ubuntu@22.04
slots:
  ssh-agent:
`, s.projectId, "ws", "producer", "ssh-agent")
	connectedSlot := interfaces.NewConnectedSlot(slot, nil, nil)
	deviceSpec, err := lxd_device.NewSpecification(testuser.Name, "consumer")
	c.Assert(err, check.IsNil)

	c.Assert(deviceSpec.AddConnectedPlug(s.iface, connectedPlug, connectedSlot), check.IsNil)
	expectedProxy := &workshop.SshAgent{ProxyEntry: workshop.ProxyEntry{
		Name: plug.Name,
		Connect: workshop.ProxyTarget{
			Address:  "/tmp/dir/ssh",
			Protocol: "unix",
		},
		Listen: workshop.ProxyTarget{
			Address:  "/var/lib/workshop/run/consumer_ssh-agent.ssh",
			Protocol: "unix",
		},
		Direction: workshop.WorkshopToHost,
	}}
	c.Assert(deviceSpec.Profile.Agent, check.DeepEquals, expectedProxy)
}
