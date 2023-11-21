//go:build integration
// +build integration

package lxdbackend_integration_test

import (
	"context"
	"os"
	"os/user"
	"path/filepath"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/daemon"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
	lxd "github.com/lxc/lxd/client"
	"github.com/spf13/afero"
	"gopkg.in/check.v1"
)

type wsOps struct {
	// per suite
	lxdClient lxd.InstanceServer
	be        workshopbackend.WorkshopBackend

	// per test
	ctx                 context.Context
	username            string
	client              *client.Client
	daemon              *daemon.Daemon
	project             *workshopbackend.Project
	lookupUserRestore   func()
	lookupUserIdRestore func()
	newProjectidRestore func()
}

var _ = check.Suite(&wsOps{})

func (f *wsOps) SetUpTest(c *check.C) {
	socketPath := c.MkDir() + ".workshop.socket"
	f.be = workshopbackend.New()

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

	f.project = &workshopbackend.Project{
		ProjectId: "42424242",
		Path:      c.MkDir(),
	}
	f.username = "testuser"
	f.ctx = createTestContext(f.username, f.project.ProjectId)

	f.lxdClient, _ = f.be.(*workshopbackend.LxdBackend).LxdClient(f.ctx)
	err = workshopbackend.InitProject(f.lxdClient, f.username)
	c.Check(err, check.IsNil)

	f.lookupUserRestore = testutil.FakeFunc(func(name string) (*user.User, error) {
		u := &user.User{
			Name:     f.username,
			Username: f.username,
			Uid:      "1000",
			Gid:      "1000",
		}
		return u, nil
	}, &workshopbackend.LookupUsername)

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
	}, &workshopbackend.NewProjectId)
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

func createTestContext(username, projectId string) context.Context {
	ctx := context.WithValue(context.Background(), workshopbackend.ContextUser, username)
	ctx = context.WithValue(ctx, workshopbackend.ContextProjectId, projectId)
	return ctx
}

func launchTestWorkshop(c *check.C, ctx context.Context, be workshopbackend.WorkshopBackend, dir, username string) {
	restore := testutil.FakeFunc(func(name string) (*user.User, error) {
		u := &user.User{
			Name:     username,
			Username: username,
			Uid:      "1000",
			Gid:      "1000",
		}
		return u, nil
	}, &workshopbackend.LookupUsername)
	defer restore()

	var err error

	os.WriteFile(filepath.Join(dir, ".workshop.test.yaml"), []byte(`name: test
base: ubuntu@22.04
`), 0644)

	_, _, err = be.CreateOrLoadProject(ctx, dir)
	c.Assert(err, check.IsNil)
	err = be.LaunchWorkshop(ctx, "test", "ubuntu@22.04")
	c.Assert(err, check.IsNil)
	err = be.StartWorkshop(ctx, "test")
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TearDownSuite(c *check.C) {
	cleanUpLxdProject(c, f.lxdClient, workshopbackend.LxdProjectName(f.username))
	cleanUpLxdProject(c, f.lxdClient, workshopbackend.LxdSystemProjectName(f.username))
}

func (f *wsOps) TestLxdBackendTrivialLaunch(c *check.C) {
	// Execute
	err := f.be.LaunchWorkshop(f.ctx, "test-1", "ubuntu@22.04")
	defer f.be.RemoveWorkshop(f.ctx, "test-1")

	//Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.Workshop(f.ctx, "test-1")
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendUnstashWorkshop(c *check.C) {
	// Execute
	err := f.be.StashWorkshop(f.ctx, "test")

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

func (f *wsOps) TestLxdBackendStateStorageVolumeAddRemove(c *check.C) {
	// Execute
	err := f.be.CreateStateStorage(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	vol, _, err := f.lxdClient.GetStoragePoolVolume("default", "custom", workshopbackend.WorkshopStateVolumeName("test", f.project.ProjectId))
	c.Assert(err, check.IsNil)
	c.Assert(vol.ContentType, check.Equals, "filesystem")

	// Execute
	err = f.be.DeleteStateStorage(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendRemoveWorkshopStash(c *check.C) {
	// Setup
	err := f.be.LaunchWorkshop(f.ctx, "test-1", "ubuntu@22.04")
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
	cli := f.lxdClient.UseProject(workshopbackend.LxdSystemProjectName(f.username))
	_, _, err = cli.GetInstance(workshopbackend.InstanceName("test-1", f.project.ProjectId))
	c.Assert(err, check.ErrorMatches, "Instance not found")
}

func (f *wsOps) TestLxdBackendStartWorkshop(c *check.C) {
	// Setup
	err := f.be.LaunchWorkshop(f.ctx, "test-1", "ubuntu@22.04")
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
	args := workshopbackend.Execution{
		ExecArgs: workshopbackend.ExecArgs{
			UserId:  0,
			GroupId: 0,
			Command: []string{
				"bash", "-eu", "-c",
				"systemctl is-system-running",
			},
			WorkDir: "/",
		},
		ExecControls: workshopbackend.ExecControls{
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
	err := f.be.LaunchWorkshop(f.ctx, "test-1", "ubuntu@22.04")
	c.Assert(err, check.IsNil)
	err = f.be.StartWorkshop(f.ctx, "test-1")
	c.Assert(err, check.IsNil)

	//Validate
	err = f.be.RemoveWorkshop(f.ctx, "test-1")
	c.Assert(err, check.IsNil)
}
