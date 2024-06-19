//go:build integration
// +build integration

package store_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/workshop/internal/dirs"
	store "github.com/canonical/workshop/internal/fakestore"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"gopkg.in/check.v1"
)

type storeIntegration struct {
	oldRoot string
}

var _ = check.Suite(&storeIntegration{})

func TestFakeStoreIntegration(t *testing.T) { check.TestingT(t) }

func (f *storeIntegration) SetUpSuite(c *check.C) {
	c.Assert(os.Setenv("SDK_STORE_URL", "http://localhost:8080"), check.IsNil)
	f.oldRoot = dirs.BaseDir
	dirs.SetRootDir(c.MkDir())
	c.Assert(dirs.CreateDirs(), check.IsNil)
}

func (f *storeIntegration) TearDownSuite(c *check.C) {
	c.Assert(os.Unsetenv("SDK_STORE_URL"), check.IsNil)
	dirs.SetRootDir(f.oldRoot)
}

func (f *storeIntegration) TestStoreDownloadOK(c *check.C) {
	s := store.New()
	setup := sdk.Setup{Name: "test-sdk-basic", Channel: "latest/stable"}
	err := s.DownloadSdk(context.Background(), setup)
	c.Assert(err, check.IsNil)
	c.Assert(setup.Filename(), testutil.FilePresent)
}

func (f *storeIntegration) TestStoreDownloadCleanupPrevious(c *check.C) {
	s := store.New()
	setup := sdk.Setup{Name: "test-sdk-basic", Channel: "latest/stable", Revision: 1}
	prev := filepath.Join(dirs.SdkDir, setup.Name) + "_5.sdk"
	_, err := os.Create(prev)
	c.Assert(err, check.IsNil)
	err = s.DownloadSdk(context.Background(), setup)
	c.Assert(err, check.IsNil)
	c.Assert(setup.Filename(), testutil.FilePresent)
	c.Assert(prev, check.Not(testutil.FilePresent))
}

func (f *storeIntegration) TestStoreDownloadNotfound(c *check.C) {
	s := store.New()
	setup := sdk.Setup{Name: "test-sdk-unknown", Channel: "latest/stable"}
	err := s.DownloadSdk(context.Background(), setup)
	c.Assert(err, check.ErrorMatches, `SDK not found`)
	c.Assert(setup.Filename(), check.Not(testutil.FilePresent))
}

func (f *storeIntegration) TestStoreDownloadSkipIfExists(c *check.C) {
	os.Setenv("WORKSHOP_DEBUG", "1")
	defer os.Unsetenv("WORKSHOP_DEBUG")

	m, r := logger.MockLogger()
	defer r()

	s := store.New()
	setup := sdk.Setup{Name: "test-sdk-basic", Channel: "latest/stable"}

	// Lock the file to emulate a concurrent download is going on
	target := setup.Filename()
	fl, err := osutil.NewFileLock(target + ".lock")
	c.Assert(err, check.IsNil)
	c.Assert(fl.Lock(), check.IsNil)

	go func() {
		err := s.DownloadSdk(context.Background(), setup)
		c.Assert(err, check.IsNil)
		c.Assert(m.String(), check.Matches, fmt.Sprintf("*.DEBUG: %s/test-sdk-basic_0.sdk exists, nothing to download...", dirs.SdkDir))
	}()

	// "download" is finished
	_, err = os.Create(target)
	c.Assert(err, check.IsNil)
	fl.Close()
}

type failingReader struct{}

func (*failingReader) Read(p []byte) (n int, err error) {
	return 0, context.DeadlineExceeded
}
func (*failingReader) Close() error { return nil }

func (f *storeIntegration) TestStoreDownloadRemoveUnfinished(c *check.C) {
	r := store.FakeSdkStoreSdkReader(func(ctx context.Context, setup sdk.Setup) (io.ReadCloser, error) {
		return &failingReader{}, nil
	})
	defer r()

	s := store.New()
	setup := sdk.Setup{Name: "test-sdk-basic", Channel: "latest/stable", Revision: 55}
	err := s.DownloadSdk(context.Background(), setup)
	c.Assert(err, check.NotNil)
	c.Assert(setup.Filename(), check.Not(testutil.FilePresent))
}

func (s *storeSuite) TestSdkActionInstallStoreError(c *check.C) {
	defer sdk.MockSanitizePlugsSlots(func(sdkInfo *sdk.Info) {})()

	store := store.New()
	acts := []sdk.SdkAction{{
		ProjectId: "24242424",
		Workshop:  "test-workshop",
		Action:    sdk.Install,
		Name:      "test-sdk-unknown",
		Channel:   "latest/stable",
	}}
	res, err := store.SdkAction(context.Background(), nil, acts)
	c.Assert(res, check.HasLen, 0)
	c.Assert(err, check.ErrorMatches, `SDK not found`)
}
