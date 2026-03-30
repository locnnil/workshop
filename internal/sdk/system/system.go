package system

import (
	"archive/tar"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/progress"
	"github.com/canonical/workshop/internal/sdk"
)

var (
	//go:embed meta/*
	SystemSdkFs embed.FS

	SystemSdkRevision = sdk.R(1)

	RetrieveSystemSdk = retrieveSystemSdk
)

// Update the system SDK revision number when this hash changes.
const SystemSdkDigest = "5891a3a98ed62339c5c24ded56de52a18873bd73ba8e1e03725376e7fc89c7560944b5fb7260c288b17e115e538d7da6"

func SystemSdkMeta() (*sdk.Meta, error) {
	setup := sdk.Setup{
		Name:     "system",
		Source:   sdk.SystemSource,
		Revision: SystemSdkRevision,
		Sha3_384: SystemSdkDigest,
	}
	sdkYaml, err := SystemSdkFs.ReadFile("meta/sdk.yaml")
	if err != nil {
		return nil, err
	}
	return &sdk.Meta{Setup: setup, SdkYAML: string(sdkYaml)}, nil
}

func retrieveSystemSdk(file *os.File, setup sdk.Setup, report *progress.Reporter) error {
	if setup.Revision != SystemSdkRevision {
		return fmt.Errorf("system SDK (%s) not available", setup.Revision)
	}

	writer := tar.NewWriter(file)
	if err := addWritableFS(writer, SystemSdkFs); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	if report != nil {
		size := int64(1)
		info, err := file.Stat()
		if err != nil {
			logger.Noticef("System SDK on Retrieve: %v", err)
		} else {
			size = info.Size()
		}
		report.Report("download", size, size)
	}

	return nil
}

// Like w.AddFs(fsys) but ensures the user always has write permissions.
func addWritableFS(w *tar.Writer, fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		typ := d.Type()
		linkTarget := ""
		if typ == fs.ModeSymlink {
			var err error
			linkTarget, err = fs.ReadLink(fsys, name)
			if err != nil {
				return err
			}
		} else if !typ.IsRegular() && typ != fs.ModeDir {
			return errors.New("tar: cannot add non-regular file")
		}

		h, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return err
		}
		h.Name = name
		if typ.IsDir() {
			h.Name += "/"
		}
		// Adjust permissions so user can always write.
		h.Mode |= 0200

		if err := w.WriteHeader(h); err != nil {
			return err
		}
		if !typ.IsRegular() {
			return nil
		}

		f, err := fsys.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
}

func FakeRetrieveSystemSdk(f func(file *os.File, setup sdk.Setup, report *progress.Reporter) error) func() {
	oldRetrieveSystemSdk := RetrieveSystemSdk
	RetrieveSystemSdk = f
	return func() { RetrieveSystemSdk = oldRetrieveSystemSdk }
}
