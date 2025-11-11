package lxdbackend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
)

var volumeIndex = `name: %s
backend: %s
type: custom
config:
  volume:
    name: %s
    description: "SDK Volume"
    type: custom
    content_type: filesystem
`

func volumeIndexContent(name string) string {
	return fmt.Sprintf(volumeIndex, name, storagePool, name)
}

func (s *Backend) ImportSdk(ctx context.Context, meta sdk.Meta, tarball *os.File) error {
	name := sdk.VolumeName(meta.Name, meta.Revision)

	// There could be multiple launches that require the same volume. We don't
	// want to unpack and import the volume multiple times.
	if err := lockVolume(ctx, name); err != nil {
		return err
	}
	defer unlockVolume(name)

	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	_, _, err = conn.GetStoragePoolVolume(storagePool, "custom", name)
	if err == nil {
		return workshop.ErrVolumeAlreadyExists
	}
	if !api.StatusErrorCheck(err, http.StatusNotFound) {
		return err
	}

	// The tarballs will be transformed into a LXD-compatible backup format to
	// create them directly as a custom volume. The LXD's tar archive has the
	// following format:
	//
	// backup/
	//  volume/
	//  index.yaml

	dir, err := os.MkdirTemp("", name)
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	// umask 0 with --no-same-permissions preserves normal permissions,
	// just not setuid, setgid, etc. The parent directory should be empty
	// with 700 permissions. For more details see
	// https://www.gnu.org/software/tar/manual/html_section/Security.html
	unpack := exec.CommandContext(ctx, "bash",
		"-c",
		`umask 0 && exec -- "$0" "$@"`,
		"tar",
		"--extract",
		"--no-same-owner",
		"--no-same-permissions",
		"--keep-old-files",
		"--file=/dev/stdin",
		"--transform",
		"s,^,volume/,",
		"--directory="+dir,
	)
	unpack.Stdin = tarball

	if _, err := unpack.Output(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			logger.Debugf("Failed to unpack volume tarball: %s", exitErr.Stderr)
		}
		return err
	}

	// Generate index.yaml for the volume.
	if err = os.WriteFile(filepath.Join(dir, "index.yaml"), []byte(volumeIndexContent(name)), 0644); err != nil {
		return err
	}

	newtar := filepath.Join(dir, filepath.Base(tarball.Name()))
	repack := exec.CommandContext(ctx, "tar",
		"--create",
		"--format=posix",
		"--force-local",
		"--remove-files",
		"--file",
		newtar,
		"--transform",
		"s,^,backup/,",
		"--directory="+dir,
		"--no-same-owner",
		"volume/",
		"index.yaml",
	)

	if _, err := repack.Output(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			logger.Debugf("Failed to repack volume tarball: %s", exitErr.Stderr)
			return exitErr
		}
		return err
	}

	f, err := os.Open(newtar)
	if err != nil {
		return err
	}
	defer f.Close()

	vol := lxd.StoragePoolVolumeBackupArgs{
		BackupFile: f,
		Name:       name,
	}

	op, err := conn.CreateStoragePoolVolumeFromBackup(storagePool, vol)
	if err != nil {
		return err
	}

	if err = op.WaitContext(ctx); err != nil {
		return err
	}

	volPut := api.StorageVolumePut{
		Config: map[string]string{
			"user.kind":         "sdk",
			"user.sdk.name":     meta.Name,
			"user.sdk.revision": meta.Revision.String(),
			"user.sha3-384":     meta.Sha3_384,
			"user.sdk.meta":     meta.SdkYAML,
		},
	}
	return conn.UpdateStoragePoolVolume(storagePool, "custom", name, volPut, "")
}

func (s *Backend) DeleteSdk(ctx context.Context, setup sdk.Setup) error {
	return s.DeleteVolume(ctx, sdk.VolumeName(setup.Name, setup.Revision))
}

func (s *Backend) InstallSdk(ctx context.Context, name string, setup sdk.Setup) error {
	user, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", workshop.ContextUser)
	}

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	usr, env, err := osutil.UserAndEnv(user)
	if err != nil {
		return err
	}
	userDataDir := workshop.UserDataRootDir(usr.HomeDir, env)
	mount := workshop.SdkMount(userDataDir, projectId, name, setup)

	if mount.MakeWhere {
		if err := s.mkdir(ctx, name, mount.Where); err != nil {
			return err
		}
	}

	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	inst, etag, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}

	inst.Devices[mount.Name] = mountToLxdDisk(mount)

	if err := addSdk(inst.Config, setup); err != nil {
		return err
	}

	op, err := conn.UpdateInstance(inst.Name, inst.Writable(), etag)
	if err != nil {
		return err
	}
	return op.WaitContext(ctx)
}

func (s *Backend) mkdir(ctx context.Context, name string, path string) error {
	fs, err := s.WorkshopFs(ctx, name)
	if err != nil {
		return err
	}
	defer fs.Close()

	return fs.MkdirAll(path, 0755)
}

func addSdk(config map[string]string, setup sdk.Setup) error {
	var sdks map[string]workshop.SdkInstallation
	value, exist := config[workshop.ConfigWorkshopSdks]
	if !exist {
		sdks = map[string]workshop.SdkInstallation{}
	} else if err := json.Unmarshal([]byte(value), &sdks); err != nil {
		return err
	} else if _, exist := sdks[setup.Name]; exist {
		return fmt.Errorf("%q SDK is already installed", setup.Name)
	}

	sdks[setup.Name] = workshop.SdkInstallation{Setup: setup, InstallTime: workshop.InstallTimeNow()}

	buf, err := json.Marshal(sdks)
	if err != nil {
		return err
	}
	config[workshop.ConfigWorkshopSdks] = string(buf)

	return nil
}

func (s *Backend) UninstallSdk(ctx context.Context, name string, setup sdk.Setup) error {
	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	inst, etag, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}

	if err := removeSdk(inst.Config, setup.Name); err != nil {
		return err
	}

	delete(inst.Devices, sdk.VolumeName(setup.Name, setup.Revision))

	op, err := conn.UpdateInstance(inst.Name, inst.Writable(), etag)
	if err != nil {
		return err
	}
	return op.WaitContext(ctx)
}

func removeSdk(config map[string]string, sk string) error {
	var sdks map[string]workshop.SdkInstallation
	value, exist := config[workshop.ConfigWorkshopSdks]
	if !exist {
		return nil
	}

	if err := json.Unmarshal([]byte(value), &sdks); err != nil {
		return err
	}

	delete(sdks, sk)

	buf, err := json.Marshal(sdks)
	if err != nil {
		return err
	}
	config[workshop.ConfigWorkshopSdks] = string(buf)

	return nil
}
