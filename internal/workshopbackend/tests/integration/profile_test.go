//go:build integration
// +build integration

package workshopbackend_test

import (
	"context"

	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
	lxd "github.com/lxc/lxd/client"
	"gopkg.in/check.v1"
)

type profileTest struct {
	ctx      context.Context
	client   lxd.InstanceServer
	username string
	be       workshopbackend.WorkshopBackend

	newProjectidRestore func()
}

var _ = check.Suite(&profileTest{})

func (f *profileTest) SetUpTest(c *check.C) {
	f.username = "testuser"
	f.newProjectidRestore = testutil.FakeFunc(func() (string, error) {
		return "42424242", nil
	}, &workshopbackend.NewProjectId)
	f.ctx = createTestContext(f.username, "42424242")
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
