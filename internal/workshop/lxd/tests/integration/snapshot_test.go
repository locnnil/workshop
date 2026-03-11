//go:build integration

package lxdbackend_integration_test

import (
	"context"
	"crypto/sha3"
	_ "embed"
	"encoding/hex"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"gopkg.in/check.v1"
	"gopkg.in/yaml.v3"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
	"github.com/canonical/workshop/internal/workshop/lxd/tests/helper"
)

type snapshotSuite struct {
	usr     *user.User
	project workshop.Project
	ctx     context.Context

	restoreLookupUsr   func()
	restoreUserEnv     func()
	restoreImageServer func()

	bd *lxdbackend.Backend
}

var _ = check.Suite(&snapshotSuite{})

func (s *snapshotSuite) SetUpSuite(c *check.C) {
	s.usr = &user.User{Username: "testuser", Uid: "1000", Gid: "1000", HomeDir: c.MkDir()}
	s.project = workshop.Project{
		ProjectId: "42424242",
		Path:      c.MkDir(),
	}
	s.ctx = helper.CreateTestContext(s.usr.Username, s.project.ProjectId)

	s.restoreLookupUsr = osutil.FakeUserLookup(func(name string) (*user.User, error) {
		return s.usr, nil
	})
	s.restoreUserEnv = osutil.FakeUserEnvironment(func(user *user.User) (map[string]string, error) {
		return nil, nil
	})
	s.restoreImageServer = lxdbackend.FakeImageServer(helper.MinimalImageServer)

	dirs.SetRootDir(c.MkDir())
	dirs.SetCacheDir(c.MkDir())
	dirs.ExecDir = c.MkDir()
	dirs.SocketPath = filepath.Join(dirs.BaseDir, "workshop.socket")
	c.Assert(dirs.CreateDirs(), check.IsNil)
	c.Assert(os.MkdirAll(workshop.AptCacheDir(s.project.ProjectId, "test"), 0755), check.IsNil)
	c.Assert(os.WriteFile(filepath.Join(dirs.ExecDir, "workshopctl"), nil, 0644), check.IsNil)

	var err error
	s.bd, err = lxdbackend.New()
	c.Assert(err, check.IsNil)

	conn, err := s.bd.LxdClient(s.ctx)
	c.Assert(err, check.IsNil)
	defer conn.Disconnect()
}

func (s *snapshotSuite) TearDownSuite(c *check.C) {
	conn, err := s.bd.LxdClient(s.ctx)
	c.Check(err, check.IsNil)
	defer conn.Disconnect()

	helper.CleanupLxdProject(c, conn, "workshop."+s.usr.Username)
	helper.CleanupLxdProject(c, conn, "workshop-snapshots."+s.usr.Username)

	s.restoreLookupUsr()
	s.restoreUserEnv()
	s.restoreImageServer()
}

//go:embed snapshot-format.yaml
var snapshotFormat []byte

// Attempt to specify the filesystem layout of a snapshot. Changes to this may
// invalidate snapshots of existing workshops, so the snapshot format revision
// number should be bumped to force a full refresh. This test is mainly
// concerned with workshop config and devices. While these aren't generally
// copied to snapshots, they can influence the filesystem before the snapshot
// is taken (e.g. cloud-config). Direct changes to the filesystem and other
// backend-agnostic conventions are covered by `apiSuite.TestSnapshotFormat`.
func (s *snapshotSuite) TestLxdBackendSnapshotFormat(c *check.C) {
	var format map[string]any
	err := yaml.Unmarshal(snapshotFormat, &format)
	c.Assert(err, check.IsNil)

	// Launch workshop.
	image, err := s.bd.GetBase(s.ctx, "ubuntu@24.04")
	c.Assert(err, check.IsNil)
	err = s.bd.DownloadBase(s.ctx, image, nil)
	c.Assert(err, check.IsNil)
	wf := &workshop.File{
		Name: "test",
		Base: "ubuntu@24.04",
		Sdks: []workshop.SdkRecord{
			{Name: "store-sdk", Channel: "latest/stable"},
			{Name: "local-sdk", Source: sdk.ProjectSource},
		},
	}
	snapshot := workshop.Snapshot{Image: image}
	err = s.bd.LaunchOrRebuildWorkshop(s.ctx, wf, snapshot)
	c.Assert(err, check.IsNil)
	defer helper.RemoveTestWorkshop(c, s.ctx, s.bd)

	// Validate post-launch metadata.
	launched := s.workshopFormat(c, wf, snapshot)
	c.Check(launched, testutil.JsonEquals, format["launched"])

	// Start workshop. Run dir is needed for workshop.socket.
	fs, err := s.bd.WorkshopFs(s.ctx, wf.Name)
	c.Assert(err, check.IsNil)
	err = fs.MkdirAll(dirs.WorkshopRunDir, 0755)
	fs.Close()
	c.Assert(err, check.IsNil)
	err = s.bd.StartWorkshop(s.ctx, wf.Name)
	c.Assert(err, check.IsNil)

	// Validate post-start metadata.
	started := s.workshopFormat(c, wf, snapshot)
	c.Check(started, testutil.JsonEquals, format["started"])

	// Install Store SDK.
	meta := sdk.Meta{
		Setup: sdk.Setup{
			Name:     "store-sdk",
			Channel:  "latest/stable",
			Revision: sdk.R(23),
			Sha3_384: "18b8ce233667942e94e1f5bdd22bcd516c4a375030a359a5bb09220b416d215fffda138d8d45eaab419ae2403c81ec5d",
		},
		SdkYAML: `name: store-sdk
`,
	}
	helper.MockSdkVolume(c, s.ctx, s.bd, meta)
	defer func() { _ = s.bd.DeleteSdk(s.ctx, meta.Setup) }()
	err = s.bd.InstallSdk(s.ctx, wf.Name, meta.Setup)
	c.Assert(err, check.IsNil)
	defer func() { _ = s.bd.UninstallSdk(s.ctx, wf.Name, meta.Name) }()

	// Validate post-install metadata.
	snapshot.Sdks = append(snapshot.Sdks, sdk.SetupId(meta.Setup))
	sdkAttached := s.workshopFormat(c, wf, snapshot)
	c.Check(sdkAttached, testutil.JsonEquals, format["sdk-attached"])

	// Install in-project SDK.
	setup2 := sdk.Setup{
		Name:     "local-sdk",
		Source:   sdk.ProjectSource,
		Revision: sdk.R(-34),
		Sha3_384: "dc00101dfd688cdc058e31d3b82e680df123f85935f741fabcb8f0dfd29d80612f131db8487621abad7ee856223bede1",
	}
	userDataDir := workshop.UserDataRootDir(s.usr.HomeDir, nil)
	sdkDir := workshop.LocalSdkDir(userDataDir, s.project.ProjectId, wf.Name, setup2.Name)
	err = os.MkdirAll(filepath.Join(sdkDir, setup2.Sha3_384), 0755)
	c.Assert(err, check.IsNil)
	err = s.bd.InstallSdk(s.ctx, wf.Name, setup2)
	c.Assert(err, check.IsNil)

	// Validate post-install metadata.
	snapshot.Sdks = append(snapshot.Sdks, sdk.SetupId(setup2))
	sdkMounted := s.workshopFormat(c, wf, snapshot)
	c.Check(sdkMounted, testutil.JsonEquals, format["sdk-mounted"])

	// Snapshot workshop.
	err = s.bd.TakeSnapshot(s.ctx, wf.Name, snapshot)
	c.Assert(err, check.IsNil)

	// Validate snapshot metadata.
	sdkSnapshot := s.snapshotFormat(c, wf.Name, snapshot)

	conn, err := s.bd.LxdClient(s.ctx)
	c.Assert(err, check.IsNil)
	defer conn.Disconnect()
	newApi := conn.HasExtension("instance_refresh_config")
	if !newApi {
		for _, name := range []string{"cache.apt", "workshop.network", "workshop.socket", "workshop.workshopctl"} {
			c.Check(sdkSnapshot.Devices[name], check.DeepEquals, map[string]string{"type": "none"})
			delete(sdkSnapshot.Devices, name)
		}
	}

	c.Check(sdkSnapshot, testutil.JsonEquals, format["snapshot"])
}

func (s *snapshotSuite) workshopFormat(c *check.C, file *workshop.File, snapshot workshop.Snapshot) api.InstancePut {
	conn, err := s.bd.LxdClient(s.ctx)
	c.Assert(err, check.IsNil)
	defer conn.Disconnect()

	name := lxdbackend.InstanceName(file.Name, s.project.ProjectId)
	inst := fullInstance(c, conn, name).Writable()

	// Remove architecture to make the test hardware-agnostic. It already
	// affects the base image fingerprint so we don't need to worry about
	// it too much.
	inst.Architecture = ""

	// Remove config options which aren't constant.
	for k := range inst.Config {
		if strings.HasPrefix(k, "volatile.") || strings.HasPrefix(k, "image.") {
			delete(inst.Config, k)
		}
	}
	c.Check(inst.Config["user.workshop.base-fingerprint"], check.Equals, snapshot.Image.Fingerprint)
	delete(inst.Config, "user.workshop.base-fingerprint")

	// Marshalling might be nondeterministic.
	var wf workshop.File
	c.Assert(yaml.Unmarshal([]byte(inst.Config["user.workshop.file"]), &wf), check.IsNil)
	c.Check(&wf, check.DeepEquals, file)
	delete(inst.Config, "user.workshop.file")

	// Avoid having to update the saved configs when bumping the revision.
	c.Check(inst.Config["user.workshop.format-revision"], check.Equals, lxdbackend.SnapshotFormatRevision.String())
	delete(inst.Config, "user.workshop.format-revision")

	// Hardware-dependent, not much influence on snapshots.
	delete(inst.Config, "nvidia.driver.capabilities")
	delete(inst.Config, "nvidia.runtime")

	// These ones are a bit long, replace with hash for readability.
	digest := sha3.Sum384([]byte(inst.Config["user.network-config"]))
	inst.Config["user.network-config"] = hex.EncodeToString(digest[:])
	digest = sha3.Sum384([]byte(inst.Config["user.user-data"]))
	inst.Config["user.user-data"] = hex.EncodeToString(digest[:])

	// Host paths of default devices can change without affecting the
	// workshop, so we exclude them from the hash. Other device options
	// should be included in case they influence the rootfs.
	delete(inst.Devices["workshop.workshopctl"], "source")
	delete(inst.Devices["cache.apt"], "source")
	delete(inst.Devices["workshop.socket"], "connect")

	for _, sk := range snapshot.Sdks {
		device := inst.Devices[workshop.SdkDeviceName(sk.Name)]
		var installedAt time.Time
		c.Assert(installedAt.UnmarshalText([]byte(device["user.sdk.installed-at"])), check.IsNil)
		c.Check(installedAt.IsZero(), check.Equals, false)
		delete(device, "user.sdk.installed-at")

		if _, ok := device["pool"]; !ok {
			delete(device, "source")
		}
	}

	return inst
}

func (s *snapshotSuite) snapshotFormat(c *check.C, name string, snapshot workshop.Snapshot) api.InstancePut {
	conn, err := s.bd.LxdClient(s.ctx)
	c.Assert(err, check.IsNil)
	defer conn.Disconnect()
	snapshotConn := conn.UseProject("workshop-snapshots." + s.usr.Username)

	sk := snapshot.Sdks[len(snapshot.Sdks)-1].Name
	snapshotName := s.snapshotName(c, snapshotConn, name, sk)
	defer func() {
		// TODO: move to main test once backend gets a RemoveSnapshot API.
		op, err := snapshotConn.DeleteInstance(snapshotName, false)
		c.Assert(err, check.IsNil)
		c.Assert(op.Wait(), check.IsNil)
	}()

	inst := fullInstance(c, snapshotConn, snapshotName).Writable()

	// Remove architecture to make the test hardware-agnostic. It already
	// affects the base image fingerprint so we don't need to worry about
	// it too much.
	inst.Architecture = ""

	// Remove config options which aren't constant.
	for k := range inst.Config {
		if strings.HasPrefix(k, "volatile.") || strings.HasPrefix(k, "image.") {
			delete(inst.Config, k)
		}
	}
	c.Check(inst.Config["user.workshop.base-fingerprint"], check.Equals, snapshot.Image.Fingerprint)
	delete(inst.Config, "user.workshop.base-fingerprint")

	digest, err := s.bd.HashSnapshot(snapshot)
	c.Assert(err, check.IsNil)

	// Avoid having to update the saved configs when bumping the revision.
	c.Check(inst.Config["user.workshop.format-revision"], check.Equals, lxdbackend.SnapshotFormatRevision.String())
	delete(inst.Config, "user.workshop.format-revision")
	c.Check(inst.Config["user.workshop.sha3-384"], check.Equals, digest)
	delete(inst.Config, "user.workshop.sha3-384")

	return inst
}

func (s *snapshotSuite) snapshotName(c *check.C, snapshotConn lxd.InstanceServer, w, sk string) string {
	args := lxd.GetInstancesArgs{
		InstanceType: api.InstanceTypeContainer,
		Filters: []string{
			"config.user.workshop.project-id=" + s.project.ProjectId,
			"config.user.workshop.name=" + w,
			"config.user.workshop.snapshot-type=sdk",
			"config.user.workshop.sdk=" + sk,
		},
	}
	snapshots, err := snapshotConn.GetInstances(args)
	c.Assert(err, check.IsNil)
	c.Assert(snapshots, check.HasLen, 1)
	return snapshots[0].Name
}
