//go:build integration
// +build integration

package workshopbackend_test

import (
	"context"
	"os"
	"os/user"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/daemon"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	"github.com/spf13/afero"
	"gopkg.in/check.v1"
)

type wsOps struct {
	// per suite
	lxdClient lxd.InstanceServer
	be        workshop.WorkshopBackend

	// per test
	ctx                 context.Context
	username            string
	client              *client.Client
	daemon              *daemon.Daemon
	project             *workshop.Project
	lookupUserRestore   func()
	lookupUserIdRestore func()
	newProjectidRestore func()
	restoreDevices      func()
	restoreImageServer  func()
}

var _ = check.Suite(&wsOps{})

func (f *wsOps) SetUpTest(c *check.C) {
	socketPath := c.MkDir() + ".workshop.socket"
	var err error
	f.be, err = workshop.New()
	c.Assert(err, check.IsNil)

	d, err := daemon.New(&daemon.Options{
		Dir:        c.MkDir(),
		SocketPath: socketPath,
	}, f.be)
	c.Assert(err, check.IsNil)
	err = d.Init()
	c.Assert(err, check.IsNil)
	d.Start()
	f.daemon = d

	c.Check(err, check.IsNil)
	f.client, err = client.New(&client.Config{
		Socket: socketPath,
	})
	c.Assert(err, check.IsNil)

	f.project = &workshop.Project{
		ProjectId: "42424242",
		Path:      c.MkDir(),
	}
	f.ctx = createTestContext(f.username, f.project.ProjectId)

	f.lxdClient, _ = f.be.(*workshop.LxdBackend).LxdClient(f.ctx)
	err = workshop.InitProject(f.lxdClient, f.username)
	c.Check(err, check.IsNil)

	f.lookupUserRestore = testutil.FakeFunc(func(name string) (*user.User, error) {
		u := &user.User{
			Name:     f.username,
			Username: f.username,
			Uid:      "1000",
			Gid:      "1000",
		}
		return u, nil
	}, &workshop.LookupUsername)

	f.lookupUserIdRestore = testutil.FakeFunc(func(uid string) (*user.User, error) {
		u := &user.User{
			Name:     f.username,
			Username: f.username,
			Uid:      "1000",
			Gid:      "1000",
		}
		return u, nil
	}, &daemon.LookupUserId)

	f.newProjectidRestore = testutil.FakeFunc(func() (string, error) {
		return f.project.ProjectId, nil
	}, &workshop.NewProjectId)
	launchTestWorkshop(c, f.ctx, f.be, f.project.Path, f.username)
}

func (f *wsOps) TearDownTest(c *check.C) {
	defer f.be.RemoveWorkshop(f.ctx, "test")
	err := f.daemon.Stop(nil)
	c.Check(err, check.IsNil)
	f.lookupUserRestore()
	f.lookupUserIdRestore()
	f.newProjectidRestore()
	err = os.RemoveAll(f.project.Path)
	c.Check(err, check.IsNil)
}

func (f *wsOps) SetUpSuite(c *check.C) {
	f.username = "testuser"
	f.restoreDevices = workshop.FakeDefaultDevices(defaultTestDevices)
	f.restoreImageServer = workshop.FakeImageServer(minimalImageServer)
}

func (f *wsOps) TearDownSuite(c *check.C) {
	cleanUpLxdProject(c, f.lxdClient, workshop.LxdProjectName(f.username))
	cleanUpLxdProject(c, f.lxdClient, workshop.LxdSystemProjectName(f.username))
	f.restoreDevices()
	f.restoreImageServer()
}

func (f *wsOps) TestLxdBackendWorkshopStashUnstash(c *check.C) {
	// Execute
	err := f.be.StashWorkshop(f.ctx, "test")
	c.Assert(err, check.IsNil)

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.Workshop(f.ctx, "test")
	c.Assert(err, check.NotNil)

	// Execute
	err = f.be.UnstashWorkshop(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.Workshop(f.ctx, "test")
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendWorkshopStashRestartIfFailed(c *check.C) {
	// Setup
	// Change the stash name prefix to invalid to emulate
	// migration failure. The instance must preserve its
	// list of the SDK profiles
	old := workshop.StashNamePrefix
	workshop.StashNamePrefix = "?"
	defer func() { workshop.StashNamePrefix = old }()

	// Execute (will fail due to the incorrect stash instance name)
	err := f.be.StashWorkshop(f.ctx, "test")
	c.Assert(err, check.NotNil)

	// Validate
	inst, _, err := f.lxdClient.GetInstance(workshop.InstanceName("test", f.project.ProjectId))
	c.Assert(err, check.IsNil)
	c.Assert(inst.Status, check.Equals, "Running")
}

func (f *wsOps) TestLxdBackendWorkshopStashRemove(c *check.C) {
	// Execute
	err := f.be.StashWorkshop(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.Workshop(f.ctx, "test")
	c.Assert(err, check.NotNil)

	// Execute
	err = f.be.RemoveWorkshopStash(f.ctx, "test")
	c.Assert(err, check.IsNil)

	// Validate
	err = f.be.UnstashWorkshop(f.ctx, "test")
	c.Assert(err, check.ErrorMatches, "workshop not found")
}

func (f *wsOps) TestLxdBackendStateStorageVolumeAddRemove(c *check.C) {
	// Execute
	err := f.be.CreateStateStorage(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	vol, _, err := f.lxdClient.GetStoragePoolVolume("default", "custom", workshop.WorkshopStateVolumeName("test", f.project.ProjectId))
	c.Assert(err, check.IsNil)
	c.Assert(vol.ContentType, check.Equals, "filesystem")

	// Execute
	err = f.be.DeleteStateStorage(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendRemoveWorkshopStash(c *check.C) {
	// Setup
	wf := &workshop.WorkshopFile{Name: "test-1", Base: "ubuntu@20.04"}
	err := f.be.LaunchWorkshop(f.ctx, wf)
	defer f.be.RemoveWorkshop(f.ctx, "test-1")
	c.Assert(err, check.IsNil)

	// Execute
	err = f.be.StashWorkshop(f.ctx, "test-1")

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.Workshop(f.ctx, "test-1")
	c.Assert(err, check.NotNil)

	// Execute
	err = f.be.RemoveWorkshopStash(f.ctx, "test-1")
	c.Assert(err, check.IsNil)
	cli := f.lxdClient.UseProject(workshop.LxdSystemProjectName(f.username))
	_, _, err = cli.GetInstance(workshop.InstanceName("test-1", f.project.ProjectId))
	c.Assert(err, check.ErrorMatches, ".*Instance not found")
}

func (f *wsOps) TestLxdBackendStartWorkshop(c *check.C) {
	// Setup
	wf := &workshop.WorkshopFile{Name: "test-1", Base: "ubuntu@20.04"}
	err := f.be.LaunchWorkshop(f.ctx, wf)
	c.Assert(err, check.IsNil)
	defer f.be.RemoveWorkshop(f.ctx, "test-1")

	// Execute
	err = f.be.StartWorkshop(f.ctx, "test-1")

	//Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.Workshop(f.ctx, "test-1")
	c.Assert(err, check.IsNil)

	// now, ensure that the systemd is in the final state
	memFs := afero.NewMemMapFs()
	out, _ := memFs.Create("stdout")
	args := workshop.Execution{
		ExecArgs: workshop.ExecArgs{
			UserId:  0,
			GroupId: 0,
			Command: []string{
				"bash", "-eu", "-c",
				"systemctl is-system-running",
			},
			WorkDir: "/",
		},
		ExecControls: workshop.ExecControls{
			Stdin:  nil,
			Stdout: out,
			Stderr: out,
		},
	}

	exectx, err := f.be.Exec(f.ctx, "test-1", &args)
	c.Assert(err, check.IsNil)
	err = exectx.WaitExecution(f.ctx)
	_, err = afero.ReadFile(memFs, out.Name())
	c.Assert(err, check.IsNil)

	// TODO: uncomment, when LXD fixes the image launching, currently both our
	// bases are run in a degraded status
	// c.Assert(string(buf), check.Equals, "running\n")
}

func (f *wsOps) TestLxdBackendDeleteWorkshop(c *check.C) {
	// Execute
	wf := &workshop.WorkshopFile{Name: "test-1", Base: "ubuntu@22.04"}
	err := f.be.LaunchWorkshop(f.ctx, wf)
	c.Assert(err, check.IsNil)
	err = f.be.StartWorkshop(f.ctx, "test-1")
	c.Assert(err, check.IsNil)

	//Validate
	err = f.be.RemoveWorkshop(f.ctx, "test-1")
	c.Assert(err, check.IsNil)
}
