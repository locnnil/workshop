//go:build integration
// +build integration

package workshopbackend_test

import (
	"context"
	"os"
	"os/user"

	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
	"gopkg.in/check.v1"
)

type wsOps struct {
	// per suite
	be workshop.Backend

	// per test
	ctx                context.Context
	username           string
	project            *workshop.Project
	restoreLookupUsr   func()
	restoreNewId       func()
	restoreDevices     func()
	restoreImageServer func()
}

var _ = check.Suite(&wsOps{})

func (f *wsOps) SetUpSuite(c *check.C) {
	var err error

	f.be, err = lxdbackend.New()
	c.Assert(err, check.IsNil)

	f.username = "testuser"
	f.project = &workshop.Project{
		ProjectId: "42424242",
		Path:      c.MkDir(),
	}
	f.ctx = createTestContext(f.username, "42424242")

	f.restoreDevices = lxdbackend.FakeDefaultDevices(defaultTestDevices)
	f.restoreImageServer = lxdbackend.FakeImageServer(minimalImageServer)
	f.restoreLookupUsr = testutil.FakeFunc(func(name string) (*user.User, error) {
		u := &user.User{Name: f.username, Username: f.username, Uid: "1000", Gid: "1000"}
		return u, nil
	}, &workshop.LookupUsername)
	f.restoreNewId = testutil.FakeFunc(func() (string, error) {
		return f.project.ProjectId, nil
	}, &workshop.NewProjectId)

	err = f.be.Download(f.ctx, "ubuntu@24.04", nil)
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TearDownSuite(c *check.C) {
	lxdclient, err := f.be.(*lxdbackend.Backend).LxdClient(f.ctx)
	c.Check(err, check.IsNil)

	cleanUpLxdProject(c, lxdclient, lxdbackend.LxdProjectName(f.username))
	cleanUpLxdProject(c, lxdclient, lxdbackend.LxdSystemProjectName(f.username))
	f.restoreLookupUsr()
	f.restoreNewId()

	f.restoreDevices()
	f.restoreImageServer()

	err = os.RemoveAll(f.project.Path)
	c.Check(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendWorkshopStashUnstash(c *check.C) {
	launchTestWorkshop(c, f.ctx, f.be, f.project.Path, f.username)
	defer f.be.RemoveWorkshop(f.ctx, "test")

	// Execute
	err := f.be.StashWorkshop(f.ctx, "test")
	c.Assert(err, check.IsNil)

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.Workshop(f.ctx, "test")
	c.Assert(err, check.NotNil)

	// Execute
	err = f.be.UnstashWorkshop(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.Workshop(f.ctx, "test")
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendWorkshopStashRemove(c *check.C) {
	launchTestWorkshop(c, f.ctx, f.be, f.project.Path, f.username)

	// Execute
	err := f.be.StashWorkshop(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.Workshop(f.ctx, "test")
	c.Assert(err, check.NotNil)

	// Execute
	err = f.be.RemoveWorkshopStash(f.ctx, "test")
	c.Assert(err, check.IsNil)

	// Validate
	err = f.be.UnstashWorkshop(f.ctx, "test")
	c.Assert(err, check.ErrorMatches, "workshop not found")
}

func (f *wsOps) TestLxdBackendStateStorageVolumeAddRemove(c *check.C) {
	launchTestWorkshop(c, f.ctx, f.be, f.project.Path, f.username)
	defer f.be.RemoveWorkshop(f.ctx, "test")

	// Execute
	err := f.be.CreateStateStorage(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)

	// Execute
	err = f.be.DeleteStateStorage(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendRemoveWorkshopStash(c *check.C) {
	// Setup
	launchTestWorkshop(c, f.ctx, f.be, f.project.Path, f.username)

	// Execute
	err := f.be.StashWorkshop(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.be.Workshop(f.ctx, "test")
	c.Assert(err, testutil.ErrorIs, workshop.ErrWorkshopNotFound)

	// Execute
	err = f.be.RemoveWorkshopStash(f.ctx, "test")
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendDeleteWorkshop(c *check.C) {
	// Execute
	launchTestWorkshop(c, f.ctx, f.be, f.project.Path, f.username)

	// Validate
	err := f.be.RemoveWorkshop(f.ctx, "test")
	c.Assert(err, check.IsNil)
	_, err = f.be.Workshop(f.ctx, "test")
	c.Assert(err, testutil.ErrorIs, workshop.ErrWorkshopNotFound)
}

func (f *wsOps) TestLxdBackendDownloadWorkshopBase(c *check.C) {
	err := f.be.Download(f.ctx, "ubuntu@24.04", nil)
	c.Assert(err, check.IsNil)
}
