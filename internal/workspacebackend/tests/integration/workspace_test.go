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

func (f *WsOps) TestLxdBackendMakeWorkspaceAvailable(c *check.C) {
	// Setup

	// Execute
	err := f.be.MakeWorkspaceUnavailable(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.GetWorkspace(f.ctx, "test")
	c.Assert(err, check.NotNil)

	// Execute
	err = f.be.MakeWorkspaceAvailable(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.GetWorkspace(f.ctx, "test")
	c.Assert(err, check.IsNil)

}
