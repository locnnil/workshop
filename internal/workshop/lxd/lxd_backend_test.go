package lxdbackend_test

import (
	"testing"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
)

type LxdBeTests struct {
	project workshop.Project
}

var _ = check.Suite(&LxdBeTests{})

func TestLxdBackendSuite(t *testing.T) { check.TestingT(t) }

func (s *LxdBeTests) SetUpTest(c *check.C) {
	dir := c.MkDir()
	s.project = workshop.Project{ProjectId: "42ws42ws", Path: dir}
}

func (f *LxdBeTests) TestReadProjectsSuccess(c *check.C) {
	configData := `[{"path":"/home/dmitry/Work/ros-tutorials","id":"01ac7c0e"},{"path":"/home/dmitry/Work/ros2-tutorials","id":"47b66ebc"}]`

	projects, err := lxdbackend.ReadProjects([]byte(configData))
	c.Assert(err, check.IsNil)
	c.Assert(projects, testutil.DeepUnsortedMatches, []workshop.Project{
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

var marshalledWorkshop = `name: test
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
