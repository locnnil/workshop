//go:build integration
// +build integration

package workshopbackend_test

import (
	"context"

	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
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
	device := workshopbackend.Mount("sdk-device", c.MkDir(), "/root")
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

func (f *profileTest) TestSdkProfileBindMountFailsIfTargetDoesNotExist(c *check.C) {
	// Setup
	var backend workshopbackend.Profile = &workshopbackend.LxdBackend{}
	profile := workshopbackend.NewSdkProfile("sdk")
	device := workshopbackend.Mount("sdk-device", c.MkDir(), "/not-exist")
	err := profile.AddDevice(device)
	c.Assert(err, check.IsNil)

	// Execute
	err = backend.AssignProfile(f.ctx, "test", profile)
	c.Assert(err, check.ErrorMatches, `.*cannot create a workshop mount with target "/not-exist": file does not exist`)

	// Setup
	profile = workshopbackend.NewSdkProfile("sdk")
	device = workshopbackend.Mount("sdk-device", c.MkDir(), "/root/.profile")
	err = profile.AddDevice(device)
	c.Assert(err, check.IsNil)

	// Execute
	err = backend.AssignProfile(f.ctx, "test", profile)
	c.Assert(err, check.ErrorMatches, `.*cannot create a workshop mount with target "/root/.profile": the target is not a directory`)
}

func (f *profileTest) TestSdkProfileBindMountPreventsLxdFromRemovingTarget(c *check.C) {
	// Setup
	var backend workshopbackend.Profile = &workshopbackend.LxdBackend{}
	profile := workshopbackend.NewSdkProfile("sdk")
	device := workshopbackend.Mount("sdk-device", c.MkDir(), "/opt")
	err := profile.AddDevice(device)
	c.Assert(err, check.IsNil)

	// Execute
	err = backend.AssignProfile(f.ctx, "test", profile)
	c.Assert(err, check.IsNil)
	err = backend.RemoveProfile(f.ctx, "test", profile.Name())
	c.Assert(err, check.IsNil)

	// Validate
	fs, err := f.client.GetInstanceFileSFTP(workshopbackend.InstanceName("test", "42424242"))
	c.Assert(err, check.IsNil)
	defer fs.Close()
	_, err = fs.Stat("/opt")
	c.Assert(err, check.IsNil)
}
