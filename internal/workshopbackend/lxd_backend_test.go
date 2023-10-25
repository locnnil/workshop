package workshopbackend_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"
	"github.com/lxc/lxd/shared/api"
	"gopkg.in/check.v1"
)

type LxdBeTests struct {
	project *workshopbackend.Project
}

var _ = check.Suite(&LxdBeTests{})

func TestLxdBackendSuite(t *testing.T) { check.TestingT(t) }

func (s *LxdBeTests) SetUpTest(c *check.C) {
	dir := c.MkDir()
	s.project = &workshopbackend.Project{ProjectId: "42ws42ws", Path: dir}
}

func (s *LxdBeTests) TearDownTest(c *check.C) {
}

func (f *LxdBeTests) TestLoadWorkshopSuccess(c *check.C) {
	// Setup
	os.WriteFile(filepath.Join(f.project.Path, ".workshop.ws.yaml"), []byte(`name: ws
base: ubuntu@20.04
sdks:
  go:
    channel: latest/stable
`), 0644)
	project := workshopbackend.Project{ProjectId: f.project.ProjectId, Path: f.project.Path}

	b := &workshopbackend.LxdBackend{}

	// Execute
	ws, err := workshopbackend.LoadWorkshop(b, &api.Instance{
		Name: workshopbackend.InstanceName("ws", f.project.ProjectId),
		InstancePut: api.InstancePut{Config: map[string]string{
			"user.workshop.project-id": f.project.ProjectId,
			"user.workshop.content":    `{"go":{"name":"go","channel":"latest/stable","revision":277}}`,
		}},
		StatusCode: api.Running,
	}, &project)

	// Validate
	c.Assert(err, check.IsNil)
	c.Assert(ws.Name, check.Equals, "ws")
	c.Assert(ws.IsRunning(), check.Equals, true)
	c.Assert(ws.Errors(), check.HasLen, 0)
	c.Assert(ws.ProjectId(), check.Equals, f.project.ProjectId)
	c.Assert(ws.Content(), testutil.DeepUnsortedMatches, []sdk.Setup{{
		Name:     "go",
		Channel:  "latest/stable",
		Revision: 277,
	},
	})
}

func (f *LxdBeTests) TestLoadWorkshopMissingErrors(c *check.C) {
	// Setup
	project := workshopbackend.Project{ProjectId: f.project.ProjectId, Path: f.project.Path}

	b := &workshopbackend.LxdBackend{}

	// Execute
	ws, err := workshopbackend.LoadWorkshop(b, &api.Instance{
		Name: workshopbackend.InstanceName("ws", f.project.ProjectId),
		InstancePut: api.InstancePut{Config: map[string]string{
			"user.workshop.project-id": f.project.ProjectId,
		},
		}}, &project)

	// Validate
	c.Assert(err, check.IsNil)
	c.Assert(ws.Name, check.Equals, "ws")
	c.Assert(ws.Errors(), testutil.Contains, workshopbackend.MissingFile)
	c.Assert(ws.Errors(), check.HasLen, 1)

	// Setup
	os.RemoveAll(f.project.Path)

	// Execute
	ws, err = workshopbackend.LoadWorkshop(b, &api.Instance{
		Name: workshopbackend.InstanceName("ws", f.project.ProjectId),
		InstancePut: api.InstancePut{Config: map[string]string{
			"user.workshop.project-id": f.project.ProjectId,
		},
		}}, &project)

	// Validate
	c.Assert(err, check.IsNil)
	c.Assert(ws.Errors(), testutil.Contains, workshopbackend.MissingFile)
	c.Assert(ws.Errors(), testutil.Contains, workshopbackend.MissingProject)
	c.Assert(ws.Errors(), check.HasLen, 2)
}

func (f *LxdBeTests) TestLxdBackendMergeFilesAndInstances(c *check.C) {
	// Setup
	os.WriteFile(filepath.Join(f.project.Path, ".workshop.t1.yaml"), []byte(`name: t1
base: ubuntu@20.04`), 0644)
	os.WriteFile(filepath.Join(f.project.Path, ".workshop.t2.yaml"), []byte(`name: t2
base: ubuntu@20.04`), 0644)
	files, err := f.project.EnumWorkshopFiles()
	c.Assert(err, check.IsNil)

	instances := []*workshopbackend.Workshop{
		{
			Name: "t1",
		},
		{
			Name: "t2",
		},
	}

	instances[0].SetRunning(true)
	instances[1].SetRunning(true)

	// Execute
	_, merged := workshopbackend.MergeInstancesAndFiles(files, instances)

	// Validate
	c.Assert(merged, check.HasLen, 2)
	c.Assert(merged[0].IsRunning(), check.Equals, true)
	c.Assert(merged[1].IsRunning(), check.Equals, true)
	c.Assert(merged[0].Errors(), check.HasLen, 0)
	c.Assert(merged[1].Errors(), check.HasLen, 0)
}

func (f *LxdBeTests) TestLxdBackendMergeFilesAndInstancesWorkshopOff(c *check.C) {
	// Setup

	os.WriteFile(filepath.Join(f.project.Path, ".workshop.t1.yaml"), []byte(`name: t1
base: ubuntu@20.04`), 0644)
	os.WriteFile(filepath.Join(f.project.Path, ".workshop.t2.yaml"), []byte(`name: t2
base: ubuntu@20.04`), 0644)

	files, err := f.project.EnumWorkshopFiles()
	c.Assert(err, check.IsNil)

	instances := []*workshopbackend.Workshop{
		{
			Name: "t1",
		},
	}

	instances[0].SetRunning(true)

	// Execute
	wsFiles, merged := workshopbackend.MergeInstancesAndFiles(files, instances)

	// Validate
	c.Assert(merged, check.HasLen, 1)
	c.Assert(wsFiles, check.HasLen, 1)

	c.Assert(merged[0].IsRunning(), check.Equals, true)
	c.Assert(merged[0].Errors(), check.HasLen, 0)
}

func (f *LxdBeTests) TestProjectSubDirectoryProvideAsPath(c *check.C) {
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
			os.Create(workshopbackend.LockPath(filepath.Join(root, i.project)))
		}

		path, err := workshopbackend.ProjectPath(filepath.Join(root, i.cwd))

		c.Assert(path, check.Equals, filepath.Join(root, i.expected))
		c.Assert(err, check.Equals, i.err)
		os.RemoveAll(filepath.Join(root, i.project))
		os.RemoveAll(filepath.Join(root, i.cwd))
	}
}

func (f *LxdBeTests) TestReadProjectsSuccess(c *check.C) {
	configData := `[{"path":"/home/dmitry/Work/ros-tutorials","id":"01ac7c0e"},{"path":"/home/dmitry/Work/ros2-tutorials","id":"47b66ebc"}]`

	projects, err := workshopbackend.ReadProjects([]byte(configData))
	c.Assert(err, check.IsNil)
	c.Assert(projects, testutil.DeepUnsortedMatches, []*workshopbackend.Project{
		{
			Path:      "/home/dmitry/Work/ros-tutorials",
			ProjectId: "01ac7c0e",
		},
		{
			Path:      "/home/dmitry/Work/ros2-tutorials",
			ProjectId: "47b66ebc",
		},
	})

	projects, err = workshopbackend.ReadProjects([]byte("[]"))
	c.Assert(err, check.IsNil)
	c.Assert(projects, check.NotNil)
	c.Assert(projects, check.HasLen, 0)

	projects, err = workshopbackend.ReadProjects(nil)
	c.Assert(err, check.IsNil)
	c.Assert(projects, check.NotNil)
	c.Assert(projects, check.HasLen, 0)
}
