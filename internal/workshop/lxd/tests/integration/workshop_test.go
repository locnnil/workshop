//go:build integration
// +build integration

package lxdbackend_integration_test

import (
	"context"
	"os"
	"os/user"
	"sync"

	"github.com/canonical/workshop/internal/progress"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
	"github.com/canonical/workshop/internal/workshop/lxd/tests/helper"
	"gopkg.in/check.v1"
)

type wsOps struct {
	bd                 workshop.Backend
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

	f.bd, err = lxdbackend.New()
	c.Assert(err, check.IsNil)

	f.username = "testuser"
	f.project = &workshop.Project{
		ProjectId: "42424242",
		Path:      c.MkDir(),
	}
	f.ctx = helper.CreateTestContext(f.username, "42424242")

	f.restoreDevices = lxdbackend.FakeDefaultDevices(helper.DefaultTestDevices)
	f.restoreImageServer = lxdbackend.FakeImageServer(helper.MinimalImageServer)
	f.restoreLookupUsr = testutil.FakeFunc(func(name string) (*user.User, error) {
		u := &user.User{Name: f.username, Username: f.username, Uid: "1000", Gid: "1000"}
		return u, nil
	}, &workshop.LookupUsername)
	f.restoreNewId = testutil.FakeFunc(func() (string, error) {
		return f.project.ProjectId, nil
	}, &workshop.NewProjectId)
}

func (f *wsOps) TearDownSuite(c *check.C) {
	lxdclient, err := f.bd.(*lxdbackend.Backend).LxdClient(f.ctx)
	c.Check(err, check.IsNil)

	helper.CleanupLxdProject(c, lxdclient, lxdbackend.LxdProjectName(f.username))
	helper.CleanupLxdProject(c, lxdclient, lxdbackend.LxdSystemProjectName(f.username))
	f.restoreLookupUsr()
	f.restoreNewId()

	f.restoreDevices()
	f.restoreImageServer()

	err = os.RemoveAll(f.project.Path)
	c.Check(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendWorkshopStashUnstash(c *check.C) {
	helper.LaunchTestWorkshop(c, f.ctx, f.bd, f.project.Path)
	defer f.bd.RemoveWorkshop(f.ctx, "test")

	// Execute
	err := f.bd.StashWorkshop(f.ctx, "test")
	c.Assert(err, check.IsNil)

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.bd.Workshop(f.ctx, "test")
	c.Assert(err, check.NotNil)

	// Execute
	err = f.bd.UnstashWorkshop(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.bd.Workshop(f.ctx, "test")
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendWorkshopStashRemove(c *check.C) {
	helper.LaunchTestWorkshop(c, f.ctx, f.bd, f.project.Path)

	// Execute
	err := f.bd.StashWorkshop(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.bd.Workshop(f.ctx, "test")
	c.Assert(err, check.NotNil)

	// Execute
	err = f.bd.RemoveWorkshopStash(f.ctx, "test")
	c.Assert(err, check.IsNil)

	// Validate
	err = f.bd.UnstashWorkshop(f.ctx, "test")
	c.Assert(err, check.ErrorMatches, "workshop not found")
}

func (f *wsOps) TestLxdBackendStateStorageVolumeAddRemove(c *check.C) {
	helper.LaunchTestWorkshop(c, f.ctx, f.bd, f.project.Path)
	defer f.bd.RemoveWorkshop(f.ctx, "test")

	// Execute
	err := f.bd.CreateStateStorage(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)

	// Execute
	err = f.bd.DeleteStateStorage(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendRemoveWorkshopStash(c *check.C) {
	// Setup
	helper.LaunchTestWorkshop(c, f.ctx, f.bd, f.project.Path)

	// Execute
	err := f.bd.StashWorkshop(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.bd.Workshop(f.ctx, "test")
	c.Assert(err, testutil.ErrorIs, workshop.ErrWorkshopNotFound)

	// Execute
	err = f.bd.RemoveWorkshopStash(f.ctx, "test")
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendDeleteWorkshop(c *check.C) {
	// Execute
	helper.LaunchTestWorkshop(c, f.ctx, f.bd, f.project.Path)

	// Validate
	err := f.bd.RemoveWorkshop(f.ctx, "test")
	c.Assert(err, check.IsNil)
	_, err = f.bd.Workshop(f.ctx, "test")
	c.Assert(err, testutil.ErrorIs, workshop.ErrWorkshopNotFound)
}

func (f *wsOps) image(c *check.C, alias string) (string, error) {
	cli, err := f.bd.(*lxdbackend.Backend).LxdClient(f.ctx)
	c.Check(err, check.IsNil)
	entry, _, err := cli.GetImageAlias(lxdbackend.ImageAlias(alias))
	if err != nil {
		return "", err
	}
	return entry.Target, err
}

func (f *wsOps) deleteimage(c *check.C, fp string) error {
	cli, err := f.bd.(*lxdbackend.Backend).LxdClient(f.ctx)
	c.Check(err, check.IsNil)
	op, err := cli.DeleteImage(fp)
	c.Check(err, check.IsNil)
	return op.Wait()
}

func (f *wsOps) TestLxdBackendDownloadWorkshopBase(c *check.C) {
	// ensure there is no image in LXD storage
	fp, err := f.image(c, "ubuntu@22.04")
	if err == nil {
		c.Assert(f.deleteimage(c, fp), check.IsNil)
	}

	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			err := f.bd.Download(f.ctx, "ubuntu@22.04", nil)
			c.Check(err, check.IsNil)
			wg.Done()
		}()
	}
	wg.Wait()
	fp, err = f.image(c, "ubuntu@22.04")
	c.Assert(err, check.IsNil)
	c.Assert(f.deleteimage(c, fp), check.IsNil)
}
func (f *wsOps) TestLxdBackendDownloadMalformedBase(c *check.C) {
	err := f.bd.Download(f.ctx, "ubuntu:24.04", nil)
	c.Check(err, check.ErrorMatches, `"ubuntu:24.04" is not a correct base name`)
	err = f.bd.Download(f.ctx, "ubuntu@", nil)
	c.Check(err, check.ErrorMatches, `"ubuntu@" is not a correct base name`)
}

func (f *wsOps) TestLxdBackendDownloadBaseImageNotFound(c *check.C) {
	err := f.bd.Download(f.ctx, "ubuntu@1.01", nil)
	c.Check(err, check.ErrorMatches, `"ubuntu@1.01" download failed.*`)
}

func (f *wsOps) TestLxdBackendDownloadProtocolNotSupported(c *check.C) {
	defer lxdbackend.FakeImageServer("https://cloud-images.ubuntu.com/minimal/releases/")()
	err := f.bd.Download(f.ctx, "ubuntu@20.04", nil)
	c.Check(err, check.ErrorMatches, `unknown image server URL prefix \(supported: simplestreams, lxd\)`)
}

func (f *wsOps) TestLxdBackendDownloadWorkshopBaseResumeAfterCancellation(c *check.C) {
	// ensure there is no image in LXD storage
	fp, err := f.image(c, "ubuntu@22.04")
	if err == nil {
		c.Assert(f.deleteimage(c, fp), check.IsNil)
	}

	wcancel, cancel := context.WithCancel(f.ctx)
	defer cancel()

	var wg sync.WaitGroup
	var once sync.Once
	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func() {
			r := &progress.Reporter{
				Name: "1",
				Report: func(label string, done, total int) {
					once.Do(func() { cancel() })
				},
			}
			err := f.bd.Download(wcancel, "ubuntu@22.04", r)
			c.Check(err, testutil.ErrorIs, context.Canceled)
			wg.Done()
		}()
	}
	wg.Wait()

	// attempt to download after interruption (must pickup an ongoing operation
	// and wait for it).
	err = f.bd.Download(f.ctx, "ubuntu@22.04", nil)
	c.Assert(err, check.IsNil)

	fp, err = f.image(c, "ubuntu@22.04")
	c.Assert(err, check.IsNil)
	c.Assert(f.deleteimage(c, fp), check.IsNil)
}

func (f *wsOps) TestLxdBackendDownloadMultipleBasesConcurrently(c *check.C) {
	// ensure there is no image in LXD storage
	for _, b := range workshop.SupportedBases {
		fp, err := f.image(c, b)
		if err == nil {
			c.Assert(f.deleteimage(c, fp), check.IsNil)
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(workshop.SupportedBases))
	for i := 0; i < len(workshop.SupportedBases); i++ {
		idx := i
		go func() {
			err := f.bd.Download(f.ctx, workshop.SupportedBases[idx], nil)
			c.Check(err, check.IsNil)
			wg.Done()
		}()
	}
	wg.Wait()

	for _, b := range workshop.SupportedBases {
		fp, err := f.image(c, b)
		c.Assert(err, check.IsNil)
		c.Assert(f.deleteimage(c, fp), check.IsNil)
	}
}
