//go:build integration
// +build integration

package workshopbackend_test

import (
	"context"
	"net"
	"os"
	"path/filepath"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
	"gopkg.in/check.v1"
)

type profileTest struct {
	ctx      context.Context
	client   lxd.InstanceServer
	username string
	be       workshopbackend.WorkshopBackend

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

	f.be = &workshopbackend.LxdBackend{}
	f.client, _ = f.be.(*workshopbackend.LxdBackend).LxdClient(f.ctx)
	err := f.client.CreateStoragePool(api.StoragePoolsPost{StoragePoolPut: api.StoragePoolPut{Config: map[string]string{"volume.size": "1GiB"}}, Name: "testZfsProfile", Driver: "zfs"})
	c.Assert(err, check.IsNil)
}

func (f *profileTest) TearDownSuite(c *check.C) {
	f.client, _ = f.be.(*workshopbackend.LxdBackend).LxdClient(f.ctx)
	err := f.client.DeleteStoragePool("testZfsProfile")
	c.Check(err, check.IsNil)
}

func (f *profileTest) SetUpTest(c *check.C) {
	f.restoreDevices = workshopbackend.FakeDefaultDevices(defaultTestDevices)
	f.newProjectidRestore = testutil.FakeFunc(testProjectId, &workshopbackend.NewProjectId)

	f.be = &workshopbackend.LxdBackend{}
	f.client, _ = f.be.(*workshopbackend.LxdBackend).LxdClient(f.ctx)
	err := workshopbackend.InitProject(f.client, f.username)
	c.Assert(err, check.IsNil)

	launchTestWorkshop(c, f.ctx, f.be, c.MkDir(), f.username)
}

func (f *profileTest) TearDownTest(c *check.C) {
	cleanUpLxdProject(c, f.client, workshopbackend.LxdProjectName(f.username))
	cleanUpLxdProject(c, f.client, workshopbackend.LxdSystemProjectName(f.username))
	f.newProjectidRestore()
	f.restoreDevices()
}

func (f *profileTest) TestSdkProfileCreatedAndUpdatedSuccessfully(c *check.C) {
	// Setup
	var backend workshopbackend.Profile = &workshopbackend.LxdBackend{}
	profile := workshopbackend.NewSdkProfile("sdk")
	// ensure the target directory is created as a workaround for the LXD bind-mount issue
	device := workshopbackend.Mount("sdk-device", c.MkDir(), "/new-dir")
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

	inst, _, err := f.client.GetInstance(workshopbackend.InstanceName("test", "42424242"))
	c.Assert(err, check.IsNil)
	c.Assert(inst.Profiles, testutil.DeepUnsortedMatches, []string{"default", "test-42424242-sdk"})

	// Setup (now, update the already existing profile with a new device)
	err = profile.AddDevice(workshopbackend.Mount("sdk-device-2", c.MkDir(), "/home"))
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

	inst, _, err = f.client.GetInstance(workshopbackend.InstanceName("test", "42424242"))
	c.Assert(err, check.IsNil)
	c.Assert(inst.Profiles, testutil.DeepUnsortedMatches, []string{"default", "test-42424242-sdk"})
}

func (f *profileTest) TestSdkProfileBindMountFailsIfTargetIsAFile(c *check.C) {
	// Setup
	var backend workshopbackend.Profile = &workshopbackend.LxdBackend{}
	profile := workshopbackend.NewSdkProfile("sdk")
	device := workshopbackend.Mount("sdk-device", c.MkDir(), "/root/.profile")
	err := profile.AddDevice(device)
	c.Assert(err, check.IsNil)

	// Execute
	err = backend.AssignProfile(f.ctx, "test", profile)
	c.Assert(err, check.ErrorMatches, `.*cannot create a workshop mount with target "/root/.profile": the target is not a directory`)
}

func (f *profileTest) TestSdkProfileSshAgentProxy(c *check.C) {
	// Setup
	var backend workshopbackend.Profile = &workshopbackend.LxdBackend{}
	profile := workshopbackend.NewSdkProfile("sdk")
	sshAgentDir := c.MkDir()
	sockPath := filepath.Join(sshAgentDir, "ssh")
	sock, err := net.Listen("unix", sockPath)
	c.Assert(err, check.IsNil)
	defer func() {
		sock.Close()
		os.Remove(sockPath)
	}()

	device := workshopbackend.SshAgent("agent", sockPath, "/home/workshop/ssh-agent.ssh")
	err = profile.AddDevice(device)
	c.Assert(err, check.IsNil)

	// Execute
	err = backend.AssignProfile(f.ctx, "test", profile)
	c.Assert(err, check.IsNil)

	// Validate
	fs, err := f.client.GetInstanceFileSFTP(workshopbackend.InstanceName("test", "42424242"))
	c.Assert(err, check.IsNil)
	defer fs.Close()
	var buf = make([]byte, 100)
	agentScript, err := fs.Open("/etc/profile.d/agent.sh")
	c.Assert(err, check.IsNil)
	n, _ := agentScript.Read(buf)
	c.Assert(err, check.IsNil)
	c.Assert(string(buf[:n]), check.Equals, "export SSH_AUTH_SOCK=/home/workshop/ssh-agent.ssh")

	// Execute
	// Simulate a scenario when a profile is updated not created
	err = backend.AssignProfile(f.ctx, "test", workshopbackend.NewSdkProfile("sdk"))
	c.Assert(err, check.IsNil)

	// Validate
	_, err = fs.Stat("/etc/profile.d/agent.sh")
	c.Assert(os.IsNotExist(err), check.Equals, true)
}

func (f *profileTest) TestSdkProfileRemove(c *check.C) {
	// Setup
	var backend workshopbackend.Profile = &workshopbackend.LxdBackend{}
	err := backend.RemoveProfile(f.ctx, "test", "sdk")
	c.Assert(err, testutil.ErrorIs, workshopbackend.ErrSdkProfileNotFound)
}
