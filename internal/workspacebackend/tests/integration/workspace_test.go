//go:build integration
// +build integration

package lxdbackend_integration_test

import (
	"context"
	"os"
	"os/user"
	"path/filepath"

	"github.com/canonical/workspace/client"
	"github.com/canonical/workspace/internal/daemon"
	"github.com/canonical/workspace/internal/testutil"
	"github.com/canonical/workspace/internal/workspacebackend"
	lxd "github.com/lxc/lxd/client"
	"github.com/spf13/afero"
	"gopkg.in/check.v1"
)

type wsOps struct {
	// per suite
	lxdClient lxd.InstanceServer
	be        workspacebackend.WorkspaceBackend

	// per test
	ctx                 context.Context
	username            string
	client              *client.Client
	daemon              *daemon.Daemon
	project             *workspacebackend.Project
	lookupUserRestore   func()
	lookupUserIdRestore func()
	newProjectidRestore func()
}

var _ = check.Suite(&wsOps{})

func (f *wsOps) SetUpTest(c *check.C) {
	socketPath := c.MkDir() + ".workspace.socket"
	f.be = workspacebackend.New()

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

	f.project = &workspacebackend.Project{
		ProjectId: "42424242",
		Path:      c.MkDir(),
	}
	f.username = "testuser"
	f.ctx = createTestContext(f.username, f.project.ProjectId)

	f.lxdClient, _ = f.be.(*workspacebackend.LxdBackend).LxdClient(f.ctx)
	err = workspacebackend.InitProject(f.lxdClient, f.username)
	c.Check(err, check.IsNil)

	f.lookupUserRestore = testutil.FakeFunc(func(name string) (*user.User, error) {
		u := &user.User{
			Name:     f.username,
			Username: f.username,
			Uid:      "1000",
			Gid:      "1000",
		}
		return u, nil
	}, &workspacebackend.LookupUsername)

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
	}, &workspacebackend.NewProjectId)
	launchTestWorkspace(c, f.ctx, f.be, f.project.Path, f.username)
}

func (f *wsOps) TearDownTest(c *check.C) {
	defer f.be.RemoveWorkspace(f.ctx, "test")
	err := f.daemon.Stop(nil)
	c.Check(err, check.IsNil)
	f.lookupUserRestore()
	f.lookupUserIdRestore()
	f.newProjectidRestore()
	err = os.RemoveAll(f.project.Path)
	c.Check(err, check.IsNil)
}

func createTestContext(username, projectId string) context.Context {
	ctx := context.WithValue(context.Background(), workspacebackend.ContextUser, username)
	ctx = context.WithValue(ctx, workspacebackend.ContextProjectId, projectId)
	return ctx
}

func launchTestWorkspace(c *check.C, ctx context.Context, be workspacebackend.WorkspaceBackend, dir, username string) {
	restore := testutil.FakeFunc(func(name string) (*user.User, error) {
		u := &user.User{
			Name:     username,
			Username: username,
			Uid:      "1000",
			Gid:      "1000",
		}
		return u, nil
	}, &workspacebackend.LookupUsername)
	defer restore()

	var err error

	os.WriteFile(filepath.Join(dir, ".workspace.test.yaml"), []byte(`name: test
base: ubuntu@22.04
`), 0644)

	_, _, err = be.CreateOrLoadProject(ctx, dir)
	c.Assert(err, check.IsNil)
	err = be.LaunchWorkspace(ctx, "test", "ubuntu@22.04")
	c.Assert(err, check.IsNil)
	err = be.StartWorkspace(ctx, "test")
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TearDownSuite(c *check.C) {
	cleanUpLxdProject(c, f.lxdClient, workspacebackend.LxdProjectName(f.username))
	cleanUpLxdProject(c, f.lxdClient, workspacebackend.LxdSystemProjectName(f.username))
}

func (f *wsOps) TestLxdBackendTrivialLaunch(c *check.C) {
	// Execute
	err := f.be.LaunchWorkspace(f.ctx, "test-1", "ubuntu@22.04")
	defer f.be.RemoveWorkspace(f.ctx, "test-1")

	//Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.GetWorkspace(f.ctx, "test-1")
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendUnstashWorkspace(c *check.C) {
	// Execute
	err := f.be.StashWorkspace(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.GetWorkspace(f.ctx, "test")
	c.Assert(err, check.NotNil)

	// Execute
	err = f.be.UnstashWorkspace(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.GetWorkspace(f.ctx, "test")
	c.Assert(err, check.IsNil)

}

func (f *wsOps) TestLxdBackendStateStorageVolumeAddRemove(c *check.C) {
	// Execute
	err := f.be.CreateStateStorage(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	vol, _, err := f.lxdClient.GetStoragePoolVolume("default", "custom", workspacebackend.WorkspaceStateVolumeName("test", f.project.ProjectId))
	c.Assert(err, check.IsNil)
	c.Assert(vol.ContentType, check.Equals, "filesystem")

	// Execute
	err = f.be.DeleteStateStorage(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendRemoveWorkspaceStash(c *check.C) {
	// Setup
	err := f.be.LaunchWorkspace(f.ctx, "test-1", "ubuntu@22.04")
	defer f.be.RemoveWorkspace(f.ctx, "test-1")
	c.Assert(err, check.IsNil)

	// Execute
	err = f.be.StashWorkspace(f.ctx, "test-1")

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.GetWorkspace(f.ctx, "test-1")
	c.Assert(err, check.NotNil)

	// Execute
	err = f.be.RemoveWorkspaceStash(f.ctx, "test-1")
	c.Assert(err, check.IsNil)
	cli := f.lxdClient.UseProject(workspacebackend.LxdSystemProjectName(f.username))
	_, _, err = cli.GetInstance(workspacebackend.InstanceName("test-1", f.project.ProjectId))
	c.Assert(err, check.ErrorMatches, "Instance not found")
}

func (f *wsOps) TestLxdBackendStartWorkspace(c *check.C) {
	// Setup
	err := f.be.LaunchWorkspace(f.ctx, "test-1", "ubuntu@22.04")
	c.Assert(err, check.IsNil)
	defer f.be.RemoveWorkspace(f.ctx, "test-1")

	// Execute
	err = f.be.StartWorkspace(f.ctx, "test-1")

	//Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.GetWorkspace(f.ctx, "test-1")
	c.Assert(err, check.IsNil)

	// now, ensure that the systemd is in the final state
	memFs := afero.NewMemMapFs()
	out, _ := memFs.Create("stdout")
	args := workspacebackend.Execution{
		ExecArgs: workspacebackend.ExecArgs{
			UserId:  0,
			GroupId: 0,
			Command: []string{
				"bash", "-eu", "-c",
				"systemctl is-system-running",
			},
			WorkDir: "/",
		},
		ExecControls: workspacebackend.ExecControls{
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

func (f *wsOps) TestLxdBackendDeleteWorkspace(c *check.C) {
	// Execute
	err := f.be.LaunchWorkspace(f.ctx, "test-1", "ubuntu@22.04")
	c.Assert(err, check.IsNil)
	err = f.be.StartWorkspace(f.ctx, "test-1")
	c.Assert(err, check.IsNil)

	//Validate
	err = f.be.RemoveWorkspace(f.ctx, "test-1")
	c.Assert(err, check.IsNil)
}
