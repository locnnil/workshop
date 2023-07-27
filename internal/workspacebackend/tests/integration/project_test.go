//go:build integration
// +build integration

package lxdbackend_integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/check.v1"

	"github.com/canonical/workspace/internal/testutil"
	"github.com/canonical/workspace/internal/workspacebackend"
	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
)

type LT struct {
	ctx      context.Context
	client   lxd.InstanceServer
	username string
}

var _ = check.Suite(&LT{})

func (f *LT) SetUpTest(c *check.C) {
	f.username = "testuser"
	f.ctx = context.WithValue(context.Background(), workspacebackend.ContextUser, f.username)
	be := workspacebackend.LxdBackend{}
	f.client, _ = be.LxdClient(f.ctx)
	err := workspacebackend.InitProject(f.client, f.username)
	c.Assert(err, check.IsNil)
}

func cleanUpLxdProject(c *check.C, client lxd.InstanceServer, project string) {
	cli := client.UseProject(project)
	fingers, err := cli.GetImageFingerprints()
	c.Check(err, check.IsNil)
	for _, i := range fingers {
		op, err := cli.DeleteImage(i)
		c.Assert(err, check.IsNil)
		c.Assert(op.Wait(), check.IsNil)
	}

	instances, err := cli.GetInstances(api.InstanceType("container"))
	c.Check(err, check.IsNil)
	for _, i := range instances {
		req := api.InstanceStatePut{
			Action:  "stop",
			Timeout: 1,
			Force:   true,
		}

		op, err := cli.UpdateInstanceState(i.Name, req, "")
		c.Assert(err, check.IsNil)
		c.Assert(op.Wait(), check.IsNil)

		op, err = cli.DeleteInstance(i.Name)
		c.Assert(err, check.IsNil)
		c.Assert(op.Wait(), check.IsNil)
	}

	err = cli.DeleteProject(project)
	c.Assert(err, check.IsNil)
}

func (f *LT) TearDownTest(c *check.C) {
	cleanUpLxdProject(c, f.client, workspacebackend.LxdProjectName(f.username))
}

func TestWorkspaceBackendProjectIntegration(t *testing.T) { check.TestingT(t) }

func (f *LT) TestLxdBackendCreateProjectNoWorkspaceFiles(c *check.C) {
	// Setup
	be := workspacebackend.LxdBackend{}

	projectDir := c.MkDir()

	// Execute
	prj, _, err := be.CreateOrLoadProject(f.ctx, projectDir)

	// Validate
	c.Assert(prj, check.IsNil)
	c.Assert(err, check.Equals, workspacebackend.ErrNotAProject)
	c.Assert(workspacebackend.LockPath(projectDir), testutil.FileAbsent)
	projects, _ := be.Projects(f.ctx)
	c.Assert(projects, check.HasLen, 0)
}

func (f *LT) TestLxdBackendCreateProject(c *check.C) {
	// Setup
	be := workspacebackend.LxdBackend{}
	numCalls := 0
	ids := []string{"b8639dea", "d4352dea"}
	restore := testutil.FakeFunc(func() (string, error) { numCalls = numCalls + 1; return ids[numCalls-1], nil }, &workspacebackend.NewProjectId)
	defer restore()
	projectDir, projectDir2 := c.MkDir(), c.MkDir()
	workspace := `name: test
base: ubuntu@22.04
`
	os.WriteFile(filepath.Join(projectDir, ".workspace.test.yaml"), []byte(workspace), 0644)
	os.WriteFile(filepath.Join(projectDir2, ".workspace.test.yaml"), []byte(workspace), 0644)

	// Execute
	prj, _, err := be.CreateOrLoadProject(f.ctx, projectDir)

	// Validate
	c.Assert(prj, check.NotNil)
	c.Assert(prj.Path, check.Equals, projectDir)
	c.Assert(err, check.IsNil)

	lxdProject, _, _ := f.client.GetProject(workspacebackend.LxdProjectName("testuser"))
	c.Assert(workspacebackend.LockPath(projectDir), testutil.FilePresent)
	c.Assert(lxdProject.Config["user.workspace.projects"], check.DeepEquals, fmt.Sprintf(`{"b8639dea":{"path":"%s","id":"b8639dea"}}`, projectDir))

	// Execute
	prj, _, err = be.CreateOrLoadProject(f.ctx, projectDir2)
	c.Assert(prj, check.NotNil)
	c.Assert(err, check.IsNil)

	// Validate
	lxdProject, _, _ = f.client.GetProject(workspacebackend.LxdProjectName("testuser"))
	c.Assert(workspacebackend.LockPath(projectDir2), testutil.FilePresent)
	c.Assert(lxdProject.Config["user.workspace.projects"], check.DeepEquals, fmt.Sprintf(`{"b8639dea":{"path":"%s","id":"b8639dea"},"d4352dea":{"path":"%s","id":"d4352dea"}}`, projectDir, projectDir2))
}

func (f *LT) TestLxdBackendLoadProject(c *check.C) {
	// Setup
	be := workspacebackend.LxdBackend{}
	restore := testutil.FakeFunc(func() (string, error) { return "b8639dea", nil }, &workspacebackend.NewProjectId)
	projectDir := c.MkDir()
	workspace := `name: test
base: ubuntu@22.04
`
	os.WriteFile(filepath.Join(projectDir, ".workspace.test.yaml"), []byte(workspace), 0644)
	prj, _, err := be.CreateOrLoadProject(f.ctx, projectDir)
	c.Assert(prj, check.NotNil)
	c.Assert(prj.Path, check.Equals, projectDir)
	c.Assert(err, check.IsNil)
	// restore the new project id generator, we won't need it anymore
	// as we will be loading the project
	restore()

	// Execute (this time the project must be loaded)
	prj, created, err := be.CreateOrLoadProject(f.ctx, projectDir)

	// Validate
	c.Assert(prj, check.NotNil)
	c.Assert(prj.Path, check.Equals, projectDir)
	c.Assert(err, check.IsNil)
	c.Assert(created, check.Equals, false)
	lxdProject, _, _ := f.client.GetProject(workspacebackend.LxdProjectName("testuser"))

	c.Assert(lxdProject.Config["user.workspace.projects"], check.DeepEquals, fmt.Sprintf(`{"b8639dea":{"path":"%s","id":"b8639dea"}}`, projectDir))
}

func (f *LT) TestLxdBackendLoadProjectDirectoryMoved(c *check.C) {
	// Setup
	// We pre-create a project to emulate the scenario when
	// the directory was moved, but the project's settings were not
	// yet updated.
	be := workspacebackend.LxdBackend{}
	projectDir := c.MkDir()
	newDir := projectDir + "_moved"
	f.client.UpdateProject(workspacebackend.LxdProjectName(f.username),
		api.ProjectPut{
			Config: map[string]string{
				"user.workspace.projects": fmt.Sprintf(`{"b8639dea":{"path":"%s","id":"b8639dea"}}`, projectDir),
			},
		}, "")

	workspace := `name: test
base: ubuntu@22.04
`
	os.WriteFile(filepath.Join(projectDir, ".workspace.test.yaml"), []byte(workspace), 0644)
	os.WriteFile(filepath.Join(projectDir, ".workspace.lock"), []byte("b8639dea"), 0644)
	err := os.Rename(projectDir, newDir)
	c.Assert(err, check.IsNil)

	prj, created, err := be.CreateOrLoadProject(f.ctx, newDir)
	c.Assert(prj, check.NotNil)
	c.Assert(prj.Path, check.Equals, newDir)
	c.Assert(err, check.IsNil)
	c.Assert(created, check.Equals, false)
	lxdProject, _, _ := f.client.GetProject(workspacebackend.LxdProjectName(f.username))
	c.Assert(lxdProject.Config["user.workspace.projects"], check.DeepEquals, fmt.Sprintf(`{"b8639dea":{"path":"%s","id":"b8639dea"}}`, newDir))
}

func (f *LT) TestLxdBackendLoadProjectDirectoryCopied(c *check.C) {
	// Setup
	// We pre-create a project to emulate the scenario when
	// the directory was copied, but the project's settings were not
	// yet updated.
	be := workspacebackend.LxdBackend{}
	restore := testutil.FakeFunc(func() (string, error) { return "abcdefgi", nil }, &workspacebackend.NewProjectId)
	defer restore()
	projectDir := c.MkDir()
	newDir := c.MkDir()
	f.client.UpdateProject(workspacebackend.LxdProjectName(f.username),
		api.ProjectPut{
			Config: map[string]string{
				"user.workspace.projects": fmt.Sprintf(`{"b8639dea":{"path":"%s","id":"b8639dea"}}`, projectDir),
			},
		}, "")

	workspace := `name: test
base: ubuntu@22.04
`
	os.WriteFile(filepath.Join(projectDir, ".workspace.test.yaml"), []byte(workspace), 0644)
	os.WriteFile(filepath.Join(newDir, ".workspace.test.yaml"), []byte(workspace), 0644)
	os.WriteFile(filepath.Join(projectDir, ".workspace.lock"), []byte("b8639dea"), 0644)
	os.WriteFile(filepath.Join(newDir, ".workspace.lock"), []byte("b8639dea"), 0644)

	prj, created, err := be.CreateOrLoadProject(f.ctx, newDir)
	c.Assert(prj, check.NotNil)
	c.Assert(prj.Path, check.Equals, newDir)
	c.Assert(err, check.IsNil)
	c.Assert(created, check.Equals, true)
	c.Assert(filepath.Join(newDir, ".workspace.lock"), testutil.FileEquals, "abcdefgi")
	lxdProject, _, _ := f.client.GetProject(workspacebackend.LxdProjectName(f.username))
	c.Assert(lxdProject.Config["user.workspace.projects"], check.Matches, fmt.Sprintf(`.*"abcdefgi":{"path":"%s","id":"abcdefgi"}.*`, newDir))
}

func (f *LT) TestLxdBackendListAvailableProjects(c *check.C) {
	// Setup
	be := workspacebackend.LxdBackend{}
	numCalls := 0
	ids := []string{"b8639dea", "d4352dea"}
	restore := testutil.FakeFunc(func() (string, error) { numCalls = numCalls + 1; return ids[numCalls-1], nil }, &workspacebackend.NewProjectId)
	defer restore()
	projectDir, projectDir2 := c.MkDir(), c.MkDir()
	workspace := `name: test
base: ubuntu@22.04
`
	os.WriteFile(filepath.Join(projectDir, ".workspace.test.yaml"), []byte(workspace), 0644)
	os.WriteFile(filepath.Join(projectDir2, ".workspace.test.yaml"), []byte(workspace), 0644)

	prj, _, err := be.CreateOrLoadProject(f.ctx, projectDir)
	c.Assert(prj, check.NotNil)
	c.Assert(err, check.IsNil)
	prj, _, err = be.CreateOrLoadProject(f.ctx, projectDir2)
	c.Assert(prj, check.NotNil)
	c.Assert(err, check.IsNil)

	// Execute
	projects, err := be.Projects(f.ctx)

	// Validate
	c.Assert(err, check.IsNil)
	c.Assert(projects, check.DeepEquals, map[string]*workspacebackend.Project{
		"b8639dea": {ProjectId: "b8639dea", Path: projectDir},
		"d4352dea": {ProjectId: "d4352dea", Path: projectDir2},
	})
	c.Assert(workspacebackend.LockPath(projectDir), testutil.FilePresent)
	c.Assert(workspacebackend.LockPath(projectDir2), testutil.FilePresent)
}
