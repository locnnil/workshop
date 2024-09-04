//go:build integration
// +build integration

package workshopbackend_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
	"gopkg.in/check.v1"
)

type profileTest struct {
	ctx      context.Context
	client   lxd.InstanceServer
	username string
	be       workshop.Backend

	restoreUserLookup   func()
	newProjectidRestore func()
	restoreDevices      func()
}

var _ = check.Suite(&profileTest{})

func testProjectId() (string, error) {
	return "42424242", nil
}

func (f *profileTest) SetUpSuite(c *check.C) {
	f.username = "testuser"
	f.ctx = createTestContext(f.username, "42424242")

	var err error
	f.be, err = lxdbackend.New()
	c.Assert(err, check.IsNil)
	f.client, _ = f.be.(*lxdbackend.Backend).LxdClient(f.ctx)

	f.restoreUserLookup = testutil.FakeFunc(func(name string) (*user.User, error) {
		u := &user.User{
			Name:     f.username,
			Username: f.username,
			Uid:      "1000",
			Gid:      "1000",
		}
		return u, nil
	}, &workshop.LookupUsername)
	f.restoreDevices = lxdbackend.FakeDefaultDevices(defaultTestDevices)
	f.newProjectidRestore = testutil.FakeFunc(testProjectId, &workshop.NewProjectId)
}

func (f *profileTest) TearDownSuite(c *check.C) {
	f.client, _ = f.be.(*lxdbackend.Backend).LxdClient(f.ctx)
	cleanUpLxdProject(c, f.client, lxdbackend.LxdProjectName(f.username))
	cleanUpLxdProject(c, f.client, lxdbackend.LxdSystemProjectName(f.username))
	f.newProjectidRestore()
	f.restoreDevices()
	f.restoreUserLookup()
}

func (f *profileTest) TestSdkProfileCreatedAndUpdatedSuccessfully(c *check.C) {
	// Setup
	launchTestWorkshop(c, f.ctx, f.be, c.MkDir())
	defer f.be.RemoveWorkshop(f.ctx, "test")

	var backend workshop.Profile = &lxdbackend.Backend{}
	profile := workshop.NewSdkProfile("sdk")
	// ensure the target directory is created as a workaround for the LXD bind-mount issue
	device := lxdbackend.HostWorkshopMount("sdk-device", c.MkDir(), "/new-dir")
	err := profile.AddDevice(device)
	c.Assert(err, check.IsNil)

	// Execute
	err = backend.AssignProfile(f.ctx, "test", profile)
	c.Assert(err, check.IsNil)

	// Validate
	p, _, err := f.client.GetProfile("test-42424242-sdk")
	c.Assert(err, check.IsNil)
	c.Assert(p.ProfilePut.Devices["sdk-device"], check.NotNil)
	c.Assert(p.ProfilePut.Devices, check.HasLen, 1)

	inst, _, err := f.client.GetInstance(lxdbackend.InstanceName("test", "42424242"))
	c.Assert(err, check.IsNil)
	c.Assert(inst.Profiles, testutil.DeepUnsortedMatches, []string{"default", "test-42424242-sdk"})

	// Setup (now, update the already existing profile with a new device)
	err = profile.AddDevice(lxdbackend.HostWorkshopMount("sdk-device-2", c.MkDir(), "/home"))
	c.Assert(err, check.IsNil)

	// Execute
	err = backend.AssignProfile(f.ctx, "test", profile)
	c.Assert(err, check.IsNil)

	// Validate
	p, _, err = f.client.GetProfile("test-42424242-sdk")
	c.Assert(err, check.IsNil)
	c.Assert(p.ProfilePut.Devices["sdk-device"], check.NotNil)
	c.Assert(p.ProfilePut.Devices["sdk-device-2"], check.NotNil)
	c.Assert(p.ProfilePut.Devices, check.HasLen, 2)

	inst, _, err = f.client.GetInstance(lxdbackend.InstanceName("test", "42424242"))
	c.Assert(err, check.IsNil)
	c.Assert(inst.Profiles, testutil.DeepUnsortedMatches, []string{"default", "test-42424242-sdk"})

	err = backend.RemoveProfile(f.ctx, "test", profile.Sdk)
	c.Assert(err, check.IsNil)
}

func (f *profileTest) TestSdkProfileBindMountFailsIfTargetIsAFile(c *check.C) {
	// Setup
	launchTestWorkshop(c, f.ctx, f.be, c.MkDir())
	defer f.be.RemoveWorkshop(f.ctx, "test")

	var backend workshop.Profile = &lxdbackend.Backend{}
	profile := workshop.NewSdkProfile("sdk")
	device := lxdbackend.HostWorkshopMount("sdk-device", c.MkDir(), "/root/.profile")
	err := profile.AddDevice(device)
	c.Assert(err, check.IsNil)

	// Execute
	err = backend.AssignProfile(f.ctx, "test", profile)
	c.Assert(err, check.ErrorMatches, `sdk:sdk-device's "workshop-target" /root/.profile is not a directory`)
}

func (f *profileTest) TestSdkProfileBindMountAbsSourceOK(c *check.C) {
	// Setup
	pd := c.MkDir()
	launchTestWorkshop(c, f.ctx, f.be, pd)
	defer f.be.RemoveWorkshop(f.ctx, "test")

	var backend workshop.Profile = &lxdbackend.Backend{}
	profile := workshop.NewSdkProfile("sdk")
	abs := filepath.Join(c.MkDir(), "absolute", "source")
	device := lxdbackend.HostWorkshopMount("sdk-device", abs, "/opt")
	err := profile.AddDevice(device)
	c.Assert(err, check.IsNil)

	// Execute
	err = backend.AssignProfile(f.ctx, "test", profile)
	c.Assert(err, check.IsNil)
	c.Assert(abs, testutil.FilePresent)
}

func (f *profileTest) TestSdkProfileBindMountRelativeSourceOK(c *check.C) {
	// Setup
	pd := c.MkDir()
	launchTestWorkshop(c, f.ctx, f.be, pd)
	defer f.be.RemoveWorkshop(f.ctx, "test")

	err := os.MkdirAll(filepath.Join(pd, "relpath"), 0744)
	c.Assert(err, check.IsNil)

	var backend workshop.Profile = &lxdbackend.Backend{}
	profile := workshop.NewSdkProfile("sdk")
	device := lxdbackend.HostWorkshopMount("sdk-device", "relpath", "/opt")
	err = profile.AddDevice(device)
	c.Assert(err, check.IsNil)

	// Execute
	err = backend.AssignProfile(f.ctx, "test", profile)
	c.Assert(err, check.IsNil)
}

func (f *profileTest) TestSdkProfileBindMountRelativeSourceNotExist(c *check.C) {
	// Setup
	pd := c.MkDir()
	launchTestWorkshop(c, f.ctx, f.be, pd)
	defer f.be.RemoveWorkshop(f.ctx, "test")

	var backend workshop.Profile = &lxdbackend.Backend{}
	profile := workshop.NewSdkProfile("sdk")
	device := lxdbackend.HostWorkshopMount("sdk-device", "relpath", "/opt")
	err := profile.AddDevice(device)
	c.Assert(err, check.IsNil)

	// Execute
	err = backend.AssignProfile(f.ctx, "test", profile)
	c.Assert(err, check.ErrorMatches, fmt.Sprintf(`sdk:sdk-device's "source" %s is not an existing directory`, filepath.Join(pd, "relpath")))
}

func (f *profileTest) TestSdkProfileSshAgentProxy(c *check.C) {
	// Setup
	launchTestWorkshop(c, f.ctx, f.be, c.MkDir())
	defer f.be.RemoveWorkshop(f.ctx, "test")

	var backend workshop.Profile = &lxdbackend.Backend{}
	profile := workshop.NewSdkProfile("sdk")
	sshAgentDir := c.MkDir()
	sockPath := filepath.Join(sshAgentDir, "ssh")
	sock, err := net.Listen("unix", sockPath)
	c.Assert(err, check.IsNil)
	defer func() {
		sock.Close()
		os.Remove(sockPath)
	}()

	device := lxdbackend.SshAgent("agent", sockPath, "/home/workshop/ssh-agent.ssh")
	err = profile.AddDevice(device)
	c.Assert(err, check.IsNil)

	// Execute
	err = backend.AssignProfile(f.ctx, "test", profile)
	c.Assert(err, check.IsNil)

	// Validate
	fs, err := f.client.GetInstanceFileSFTP(lxdbackend.InstanceName("test", "42424242"))
	c.Assert(err, check.IsNil)
	defer fs.Close()
	var buf = make([]byte, 100)
	agentScript, err := fs.Open("/etc/profile.d/agent.sh")
	c.Assert(err, check.IsNil)
	n, _ := agentScript.Read(buf)
	c.Assert(string(buf[:n]), check.Equals, "export SSH_AUTH_SOCK=/home/workshop/ssh-agent.ssh")

	// Execute
	// Simulate a scenario when a profile is updated not created
	err = backend.AssignProfile(f.ctx, "test", workshop.NewSdkProfile("sdk"))
	c.Assert(err, check.IsNil)

	// Validate
	_, err = fs.Stat("/etc/profile.d/agent.sh")
	c.Assert(os.IsNotExist(err), check.Equals, true)

	err = backend.RemoveProfile(f.ctx, "test", profile.Sdk)
	c.Assert(err, check.IsNil)
}

func (f *profileTest) TestSdkProfileRemoveNotFound(c *check.C) {
	// Setup
	launchTestWorkshop(c, f.ctx, f.be, c.MkDir())
	defer f.be.RemoveWorkshop(f.ctx, "test")

	var backend workshop.Profile = &lxdbackend.Backend{}
	err := backend.RemoveProfile(f.ctx, "test", "sdk")
	c.Assert(err, testutil.ErrorIs, workshop.ErrSdkProfileNotFound)
}
