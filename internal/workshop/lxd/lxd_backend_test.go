package lxdbackend_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
)

type LxdBeTests struct {
	project *workshop.Project
}

var _ = check.Suite(&LxdBeTests{})

func TestLxdBackendSuite(t *testing.T) { check.TestingT(t) }

func (s *LxdBeTests) SetUpTest(c *check.C) {
	dir := c.MkDir()
	s.project = &workshop.Project{ProjectId: "42ws42ws", Path: dir}
}

func (f *LxdBeTests) TestProjectSubDirectoryProvideAsPath(c *check.C) {
	root := c.MkDir()
	cases := []struct {
		project   string
		lockFile  bool
		cwd       string
		isSymlink bool
		expected  string
		err       error
	}{
		// nested directory
		{"/home/user", true, "/home/user/nested", false, "/home/user", nil},

		// nested directory
		{"/home/user", true, "/home/user/test/very/deeply", false, "/home/user", nil},

		// same level
		{"/home/user/same", true, "/home/user/same", false, "/home/user/same", nil},
		// same level, symlink
		{"/home/user/same", true, "/home/user/samelink", true, "/home/user/same", nil},

		// different cwd
		{"/home/user/different", true, "/home", false, "/home", nil},

		// project is in root
		{"/", true, "/home/user/notroot", false, "", nil},

		// .lock does not exist
		{"/home/user/nolock", false, "/home/user/test/nolock", false, "/home/user/test/nolock", nil},

		// path is unclean (lock exists)
		{"/home/user/unclean", true, "/home/user/unclean/", false, "/home/user/unclean", nil},

		// path is unclean (no lock)
		{"/home/user/unclean", false, "/home/user/unclean/", false, "/home/user/unclean", nil},

		// path is unclean (no lock, symlink)
		{"/home/user/projectdir", false, "/home/user/symlinktest/", true, "/home/user/projectdir", nil},
	}

	for _, i := range cases {
		os.MkdirAll(filepath.Join(root, i.project), 0755)
		if i.lockFile == true {
			os.Create(workshop.LockPath(filepath.Join(root, i.project)))
		}
		if i.isSymlink == true {
			err := os.Symlink(filepath.Join(root, i.project), filepath.Join(root, i.cwd))
			c.Assert(err, check.IsNil)
		} else {
			os.MkdirAll(filepath.Join(root, i.cwd), 0755)
		}
		// note: no filepath.join here as it calls Clean on exist for the path
		// the data must come unclean for the ProjectPath input and the test
		// must ensure it returns a clean one on every condition
		path, err := lxdbackend.ProjectPath(fmt.Sprintf("%s%s", root, i.cwd))

		c.Assert(path, check.Equals, fmt.Sprintf("%s%s", root, i.expected))
		c.Assert(err, check.Equals, i.err)
		os.RemoveAll(filepath.Join(root, i.project))
		os.RemoveAll(filepath.Join(root, i.cwd))
	}
}

func (f *LxdBeTests) TestReadProjectsSuccess(c *check.C) {
	configData := `[{"path":"/home/dmitry/Work/ros-tutorials","id":"01ac7c0e"},{"path":"/home/dmitry/Work/ros2-tutorials","id":"47b66ebc"}]`

	projects, err := lxdbackend.ReadProjects([]byte(configData))
	c.Assert(err, check.IsNil)
	c.Assert(projects, testutil.DeepUnsortedMatches, []*workshop.Project{
		{
			Path:      "/home/dmitry/Work/ros-tutorials",
			ProjectId: "01ac7c0e",
		},
		{
			Path:      "/home/dmitry/Work/ros2-tutorials",
			ProjectId: "47b66ebc",
		},
	})

	projects, err = lxdbackend.ReadProjects([]byte("[]"))
	c.Assert(err, check.IsNil)
	c.Assert(projects, check.NotNil)
	c.Assert(projects, check.HasLen, 0)

	projects, err = lxdbackend.ReadProjects(nil)
	c.Assert(err, check.IsNil)
	c.Assert(projects, check.NotNil)
	c.Assert(projects, check.HasLen, 0)
}

var marshalledWorkshop = `file-path: /home/user/work/workshop.yaml
name: test
base: ubuntu@22.04
sdks:
    one:
        channel: latest/stable
        plugs:
            one-plug:
                bind: two:two-plug
            one-plug-two:
                bind: two:two-plug
    two:
        channel: latest/edge
`

func (f *LxdBeTests) TestDefaultWorkshopConfig(c *check.C) {
	// Setup
	b := &lxdbackend.Backend{}
	file := &workshop.File{
		Path: "/home/user/work/workshop.yaml",
		Name: "test",
		Base: "ubuntu@22.04",
		Sdks: workshop.SdkList{
			{Name: "one", Channel: "latest/stable", Plugs: map[string]workshop.Plug{
				"one-plug":     {Bind: &workshop.PlugRef{Sdk: "two", Name: "two-plug"}},
				"one-plug-two": {Bind: &workshop.PlugRef{Sdk: "two", Name: "two-plug"}},
			}},
			{Name: "two", Channel: "latest/edge"},
		},
	}

	reset := lxdbackend.MockNvidiaRuntime(func() (bool, error) {
		return true, nil
	})

	// Execute
	cfg, err := lxdbackend.DefaultConfig(b, f.project.ProjectId, "1001", "1001", file)
	defer reset()

	// Validate
	c.Assert(err, check.IsNil)
	c.Assert(cfg["raw.idmap"], check.Equals, "uid 1001 1000\ngid 1001 1000")
	c.Assert(cfg["security.nesting"], check.Equals, "true")
	c.Assert(cfg["user.workshop.project-id"], check.Equals, f.project.ProjectId)

	c.Assert(cfg["nvidia.runtime"], check.Equals, "true")
	c.Assert(cfg["nvidia.driver.capabilities"], check.Equals, "all")
	c.Assert(cfg["user.workshop.file"], check.Equals, marshalledWorkshop)

	// Setup
	reset = lxdbackend.MockNvidiaRuntime(func() (bool, error) {
		return false, nil
	})
	defer reset()

	// Execute
	cfg, err = lxdbackend.DefaultConfig(b, f.project.ProjectId, "1001", "1001", file)

	// Validate
	c.Assert(err, check.IsNil)
	c.Assert(cfg["nvidia.runtime"], check.Equals, "")
	c.Assert(cfg["nvidia.driver.capabilities"], check.Equals, "")
}
