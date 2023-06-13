package workspacebackend_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/workspace/internal/workspacebackend"
	"gopkg.in/check.v1"
)

type LxdBeTests struct {
}

var _ = check.Suite(&LxdBeTests{})

func TestLxdBackend(t *testing.T) { check.TestingT(t) }

func (f *LxdBeTests) TestLxdBackendMergeFilesAndInstances(c *check.C) {
	// Setup
	dir := c.MkDir()
	project := workspacebackend.Project{ProjectId: "42ws42ws", Path: dir}

	os.WriteFile(filepath.Join(dir, ".workspace.t1.yaml"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, ".workspace.t2.yaml"), []byte(""), 0644)
	files, err := project.EnumWorkspaceFiles()
	c.Assert(err, check.IsNil)

	instances := []*workspacebackend.WorkspaceProps{
		{
			Name: "t1",
			Devices: map[string]map[string]string{
				workspacebackend.ProjectPathDevice: {"type": "disk", "pool": "default", "source": dir},
			},
		},
		{
			Name: "t2",
			Devices: map[string]map[string]string{
				workspacebackend.ProjectPathDevice: {"type": "disk", "pool": "default", "source": dir},
			},
		},
	}

	instances[0].SetState(workspacebackend.Ready, workspacebackend.None)
	instances[1].SetState(workspacebackend.Ready, workspacebackend.None)

	// Execute
	merged := workspacebackend.MergeInstancesAndFiles(files, instances)

	// Validate
	c.Assert(merged, check.HasLen, 2)
	c.Assert(merged[0].State(), check.Equals, workspacebackend.Ready)
	c.Assert(merged[1].State(), check.Equals, workspacebackend.Ready)
	c.Assert(merged[0].Reason(), check.Equals, workspacebackend.None)
	c.Assert(merged[1].Reason(), check.Equals, workspacebackend.None)
}

func (f *LxdBeTests) TestLxdBackendMergeFilesAndInstancesMissingFile(c *check.C) {
	// Setup
	dir := c.MkDir()
	project := workspacebackend.Project{ProjectId: "42ws42ws", Path: dir}

	os.WriteFile(filepath.Join(dir, ".workspace.t1.yaml"), []byte(""), 0644)
	files, err := project.EnumWorkspaceFiles()
	c.Assert(err, check.IsNil)

	instances := []*workspacebackend.WorkspaceProps{
		{
			Name: "t1",
			Devices: map[string]map[string]string{
				workspacebackend.ProjectPathDevice: {"type": "disk", "pool": "default", "source": dir},
			},
		},
		{
			Name: "t2",
			Devices: map[string]map[string]string{
				workspacebackend.ProjectPathDevice: {"type": "disk", "pool": "default", "source": dir},
			},
		},
	}

	instances[0].SetState(workspacebackend.Ready, workspacebackend.None)
	instances[1].SetState(workspacebackend.Ready, workspacebackend.None)

	// Execute
	merged := workspacebackend.MergeInstancesAndFiles(files, instances)

	// Validate
	c.Assert(merged, check.HasLen, 2)
	c.Assert(merged[0].State(), check.Equals, workspacebackend.Ready)
	c.Assert(merged[0].Reason(), check.Equals, workspacebackend.None)
	c.Assert(merged[1].State(), check.Equals, workspacebackend.Error)
	c.Assert(merged[1].Reason(), check.Equals, workspacebackend.MissingFile)
}

func (f *LxdBeTests) TestLxdBackendMergeFilesAndInstancesOffWorkspace(c *check.C) {
	// Setup
	dir := c.MkDir()
	project := workspacebackend.Project{ProjectId: "42ws42ws", Path: dir}

	os.WriteFile(filepath.Join(dir, ".workspace.t1.yaml"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, ".workspace.t2.yaml"), []byte(""), 0644)

	files, err := project.EnumWorkspaceFiles()
	c.Assert(err, check.IsNil)

	instances := []*workspacebackend.WorkspaceProps{
		{
			Name: "t1",
			Devices: map[string]map[string]string{
				workspacebackend.ProjectPathDevice: {"type": "disk", "pool": "default", "source": dir},
			},
		},
	}

	instances[0].SetState(workspacebackend.Ready, workspacebackend.None)

	// Execute
	merged := workspacebackend.MergeInstancesAndFiles(files, instances)

	// Validate
	c.Assert(merged, check.HasLen, 2)
	c.Assert(merged[0].State(), check.Equals, workspacebackend.Ready)
	c.Assert(merged[1].State(), check.Equals, workspacebackend.Off)
	c.Assert(merged[0].Reason(), check.Equals, workspacebackend.None)
	c.Assert(merged[1].Reason(), check.Equals, workspacebackend.None)
}

func (f *LxdBeTests) TestLxdBackendMergeFilesAndInstancesMissingProject(c *check.C) {
	// Setup
	dir := "/does/not/exist"
	project := workspacebackend.Project{ProjectId: "42ws42ws", Path: dir}

	files, err := project.EnumWorkspaceFiles()
	c.Assert(err, check.NotNil)

	instances := []*workspacebackend.WorkspaceProps{
		{
			Name: "t1",
			Devices: map[string]map[string]string{
				workspacebackend.ProjectPathDevice: {"type": "disk", "pool": "default", "source": dir},
			},
		},
	}

	instances[0].SetState(workspacebackend.Ready, workspacebackend.None)

	// Execute
	merged := workspacebackend.MergeInstancesAndFiles(files, instances)

	// Validate
	c.Assert(merged, check.HasLen, 1)
	c.Assert(merged[0].State(), check.Equals, workspacebackend.Error)
	c.Assert(merged[0].Reason(), check.Equals, workspacebackend.MissingProject)
}

func (f *LxdBeTests) TestProjectDirectory(c *check.C) {
	root := c.MkDir()
	cases := []struct {
		project  string
		lockFile bool
		cwd      string
		expected string
		err      error
	}{

		// nested directory
		{"/home/user/", true, "/home/user/nested", "/home/user", nil},

		// nested directory
		{"/home/user/", true, "/home/user/test/very/deeply", "/home/user", nil},

		// same level
		{"/home/user/same", true, "/home/user/same", "/home/user/same", nil},

		// different cwd
		{"/home/user/different", true, "/home", "/home", nil},

		// project is in root
		{"/", true, "/home/user/notroot", "/", nil},

		// .lock does not exist
		{"/home/user/nolock", false, "/home/user/test/nolock", "/home/user/test/nolock", nil},
	}

	for _, i := range cases {
		os.MkdirAll(filepath.Join(root, i.project), 0755)
		os.MkdirAll(filepath.Join(root, i.cwd), 0755)
		if i.lockFile == true {
			os.Create(workspacebackend.LockPath(filepath.Join(root, i.project)))
		}

		path, err := workspacebackend.ProjectPath(filepath.Join(root, i.cwd))

		c.Assert(path, check.Equals, filepath.Join(root, i.expected))
		c.Assert(err, check.Equals, i.err)
		os.RemoveAll(filepath.Join(root, i.project))
		os.RemoveAll(filepath.Join(root, i.cwd))
	}
}
