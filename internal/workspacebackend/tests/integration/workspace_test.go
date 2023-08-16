//go:build integration
// +build integration

package lxdbackend_integration_test

import (
	"context"
	"os"
	"os/user"
	"path/filepath"

	"github.com/canonical/workspace/internal/testutil"
	"github.com/canonical/workspace/internal/workspacebackend"
	lxd "github.com/lxc/lxd/client"
	"github.com/spf13/afero"
	"gopkg.in/check.v1"
)

type WsOps struct {
	ctx      context.Context
	client   lxd.InstanceServer
	username string
	be       workspacebackend.WorkspaceBackend
	project  *workspacebackend.Project
}

var _ = check.Suite(&WsOps{})

func (f *WsOps) SetUpSuite(c *check.C) {
	f.username = "testuser"
	testutil.FakeFunc(func(name string) (*user.User, error) {
		u := &user.User{
			Name:     f.username,
			Username: f.username,
			Uid:      "1000",
			Gid:      "1000",
		}
		return u, nil
	}, &workspacebackend.UserLookup)

	f.be = &workspacebackend.LxdBackend{}
	projectDir := c.MkDir()
	workspace := `name: test
base: ubuntu@22.04
`
	os.WriteFile(filepath.Join(projectDir, ".workspace.test.yaml"), []byte(workspace), 0644)

	ctx := context.WithValue(context.Background(), workspacebackend.ContextUser, f.username)
	var err error
	f.project, _, err = f.be.CreateOrLoadProject(ctx, projectDir)
	c.Assert(err, check.IsNil)

	f.ctx = context.WithValue(ctx, workspacebackend.ContextProjectId, f.project.ProjectId)

	f.client, _ = f.be.(*workspacebackend.LxdBackend).LxdClient(f.ctx)

	workspacebackend.InitProject(f.client, f.username)

	err = f.be.LaunchWorkspace(f.ctx, "test", "ubuntu@22.04")
	c.Assert(err, check.IsNil)
}

func (f *WsOps) TearDownSuite(c *check.C) {
	cleanUpLxdProject(c, f.client, workspacebackend.LxdProjectName(f.username))
	cleanUpLxdProject(c, f.client, workspacebackend.LxdSystemProjectName(f.username))
}

func (f *WsOps) TestLxdBackendTrivialLaunch(c *check.C) {
	// Execute
	err := f.be.LaunchWorkspace(f.ctx, "test-1", "ubuntu@22.04")
	defer f.be.DeleteWorkspace(f.ctx, "test-1")

	//Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.GetWorkspace(f.ctx, "test-1")
	c.Assert(err, check.IsNil)
}

func (f *WsOps) TestLxdBackendUnstashWorkspace(c *check.C) {
	// Setup

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

func (f *WsOps) TestLxdBackendStateStorageVolumeAddRemove(c *check.C) {
	// Setup

	// Execute
	err := f.be.CreateStateStorage(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	vol, _, err := f.client.GetStoragePoolVolume("default", "custom", workspacebackend.WorkspaceStateVolumeName("test", f.project.ProjectId))
	c.Assert(err, check.IsNil)
	c.Assert(vol.ContentType, check.Equals, "filesystem")

	// Execute
	err = f.be.DeleteStateStorage(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
}

func (f *WsOps) TestLxdBackendRemoveWorkspaceStash(c *check.C) {
	// Setup
	err := f.be.LaunchWorkspace(f.ctx, "test-1", "ubuntu@22.04")
	defer f.be.DeleteWorkspace(f.ctx, "test-1")
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
	cli := f.client.UseProject(workspacebackend.LxdSystemProjectName(f.username))
	_, _, err = cli.GetInstance(workspacebackend.InstanceName("test-1", f.project.ProjectId))
	c.Assert(err, check.ErrorMatches, "Instance not found")
}

func (f *WsOps) TestLxdBackendStartWorkspace(c *check.C) {
	// Setup
	err := f.be.LaunchWorkspace(f.ctx, "test-1", "ubuntu@20.04")
	c.Assert(err, check.IsNil)
	defer f.be.DeleteWorkspace(f.ctx, "test-1")

	// Execute
	err = f.be.StartWorkspace(f.ctx, "test-1")

	//Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.GetWorkspace(f.ctx, "test-1")
	c.Assert(err, check.IsNil)

	// now, ensure that the systemd is in the final state
	memFs := afero.NewMemMapFs()
	out, _ := memFs.Create("stdout")
	args := workspacebackend.ExecArgs{
		User: "root",
		Command: []string{
			"bash", "-eu", "-c",
			"systemctl is-system-running 2>/dev/null",
		},
		WorkDir: "/",
		Stdin:   nil,
		Stdout:  out,
		Stderr:  out}

	err = f.be.Exec(f.ctx, "test-1", &args)
	c.Assert(err, check.IsNil)
	buf, err := afero.ReadFile(memFs, out.Name())
	c.Assert(err, check.IsNil)
	c.Assert(string(buf), check.Equals, "running\n")
}

func (f *WsOps) TestLxdBackendDeleteWorkspace(c *check.C) {
	// Execute
	err := f.be.LaunchWorkspace(f.ctx, "test-1", "ubuntu@22.04")
	c.Assert(err, check.IsNil)
	err = f.be.StartWorkspace(f.ctx, "test-1")
	c.Assert(err, check.IsNil)

	//Validate
	err = f.be.DeleteWorkspace(f.ctx, "test-1")
	c.Assert(err, check.IsNil)
}
