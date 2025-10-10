//go:build integration
// +build integration

package lxdbackend_integration_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"os/user"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/canonical/lxd/shared/api"
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/progress"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
	"github.com/canonical/workshop/internal/workshop/lxd/tests/helper"
)

type wsOps struct {
	bd                 *lxdbackend.Backend
	ctx                context.Context
	usr                *user.User
	project            workshop.Project
	restoreLookupUsr   func()
	restoreNewId       func()
	restoreDevices     func()
	restoreImageServer func()
}

var _ = check.Suite(&wsOps{})

func (f *wsOps) SetUpSuite(c *check.C) {
	dirs.SetRootDir(c.MkDir())
	dirs.SetCacheDir(c.MkDir())
	c.Assert(dirs.CreateDirs(), check.IsNil)

	var err error

	f.bd, err = lxdbackend.New()
	c.Assert(err, check.IsNil)

	f.usr = &user.User{Username: "testuser", Uid: "1000", Gid: "1000"}
	f.project = workshop.Project{
		ProjectId: "42424242",
		Path:      c.MkDir(),
	}
	f.ctx = helper.CreateTestContext(f.usr.Username, "42424242")

	f.restoreDevices = workshop.FakeDefaultDevices(helper.DefaultTestDevices)
	f.restoreImageServer = lxdbackend.FakeImageServer(helper.MinimalImageServer)
	f.restoreLookupUsr = osutil.FakeUserLookup(func(name string) (*user.User, error) {
		return f.usr, nil
	})
	f.restoreNewId = testutil.FakeFunc(func() (string, error) {
		return f.project.ProjectId, nil
	}, &workshop.NewProjectId)
}

func (f *wsOps) TearDownSuite(c *check.C) {
	lxdclient, err := f.bd.LxdClient(f.ctx)
	c.Check(err, check.IsNil)
	defer lxdclient.Disconnect()

	helper.CleanupLxdProject(c, lxdclient, "workshop."+f.usr.Username)
	helper.CleanupLxdProject(c, lxdclient, "workshop-stash."+f.usr.Username)
	f.restoreLookupUsr()
	f.restoreNewId()

	f.restoreDevices()
	f.restoreImageServer()

	err = os.RemoveAll(f.project.Path)
	c.Check(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendWorkshopStashUnstash(c *check.C) {
	helper.LaunchTestWorkshop(c, f.ctx, f.bd, f.project.Path)
	defer helper.RemoveTestWorkshop(c, f.ctx, f.bd)

	// Wait a bit longer than the default start command, to ensure both
	// IPv4 and IPv6 addresses are ready.
	args := workshop.Execution{
		ExecArgs: workshop.ExecArgs{
			Command: []string{
				"/usr/lib/systemd/systemd-networkd-wait-online",
				"--ipv4",
				"--ipv6",
				"--operational-state=routable",
			},
			WorkDir: "/",
			Timeout: time.Minute,
		},
	}
	exectx, err := f.bd.Exec(f.ctx, "test", &args)
	c.Assert(err, check.IsNil)
	c.Assert(exectx.WaitExecution(f.ctx), check.IsNil)

	addresses := f.ipAddresses(c, "test")

	// Execute
	err = f.bd.StashWorkshop(f.ctx, "test")
	c.Assert(err, check.IsNil)

	// Validate
	_, err = f.bd.Workshop(f.ctx, "test")
	c.Assert(err, check.IsNil)

	// Execute
	err = f.bd.RemoveWorkshop(f.ctx, "test", false)
	c.Assert(err, check.IsNil)

	// Execute
	err = f.bd.UnstashWorkshop(f.ctx, "test")
	c.Assert(err, check.IsNil)

	// Validate
	_, err = f.bd.Workshop(f.ctx, "test")
	c.Assert(err, check.IsNil)

	err = f.bd.RemoveWorkshopStash(f.ctx, "test")
	c.Assert(err, check.IsNil)

	c.Check(f.ipAddresses(c, "test"), testutil.DeepUnsortedMatches, addresses)
}

func (f *wsOps) ipAddresses(c *check.C, name string) []string {
	conn, err := f.bd.LxdClient(f.ctx)
	c.Assert(err, check.IsNil)
	defer conn.Disconnect()

	inst, _, err := conn.GetInstanceFull(lxdbackend.InstanceName(name, f.project.ProjectId))
	c.Assert(err, check.IsNil)

	var addresses []string
	for _, network := range inst.State.Network {
		for _, address := range network.Addresses {
			if !slices.Contains([]string{"inet", "inet6"}, address.Family) {
				continue
			}
			if slices.Contains([]string{"link", "local"}, address.Scope) {
				continue
			}
			addresses = append(addresses, address.Address)
		}
	}
	return addresses
}

func (f *wsOps) TestLxdBackendWorkshopStashRemove(c *check.C) {
	helper.LaunchTestWorkshop(c, f.ctx, f.bd, f.project.Path)
	defer helper.RemoveTestWorkshop(c, f.ctx, f.bd)

	// Execute
	err := f.bd.StashWorkshop(f.ctx, "test")

	// Validate
	c.Assert(err, check.IsNil)
	_, err = f.bd.Workshop(f.ctx, "test")
	c.Assert(err, check.IsNil)

	// Execute
	err = f.bd.RemoveWorkshopStash(f.ctx, "test")
	c.Assert(err, check.IsNil)

	// Validate
	err = f.bd.UnstashWorkshop(f.ctx, "test")
	c.Assert(err, check.ErrorMatches, "workshop not launched")
}

func (f *wsOps) TestLxdBackendStorageVolumeAddRemove(c *check.C) {
	// Execute
	volume := workshop.VolumeSetup{
		Name:     "test",
		Kind:     "testkind",
		Sha3_384: "abc123",
	}
	err := f.bd.CreateVolume(f.ctx, volume)
	c.Assert(err, check.IsNil)

	// Validate
	vols, err := f.bd.Volumes(f.ctx, "testkind")
	c.Assert(err, check.IsNil)
	c.Check(vols, check.HasLen, 1)

	c.Check(vols[0].VolumeSetup, check.DeepEquals, volume)
	c.Check(vols[0].Workshops, check.HasLen, 0)

	// Execute
	err = f.bd.DeleteVolume(f.ctx, "test")
	c.Assert(err, check.IsNil)

	// Validate
	vols, err = f.bd.Volumes(f.ctx, "testkind")
	c.Assert(err, check.IsNil)
	c.Check(vols, check.HasLen, 0)
}

var testsdk = `name: test-sdk
title: title
base: ubuntu@20.04
version: '0.1.2'
summary: summary
description: SDK
sdkcraft-started-at: '2020-04-22T19:12:07.903032Z'
`

func (f *wsOps) TestLxdBackendStorageVolumeImportOK(c *check.C) {
	// Execute
	sdkfs := c.MkDir()
	volume := workshop.VolumeSetup{
		Name:     "test-1",
		Kind:     "sdk",
		Sdk:      "test",
		Revision: sdk.R(1),
		Metadata: testsdk,
	}
	tarball := helper.MockSdkTarball(c, volume.Sdk, sdkfs, testsdk)

	cmd := testutil.FakeCommand(c, "tar", `/usr/bin/tar "$@"`)

	var wg sync.WaitGroup

	var successCnt, existCnt int32
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			file, err := os.Open(tarball)
			c.Assert(err, check.IsNil)
			defer file.Close()
			if err := f.bd.ImportVolume(f.ctx, volume, file); err == nil {
				atomic.AddInt32(&successCnt, 1)
			} else if errors.Is(err, workshop.ErrVolumeAlreadyExists) {
				atomic.AddInt32(&existCnt, 1)
			} else {
				c.Assert(err, check.IsNil)
			}
		}()
	}
	wg.Wait()

	c.Check(atomic.LoadInt32(&successCnt), check.Equals, int32(1))
	c.Check(atomic.LoadInt32(&existCnt), check.Equals, int32(4))

	c.Check(cmd.Calls(), check.HasLen, 2)

	vinfo, err := f.bd.Volume(f.ctx, "test-1")
	c.Check(err, check.IsNil)
	c.Check(vinfo.VolumeSetup, check.DeepEquals, volume)
	c.Check(vinfo.Workshops, check.HasLen, 0)

	err = f.bd.DeleteVolume(f.ctx, "test-1")
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendStorageVolumeImportInterrupted(c *check.C) {
	// Execute
	sdkfs := c.MkDir()
	volume := workshop.VolumeSetup{
		Name:     "test-1",
		Kind:     "sdk",
		Sdk:      "test",
		Revision: sdk.R(1),
		Metadata: testsdk,
	}
	tarball := helper.MockSdkTarball(c, volume.Sdk, sdkfs, testsdk)

	cmd := testutil.FakeCommand(c, "tar", `/usr/bin/tar "$@"`)

	var wg sync.WaitGroup

	var successCnt, existCnt, canceled int32
	wg.Add(5)
	for i := 0; i < 5; i++ {
		newctx, cancel := context.WithCancel(f.ctx)

		if i%2 == 0 {
			cancel()
		} else {
			defer cancel()
		}

		go func() {
			defer wg.Done()
			file, err := os.Open(tarball)
			c.Assert(err, check.IsNil)
			defer file.Close()
			if err := f.bd.ImportVolume(newctx, volume, file); err == nil {
				atomic.AddInt32(&successCnt, 1)
			} else if errors.Is(err, workshop.ErrVolumeAlreadyExists) {
				atomic.AddInt32(&existCnt, 1)
			} else if errors.Is(err, context.Canceled) {
				atomic.AddInt32(&canceled, 1)
			} else {
				c.Assert(err, check.IsNil)
			}
		}()
	}
	wg.Wait()

	c.Check(atomic.LoadInt32(&successCnt), check.Equals, int32(1))
	c.Check(atomic.LoadInt32(&existCnt), check.Equals, int32(1))
	c.Check(atomic.LoadInt32(&canceled), check.Equals, int32(3))

	c.Check(cmd.Calls(), check.HasLen, 2)

	vinfo, err := f.bd.Volume(f.ctx, "test-1")
	c.Check(err, check.IsNil)
	c.Check(vinfo.VolumeSetup, check.DeepEquals, volume)
	c.Check(vinfo.Workshops, check.HasLen, 0)

	err = f.bd.DeleteVolume(f.ctx, "test-1")
	c.Assert(err, check.IsNil)
}

func (f *wsOps) TestLxdBackendDeleteWorkshop(c *check.C) {
	// Execute
	helper.LaunchTestWorkshop(c, f.ctx, f.bd, f.project.Path)

	// Validate
	err := f.bd.RemoveWorkshop(f.ctx, "test", true)
	c.Assert(err, check.IsNil)
	_, err = f.bd.Workshop(f.ctx, "test")
	c.Assert(err, testutil.ErrorIs, workshop.ErrWorkshopNotLaunched)
}

// List images marked by Workshop for the given base.
func (f *wsOps) listWorkshopImages(c *check.C, base string) []api.Image {
	cli, err := f.bd.LxdClient(f.ctx)
	c.Assert(err, check.IsNil)
	defer cli.Disconnect()

	images, err := cli.GetImagesWithFilter([]string{"type=container", "properties.workshop-base=" + base})
	c.Assert(err, check.IsNil)

	return images
}

// List images for the given base, including those unknown to Workshop. Only
// tested for ubuntu and ubuntu-minimal images.
func (f *wsOps) listAllImages(c *check.C, base string) []api.Image {
	parts := strings.FieldsFunc(base, func(r rune) bool { return r == '@' })
	c.Assert(parts, check.HasLen, 2)

	cli, err := f.bd.LxdClient(f.ctx)
	c.Assert(err, check.IsNil)
	defer cli.Disconnect()

	images, err := cli.GetImagesWithFilter([]string{"type=container", "properties.os=" + parts[0], "properties.version=" + parts[1]})
	c.Assert(err, check.IsNil)

	return images
}

func (f *wsOps) deleteImages(c *check.C, base string) {
	images := f.listAllImages(c, base)

	cli, err := f.bd.LxdClient(f.ctx)
	c.Assert(err, check.IsNil)
	defer cli.Disconnect()

	for _, image := range images {
		op, err := cli.DeleteImage(image.Fingerprint)
		c.Assert(err, check.IsNil)
		c.Assert(op.Wait(), check.IsNil)
	}
}

func (f *wsOps) TestLxdBackendDownloadBase(c *check.C) {
	// ensure there is no image in LXD storage
	f.deleteImages(c, "ubuntu@22.04")

	fingerprint, err := f.bd.GetBase(f.ctx, "ubuntu@22.04")
	c.Assert(err, check.IsNil)
	c.Assert(fingerprint, check.Not(check.Equals), "")

	var wg sync.WaitGroup
	wg.Add(5)
	for range 5 {
		go func() {
			err := f.bd.DownloadBase(f.ctx, "ubuntu@22.04", fingerprint, nil)
			c.Check(err, check.IsNil)
			wg.Done()
		}()
	}
	wg.Wait()

	images := f.listWorkshopImages(c, "ubuntu@22.04")
	c.Assert(images, check.HasLen, 1)
	c.Check(images[0].AutoUpdate, check.Equals, false)
	c.Check(images[0].Cached, check.Equals, false)
	c.Check(images[0].Fingerprint, check.Equals, fingerprint)

	// Check behaviour when image already downloaded.
	err = f.bd.DownloadBase(f.ctx, "ubuntu@22.04", fingerprint, nil)
	c.Check(err, check.IsNil)

	images2 := f.listWorkshopImages(c, "ubuntu@22.04")
	c.Assert(images2, check.HasLen, 1)
	c.Check(images2[0], check.DeepEquals, images[0])
}

func (f *wsOps) TestLxdBackendGetOrDownloadMalformedBase(c *check.C) {
	_, err := f.bd.GetBase(f.ctx, "ubuntu:24.04")
	c.Check(err, check.ErrorMatches, `invalid base "ubuntu:24.04" \(expected <NAME>@<VERSION>\)`)
	err = f.bd.DownloadBase(f.ctx, "ubuntu:24.04", "", nil)
	c.Check(err, check.ErrorMatches, `invalid base "ubuntu:24.04" \(expected <NAME>@<VERSION>\)`)

	_, err = f.bd.GetBase(f.ctx, "ubuntu@")
	c.Check(err, check.ErrorMatches, `invalid base "ubuntu@" \(expected <NAME>@<VERSION>\)`)
	err = f.bd.DownloadBase(f.ctx, "ubuntu@", "", nil)
	c.Check(err, check.ErrorMatches, `invalid base "ubuntu@" \(expected <NAME>@<VERSION>\)`)

	_, err = f.bd.GetBase(f.ctx, "canonical@ubuntu@24.04")
	c.Check(err, check.ErrorMatches, `invalid base "canonical@ubuntu@24.04" \(expected <NAME>@<VERSION>\)`)
	err = f.bd.DownloadBase(f.ctx, "canonical@ubuntu@24.04", "", nil)
	c.Check(err, check.ErrorMatches, `invalid base "canonical@ubuntu@24.04" \(expected <NAME>@<VERSION>\)`)
}

func (f *wsOps) TestLxdBackendDownloadBaseImageNotFound(c *check.C) {
	_, err := f.bd.GetBase(f.ctx, "ubuntu@1.01")
	c.Check(err, check.ErrorMatches, `base "ubuntu@1.01" not found.*`)

	err = f.bd.DownloadBase(f.ctx, "ubuntu@22.04", "##################", nil)
	c.Check(err, check.ErrorMatches, `"ubuntu@22.04" download failed.*`)
}

func (f *wsOps) TestLxdBackendDownloadProtocolNotSupported(c *check.C) {
	defer lxdbackend.FakeImageServer("https://cloud-images.ubuntu.com/minimal/releases")()

	_, err := f.bd.GetBase(f.ctx, "ubuntu@20.04")
	c.Check(err, check.ErrorMatches, `unknown image server URL prefix \(supported: simplestreams, lxd\)`)
	err = f.bd.DownloadBase(f.ctx, "ubuntu@20.04", "", nil)
	c.Check(err, check.ErrorMatches, `unknown image server URL prefix \(supported: simplestreams, lxd\)`)
}

func (f *wsOps) TestLxdBackendDownloadConcurrentErrors(c *check.C) {
	var wg sync.WaitGroup
	wg.Add(5)
	for range 5 {
		go func() {
			err := f.bd.DownloadBase(f.ctx, "ubuntu@22.04", "##################", nil)
			c.Check(err, check.ErrorMatches, `"ubuntu@22.04" download failed.*`)
			wg.Done()
		}()
	}
	wg.Wait()
}

func (f *wsOps) TestLxdBackendDownloadBaseResumeAfterCancellation(c *check.C) {
	// ensure there is no image in LXD storage
	f.deleteImages(c, "ubuntu@22.04")

	fingerprint, err := f.bd.GetBase(f.ctx, "ubuntu@22.04")
	c.Assert(err, check.IsNil)
	c.Assert(fingerprint, check.Not(check.Equals), "")

	wcancel, cancel := context.WithCancel(f.ctx)
	defer cancel()

	var wg sync.WaitGroup
	var once sync.Once
	wg.Add(3)
	for range 3 {
		go func() {
			r := &progress.Reporter{
				Name: "1",
				Report: func(label string, done, total int) {
					once.Do(func() { cancel() })
				},
			}
			err := f.bd.DownloadBase(wcancel, "ubuntu@22.04", fingerprint, r)
			c.Check(err, testutil.ErrorIs, context.Canceled)
			wg.Done()
		}()
	}
	wg.Wait()

	// attempt to download after interruption (must pickup an ongoing operation
	// and wait for it).
	err = f.bd.DownloadBase(f.ctx, "ubuntu@22.04", fingerprint, nil)
	c.Assert(err, check.IsNil)

	images := f.listWorkshopImages(c, "ubuntu@22.04")
	c.Assert(images, check.HasLen, 1)
	c.Check(images[0].AutoUpdate, check.Equals, false)
	c.Check(images[0].Cached, check.Equals, false)
	c.Check(images[0].Fingerprint, check.Equals, fingerprint)
}

func (f *wsOps) TestLxdBackendDownloadMultipleBasesConcurrently(c *check.C) {
	// ensure there is no image in LXD storage
	for _, b := range workshop.SupportedBases {
		f.deleteImages(c, b)
	}

	fingerprints := make([]string, len(workshop.SupportedBases))

	var wg sync.WaitGroup
	wg.Add(len(workshop.SupportedBases))
	for i, b := range workshop.SupportedBases {
		go func() {
			defer wg.Done()

			fingerprint, err := f.bd.GetBase(f.ctx, b)
			c.Assert(err, check.IsNil)
			c.Assert(fingerprint, check.Not(check.Equals), "")
			fingerprints[i] = fingerprint

			err = f.bd.DownloadBase(f.ctx, b, fingerprint, nil)
			c.Assert(err, check.IsNil)
		}()
	}
	wg.Wait()

	for i, b := range workshop.SupportedBases {
		images := f.listWorkshopImages(c, b)
		c.Assert(images, check.HasLen, 1)
		c.Check(images[0].AutoUpdate, check.Equals, false)
		c.Check(images[0].Cached, check.Equals, false)
		c.Check(images[0].Fingerprint, check.Equals, fingerprints[i])
	}
}

// Since our LXD projects don't contain images, we have to put them in the
// default project. So our images are likely to be shared with containers
// created using lxc. The next few tests ensure that it doesn't matter whether
// Workshop or lxc downloads the image first, and that LXD won't try to prune
// images shared in this way. However, there's a bug in LXD which causes it to
// prune images shared with non-default projects. This is the reason why we use
// the default project in the first place. So far the issue hasn't been
// observed in the wild, but it probably would have been if *craft used the
// same image server as Workshop. See this issue for details:
// https://github.com/canonical/lxd/issues/16515
func (f *wsOps) TestLxdBackendReuseDownloadedBase(c *check.C) {
	// Attempt twice in case the image is updated partway.
	for range 2 {
		// ensure there is no image in LXD storage
		f.deleteImages(c, "ubuntu@22.04")

		images := f.listAllImages(c, "ubuntu@22.04")
		c.Assert(images, check.HasLen, 0)

		fingerprint, err := f.bd.GetBase(f.ctx, "ubuntu@22.04")
		c.Assert(err, check.IsNil)
		c.Assert(fingerprint, check.Not(check.Equals), "")
		err = f.bd.DownloadBase(f.ctx, "ubuntu@22.04", fingerprint, nil)
		c.Assert(err, check.IsNil)

		images = f.listAllImages(c, "ubuntu@22.04")
		c.Assert(images, check.HasLen, 1)
		imageDownloaded := images[0]

		c.Check(imageDownloaded.AutoUpdate, check.Equals, false)
		c.Check(imageDownloaded.Cached, check.Equals, false)
		c.Check(imageDownloaded.Fingerprint, check.Equals, fingerprint)
		c.Check(imageDownloaded.Properties["workshop-base"], check.Equals, "ubuntu@22.04")
		c.Check(imageDownloaded.UpdateSource, check.IsNil)

		tempInstance := fmt.Sprintf("%08x-test", rand.Uint32())
		init := exec.Command("lxc", "init", "ubuntu-minimal:22.04", tempInstance)
		c.Assert(init.Run(), check.IsNil)
		cleanup := exec.Command("lxc", "delete", tempInstance)
		c.Assert(cleanup.Run(), check.IsNil)

		images = f.listAllImages(c, "ubuntu@22.04")
		if len(images) > 1 || images[0].Fingerprint != fingerprint {
			// Alias was just updated, try again.
			continue
		}

		c.Assert(images, check.HasLen, 1)
		imageCached := images[0]

		c.Check(imageCached.LastUsedAt, check.Not(check.Equals), imageDownloaded.LastUsedAt)
		imageCached.LastUsedAt = imageDownloaded.LastUsedAt
		c.Check(imageCached, check.DeepEquals, imageDownloaded)

		break
	}
}

func (f *wsOps) TestLxdBackendReuseCachedBase(c *check.C) {
	// ensure there is no image in LXD storage
	f.deleteImages(c, "ubuntu@22.04")

	images := f.listAllImages(c, "ubuntu@22.04")
	c.Assert(images, check.HasLen, 0)

	tempInstance := fmt.Sprintf("%08x-test", rand.Uint32())
	init := exec.Command("lxc", "init", "ubuntu-minimal:22.04", tempInstance)
	c.Assert(init.Run(), check.IsNil)
	cleanup := exec.Command("lxc", "delete", tempInstance)
	c.Assert(cleanup.Run(), check.IsNil)

	images = f.listAllImages(c, "ubuntu@22.04")
	c.Assert(images, check.HasLen, 1)
	imageCached := images[0]

	c.Check(imageCached.AutoUpdate, check.Equals, true)
	c.Check(imageCached.Cached, check.Equals, true)
	_, ok := imageCached.Properties["workshop-base"]
	c.Check(ok, check.Equals, false)
	c.Check(imageCached.UpdateSource, check.NotNil)

	err := f.bd.DownloadBase(f.ctx, "ubuntu@22.04", imageCached.Fingerprint, nil)
	c.Assert(err, check.IsNil)

	images = f.listAllImages(c, "ubuntu@22.04")
	c.Assert(images, check.HasLen, 1)
	imageDownloaded := images[0]

	// CreateImage updates Public, AutoUpdate, Filename, Properties
	// and Profiles if any of them are set. It also does so if
	// Profiles is nil (but not if it's a non-nil empty slice).
	// This works for us since we want to unset AutoUpdate and add the
	// workshop-base property. As a side effect, the Filename is lost as
	// well. This doesn't seem to have any practical significance.
	imageCached.Filename = ""
	imageCached.AutoUpdate = false
	imageCached.Cached = false
	imageCached.Properties["workshop-base"] = "ubuntu@22.04"
	c.Check(imageDownloaded, check.DeepEquals, imageCached)
}

func (f *wsOps) TestLxdBackendWorkshopStartFailed(c *check.C) {
	helper.LaunchTestWorkshop(c, f.ctx, f.bd, f.project.Path)
	defer helper.RemoveTestWorkshop(c, f.ctx, f.bd)

	err := f.bd.StopWorkshop(f.ctx, "test", true)
	c.Check(err, check.IsNil)

	// Leaves the workshop instance in a started state with a failed start
	// command. The StartWorkshop API must clean up its previous progress, i.e.
	// set the workshop to the Stopped state.
	defer lxdbackend.FakeStartCommand("exit 1")()

	err = f.bd.StartWorkshop(f.ctx, "test")
	c.Check(err, check.NotNil)

	w, err := f.bd.Workshop(f.ctx, "test")
	c.Check(err, check.IsNil)
	c.Check(w.Running, check.Equals, false)
}

func (f *wsOps) TestLxdBackendWorkshopLaunch(c *check.C) {
	fingerprint, err := f.bd.GetBase(f.ctx, "ubuntu@24.04")
	c.Assert(err, check.IsNil)
	err = f.bd.DownloadBase(f.ctx, "ubuntu@24.04", fingerprint, nil)
	c.Assert(err, check.IsNil)

	wf := &workshop.File{Name: "test", Base: "ubuntu@24.04"}
	err = f.bd.LaunchOrRebuildWorkshop(f.ctx, wf, fingerprint)
	c.Assert(err, check.IsNil)
	defer helper.RemoveTestWorkshop(c, f.ctx, f.bd)

	w, err := f.bd.Workshop(f.ctx, "test")
	c.Assert(err, check.IsNil)
	c.Check(w.BaseFingerprint, check.Equals, fingerprint)
}

func (f *wsOps) TestLxdBackendWorkshopRebuild(c *check.C) {
	helper.LaunchTestWorkshop(c, f.ctx, f.bd, f.project.Path)
	defer helper.RemoveTestWorkshop(c, f.ctx, f.bd)

	err := f.bd.StopWorkshop(f.ctx, "test", true)
	c.Assert(err, check.IsNil)

	fingerprint, err := f.bd.GetBase(f.ctx, "ubuntu@22.04")
	c.Assert(err, check.IsNil)
	err = f.bd.DownloadBase(f.ctx, "ubuntu@22.04", fingerprint, nil)
	c.Assert(err, check.IsNil)

	wf := &workshop.File{
		Name: "test",
		Base: "ubuntu@22.04",
	}

	// Execute
	err = f.bd.LaunchOrRebuildWorkshop(f.ctx, wf, fingerprint)
	c.Assert(err, check.IsNil)

	w, err := f.bd.Workshop(f.ctx, "test")
	c.Assert(err, check.IsNil)
	c.Check(w.BaseFingerprint, check.Equals, fingerprint)
}

func (f *wsOps) TestLxdBackendWorkshopRestoreResetsSdkConfiguration(c *check.C) {
	helper.LaunchTestWorkshop(c, f.ctx, f.bd, f.project.Path)
	defer helper.RemoveTestWorkshop(c, f.ctx, f.bd)

	w, err := f.bd.Workshop(f.ctx, "test")
	c.Assert(err, check.IsNil)
	fingerprint := w.BaseFingerprint

	sdkfs := c.MkDir()
	setup := sdk.Setup{
		Name:     "test-sdk",
		Revision: sdk.R(5),
		Source:   sdk.StoreSource,
	}
	err = w.AddSdk(f.ctx, setup)
	c.Assert(err, check.IsNil)
	c.Check(w.Sdks, check.HasLen, 1)

	volume := workshop.VolumeSetup{
		Name:     sdk.VolumeName(setup.Name, setup.Revision),
		Kind:     "sdk",
		Sdk:      setup.Name,
		Revision: setup.Revision,
		Metadata: testsdk,
	}
	tarball := helper.MockSdkTarball(c, setup.Name, sdkfs, testsdk)
	file, err := os.Open(tarball)
	c.Assert(err, check.IsNil)
	defer file.Close()

	err = f.bd.ImportVolume(f.ctx, volume, file)
	c.Assert(err, check.IsNil)
	defer func() { _ = f.bd.DeleteVolume(f.ctx, volume.Name) }()

	err = f.bd.AttachVolume(f.ctx, "test", volume.Name, sdk.SdkDir(setup.Name), true)
	c.Assert(err, check.IsNil)
	defer func() { _ = f.bd.DetachVolume(f.ctx, "test", volume.Name) }()

	info, err := f.bd.Volume(f.ctx, volume.Name)
	c.Assert(err, check.IsNil)
	c.Check(info.VolumeSetup, check.DeepEquals, volume)
	c.Check(info.Workshops, check.DeepEquals, map[string][]string{f.project.ProjectId: {"test"}})

	err = f.bd.StopWorkshop(f.ctx, "test", true)
	c.Assert(err, check.IsNil)

	err = f.bd.Snapshot(f.ctx, "test", "snapshot-1")
	c.Assert(err, check.IsNil)

	// Attach the SDK volume as "test-sdk-2" to the workshop after the snapshot
	// to immitate further SDK configuration changes. These should be gone after
	// Restore.
	setup2 := sdk.Setup{Name: "test-sdk-2", Revision: sdk.R(5)}
	err = w.AddSdk(f.ctx, setup2)
	c.Assert(err, check.IsNil)
	err = f.bd.AttachVolume(f.ctx, "test", volume.Name, sdk.SdkDir(setup2.Name), true)
	c.Assert(err, check.IsNil)

	// Restore the workshop from the snapshot.
	wf := &workshop.File{Name: "test", Base: "ubuntu@24.04"}
	err = f.bd.Restore(f.ctx, "test", "snapshot-1", wf)
	c.Assert(err, check.IsNil)

	w, err = f.bd.Workshop(f.ctx, "test")
	c.Assert(err, check.IsNil)
	c.Check(w.Running, check.Equals, false)

	// Check that Restore uses the provided "user.workshop.file," keeps its
	// base fingerprint and removes "test-sdk-2" setup from the workshop.
	c.Check(w.File, check.DeepEquals, wf)
	c.Check(w.BaseFingerprint, check.Equals, fingerprint)
	c.Check(w.Sdks, check.HasLen, 1)
	c.Check(w.Sdks[setup2.Name], check.DeepEquals, sdk.Setup{})

	// Check that "test-sdk-2" volume is not present in the workshop filesystem
	// anymore.
	fs, err := f.bd.WorkshopFs(f.ctx, "test")
	c.Assert(err, check.IsNil)
	defer fs.Close()
	_, err = fs.Stat(sdk.SdkDir(setup2.Name))
	c.Check(err, testutil.ErrorIs, os.ErrNotExist)
}

func (f *wsOps) TestLxdBackendWorkshopUsedByInVolumeInfoOK(c *check.C) {
	// First workshop
	helper.LaunchTestWorkshop(c, f.ctx, f.bd, f.project.Path)
	defer helper.RemoveTestWorkshop(c, f.ctx, f.bd)

	// Second workshop
	other := workshop.Project{ProjectId: "24242424", Path: c.MkDir()}
	otherCtx := helper.CreateTestContext(f.usr.Username, other.ProjectId)
	helper.LaunchTestWorkshop(c, otherCtx, f.bd, other.Path)
	defer helper.RemoveTestWorkshop(c, otherCtx, f.bd)

	sdkfs := c.MkDir()
	volume := workshop.VolumeSetup{
		Name:     "test-sdk-1",
		Kind:     "sdk",
		Sdk:      "test-sdk",
		Revision: sdk.R(1),
		Metadata: testsdk,
	}

	tarball := helper.MockSdkTarball(c, volume.Sdk, sdkfs, testsdk)
	file, err := os.Open(tarball)
	c.Assert(err, check.IsNil)
	defer file.Close()

	err = f.bd.ImportVolume(f.ctx, volume, file)
	c.Assert(err, check.IsNil)
	defer func() { c.Check(f.bd.DeleteVolume(f.ctx, volume.Name), check.IsNil) }()

	// Attach the volume to both workshops.
	err = f.bd.AttachVolume(f.ctx, "test", volume.Name, sdk.SdkDir(volume.Sdk), true)
	c.Assert(err, check.IsNil)
	err = f.bd.AttachVolume(otherCtx, "test", volume.Name, sdk.SdkDir(volume.Sdk), true)
	c.Assert(err, check.IsNil)

	// Ensure the volume cannot be deleted while attached to workshops.
	err = f.bd.DeleteVolume(f.ctx, volume.Name)
	c.Assert(err, testutil.ErrorIs, workshop.ErrVolumeInUse)

	// Validate UsedBy in VolumeInfo.
	info, err := f.bd.Volume(f.ctx, volume.Name)
	c.Assert(err, check.IsNil)
	c.Check(info.VolumeSetup, check.DeepEquals, volume)
	c.Check(info.Workshops, check.DeepEquals, map[string][]string{f.project.ProjectId: {"test"}, other.ProjectId: {"test"}})

	// Detach the volume from the first workshop.
	err = f.bd.DetachVolume(f.ctx, "test", volume.Name)
	c.Assert(err, check.IsNil)

	// Validate UsedBy in VolumeInfo.
	info, err = f.bd.Volume(f.ctx, volume.Name)
	c.Assert(err, check.IsNil)
	c.Check(info.VolumeSetup, check.DeepEquals, volume)
	c.Check(info.Workshops, check.DeepEquals, map[string][]string{other.ProjectId: {"test"}})

	err = f.bd.DetachVolume(otherCtx, "test", volume.Name)
	c.Assert(err, check.IsNil)
}
