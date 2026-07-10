// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package lxdbackend_test

import (
	"os"
	"path/filepath"
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
    - name: one
      channel: latest/stable
      plugs:
        one-plug:
            bind: two:two-plug
        one-plug-two:
            bind: two:two-plug
    - name: two
      channel: latest/edge
`

func (f *LxdBeTests) TestDefaultWorkshopConfig(c *check.C) {
	// Setup
	b := &lxdbackend.Backend{}
	file := &workshop.File{
		Name: "test",
		Base: "ubuntu@22.04",
		Sdks: []workshop.SdkRecord{
			{Name: "one", Channel: "latest/stable", Plugs: map[string]workshop.PlugOrBind{
				"one-plug":     {Bind: &workshop.PlugRef{Sdk: "two", Name: "two-plug"}},
				"one-plug-two": {Bind: &workshop.PlugRef{Sdk: "two", Name: "two-plug"}},
			}},
			{Name: "two", Channel: "latest/edge"},
		},
	}

	// Execute
	cfg, err := lxdbackend.DefaultConfig(b, f.project.ProjectId, "1001", "1001", file, b.FormatRevision(), "fakeimage12345")

	// Validate
	c.Assert(err, check.IsNil)
	c.Assert(cfg["raw.idmap"], check.Equals, "uid 1001 1000\ngid 1001 1000")
	c.Assert(cfg["security.nesting"], check.Equals, "true")
	c.Assert(cfg["user.workshop.project-id"], check.Equals, f.project.ProjectId)
	c.Assert(cfg["user.workshop.file"], check.Equals, marshalledWorkshop)
	c.Assert(cfg["user.workshop.format-revision"], check.Equals, b.FormatRevision().String())
	c.Assert(cfg["user.workshop.base-fingerprint"], check.Equals, "fakeimage12345")
}

func (f *LxdBeTests) TestCheckLxdVersion(c *check.C) {
	err := lxdbackend.CheckServerVersion("6.8")
	c.Assert(err, check.IsNil)

	err = lxdbackend.CheckServerVersion("6.9")
	c.Assert(err, check.IsNil)

	err = lxdbackend.CheckServerVersion("6.7")
	c.Assert(err, check.ErrorMatches, `(?s).*LXD server version.*is not supported.*`)

	err = lxdbackend.CheckServerVersion("5.9")
	c.Assert(err, check.ErrorMatches, `(?s).*LXD server version.*is not supported.*`)

	err = lxdbackend.CheckServerVersion("unreachable")
	c.Assert(err, check.ErrorMatches, ".*cannot parse LXD server version.*")

	err = lxdbackend.CheckServerVersion("6")
	c.Assert(err, check.ErrorMatches, ".*cannot parse LXD server version.*")

	err = lxdbackend.CheckServerVersion("6.x")
	c.Assert(err, check.ErrorMatches, ".*cannot parse LXD server version.*")

	err = lxdbackend.CheckServerVersion("6.8.1")
	c.Assert(err, check.IsNil)

	err = lxdbackend.CheckServerVersion("6.7.9")
	c.Assert(err, check.ErrorMatches, `(?s).*LXD server version.*is not supported.*`)
}

const zfsTestRelease = "6.8.0-51-generic"

// mockZFSPaths returns paths under base that don't exist yet, so no loaded
// module or module database is detected unless the test creates them.
func mockZFSPaths(base string) (loadedModulePath, controlDevice, modulesDir string) {
	return filepath.Join(base, "sys", "module", "zfs"),
		filepath.Join(base, "dev", "zfs"),
		filepath.Join(base, "lib", "modules")
}

func (f *LxdBeTests) TestZFSUsableModuleLoaded(c *check.C) {
	base := c.MkDir()
	loadedModulePath, controlDevice, modulesDir := mockZFSPaths(base)
	// zfsModuleLoaded requires both the sysfs entry and the control device.
	c.Assert(os.MkdirAll(loadedModulePath, 0755), check.IsNil)
	c.Assert(os.MkdirAll(filepath.Dir(controlDevice), 0755), check.IsNil)
	c.Assert(os.WriteFile(controlDevice, nil, 0644), check.IsNil)

	restore := lxdbackend.MockZFSDetection(loadedModulePath, controlDevice, modulesDir, zfsTestRelease)
	defer restore()

	c.Check(lxdbackend.ZFSUsable(), check.Equals, true)
}

// A single loaded signal on its own doesn't count as loaded: the module's
// sysfs entry without its control device (or vice versa) is not a usable ZFS.
// With no module database present either, such a host falls back to Btrfs.
func (f *LxdBeTests) TestZFSUsablePartialSignalsNotLoaded(c *check.C) {
	for _, t := range []struct {
		desc   string
		create func(loadedModulePath, controlDevice string) error
	}{
		{"only sysfs module dir", func(loadedModulePath, _ string) error {
			return os.MkdirAll(loadedModulePath, 0755)
		}},
		{"only control device", func(_, controlDevice string) error {
			if err := os.MkdirAll(filepath.Dir(controlDevice), 0755); err != nil {
				return err
			}
			return os.WriteFile(controlDevice, nil, 0644)
		}},
	} {
		base := c.MkDir()
		loadedModulePath, controlDevice, modulesDir := mockZFSPaths(base)
		c.Assert(t.create(loadedModulePath, controlDevice), check.IsNil)

		restore := lxdbackend.MockZFSDetection(loadedModulePath, controlDevice, modulesDir, zfsTestRelease)
		c.Check(lxdbackend.ZFSUsable(), check.Equals, false, check.Commentf("%s", t.desc))
		restore()
	}
}

func (f *LxdBeTests) TestZFSUsableLoadable(c *check.C) {
	for _, t := range []struct {
		desc string
		db   string
		body string
	}{
		{"modules.dep, uncompressed", "modules.dep", "kernel/zfs/zfs.ko: kernel/spl/spl.ko\n"},
		{"modules.dep, zstd-compressed", "modules.dep", "updates/dkms/zfs.ko.zst: updates/dkms/spl.ko.zst\n"},
		{"modules.dep, dependency only", "modules.dep", "kernel/foo/foo.ko.zst: kernel/zfs/zfs.ko.zst\n"},
		{"modules.builtin", "modules.builtin", "kernel/zfs/zfs.ko\n"},
	} {
		base := c.MkDir()
		loadedModulePath, controlDevice, modulesDir := mockZFSPaths(base)
		relDir := filepath.Join(modulesDir, zfsTestRelease)
		c.Assert(os.MkdirAll(relDir, 0755), check.IsNil)
		c.Assert(os.WriteFile(filepath.Join(relDir, t.db), []byte(t.body), 0644), check.IsNil)

		restore := lxdbackend.MockZFSDetection(loadedModulePath, controlDevice, modulesDir, zfsTestRelease)
		c.Check(lxdbackend.ZFSUsable(), check.Equals, true, check.Commentf("%s", t.desc))
		restore()
	}
}

func (f *LxdBeTests) TestZFSUsableAbsent(c *check.C) {
	base := c.MkDir()
	loadedModulePath, controlDevice, modulesDir := mockZFSPaths(base)
	relDir := filepath.Join(modulesDir, zfsTestRelease)
	c.Assert(os.MkdirAll(relDir, 0755), check.IsNil)
	// A module database that lists other modules, including names that merely
	// contain "zfs", but not the zfs module itself.
	c.Assert(os.WriteFile(filepath.Join(relDir, "modules.dep"),
		[]byte("kernel/fs/ext4/ext4.ko.zst: kernel/lib/crc16.ko.zst\nkernel/net/notzfs.ko:\nkernel/zfs/zfsutil.ko:\n"),
		0644), check.IsNil)

	restore := lxdbackend.MockZFSDetection(loadedModulePath, controlDevice, modulesDir, zfsTestRelease)
	defer restore()

	c.Check(lxdbackend.ZFSUsable(), check.Equals, false)
}
