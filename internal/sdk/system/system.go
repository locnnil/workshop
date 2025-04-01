package system

import (
	"archive/tar"
	"embed"
	"fmt"
	"os"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/progress"
	"github.com/canonical/workshop/internal/revert"
	"github.com/canonical/workshop/internal/sdk"
)

var (
	//go:embed meta/*
	SystemSdkFs embed.FS

	SystemSdkRevision = sdk.R(1)

	RetrieveSystemSdk = retrieveSystemSdk
)

func retrieveSystemSdk(setup sdk.Setup, report *progress.Reporter) error {
	fl, err := sdk.OpenLock(setup.Name)
	if err != nil {
		return err
	}
	if err = fl.Lock(); err != nil {
		return err
	}
	defer fl.Close()

	target := setup.Filepath()
	if osutil.FileExists(target) {
		logger.Debugf("System SDK on Retrieve: SDK found locally: %s", target)
		return nil
	}

	if setup.Revision != SystemSdkRevision {
		return fmt.Errorf("system SDK (%s) not available", setup.Revision)
	}

	r := revert.New()
	defer r.Fail()

	// TODO: remove old system SDKs when no longer in use.
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()
	r.Add(func() {
		if err1 := os.Remove(target); err1 != nil {
			logger.Noticef("System SDK on Retrieve: Cannot remove %q on a failed download: %v", target, err1)
		}
	})

	writer := tar.NewWriter(file)
	if err := writer.AddFS(SystemSdkFs); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	if report != nil {
		size := 1
		info, err := file.Stat()
		if err != nil {
			logger.Noticef("System SDK on Retrieve: %v", err)
		} else {
			size = int(info.Size())
		}
		report.Report("download", size, size)
	}

	r.Success()
	return nil
}

func FakeRetrieveSystemSdk(f func(setup sdk.Setup, report *progress.Reporter) error) func() {
	oldRetrieveSystemSdk := RetrieveSystemSdk
	RetrieveSystemSdk = f
	return func() { RetrieveSystemSdk = oldRetrieveSystemSdk }
}
