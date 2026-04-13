package lxdbackend

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/timeutil"
	"github.com/canonical/workshop/internal/workshop"
)

// LockVolume ensures exclusive access to the specified volume. If there is an
// ongoing operation on the volume, the calling goroutine will block until the
// lock is available. This function creates a new channel for the volume (if one
// doesn't already exist) and uses it to synchronize access by allowing only one
// goroutine at a time to perform operations on the volume.
//
// The channel operates as a mutex: each goroutine trying to access the volume
// must receive a value from the channel, and only the first goroutine to do so
// will acquire the lock. Once a goroutine finishes its operation, it will
// release the lock by sending to the channel (thus allowing the next goroutine
// to proceed).
func lockVolume(ctx context.Context, volume string) error {
	volumeGuardsLock.Lock()
	guard, ok := volumeGuards[volume]
	if !ok {
		guard = &volumeGuard{}
		guard.counter = 0
		guard.c = make(chan struct{}, 1)
		volumeGuards[volume] = guard

		guard.c <- struct{}{}
	}
	guard.counter += 1
	volumeGuardsLock.Unlock()

	select {
	case <-guard.c:
		return nil
	case <-ctx.Done():
		volumeGuardsLock.Lock()
		guard.counter -= 1
		if guard.counter == 0 {
			close(guard.c)
			delete(volumeGuards, volume)
		}
		volumeGuardsLock.Unlock()
		return ctx.Err()
	}
}

// UnlockVolume releases the lock on the specified volume, allowing other
// goroutines to access it. If there are no remaining goroutines waiting for the
// volume, the channel used for synchronisation will be closed and removed. This
// ensures that resources are cleaned up and no unnecessary channels remain in
// memory once the volume is no longer locked.
func unlockVolume(volume string) {
	volumeGuardsLock.Lock()
	defer volumeGuardsLock.Unlock()

	guard, ok := volumeGuards[volume]
	if !ok {
		panic(fmt.Errorf("%q volume is not locked", volume))
	}
	guard.c <- struct{}{}

	guard.counter -= 1
	if guard.counter == 0 {
		close(guard.c)
		delete(volumeGuards, volume)
	}
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

	vol := lxd.StoragePoolVolumeBackupArgs{
		BackupFile: tarball,
		Name:       name,
	}

	op, err := conn.CreateStoragePoolVolumeFromTarball(storagePool, vol)
	if err != nil {
		return err
	}
	if err = op.WaitContext(ctx); err != nil {
		return err
	}

	volume, etag, err := conn.GetStoragePoolVolume(storagePool, "custom", name)
	if err != nil {
		return err
	}
	volume.Config["user.kind"] = "sdk"
	volume.Config["user.sdk.name"] = meta.Name
	volume.Config["user.sdk.package-id"] = meta.PackageID
	volume.Config["user.sdk.revision"] = meta.Revision.String()
	volume.Config["user.sha3-384"] = meta.Sha3_384
	volume.Config["user.sdk.meta"] = meta.SdkYAML
	op, err = conn.UpdateStoragePoolVolume(storagePool, "custom", name, volume.Writable(), etag)
	if err != nil {
		return err
	}
	return op.Wait()
}

func (s *Backend) DeleteSdk(ctx context.Context, setup sdk.Setup) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	name := sdk.VolumeName(setup.Name, setup.Revision)
	op, err := conn.DeleteStoragePoolVolume(storagePool, "custom", name)
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return nil
		}
		if api.StatusErrorCheck(err, http.StatusBadRequest) && strings.Contains(err.Error(), "still in use") {
			return workshop.ErrVolumeInUse
		}
		return err
	}

	return op.Wait()
}

func (s *Backend) Sdks(ctx context.Context) ([]workshop.SdkVolume, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Disconnect()

	info, err := conn.GetConnectionInfo()
	if err != nil {
		return nil, err
	}

	filters := []string{"type=custom", "config.user.kind=sdk"}
	vols, err := conn.GetStoragePoolVolumesWithFilter(storagePool, filters)
	if err != nil {
		return nil, err
	}

	sdks := make([]workshop.SdkVolume, 0, len(vols))
	for _, vol := range vols {
		size := volumeSize(conn, vol.Name)
		sk, err := sdkVolume(&vol, info.Project, size)
		if err != nil {
			return nil, err
		}
		sdks = append(sdks, sk)
	}
	return sdks, nil
}

func (s *Backend) Sdk(ctx context.Context, setup sdk.Setup) (workshop.SdkVolume, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return workshop.SdkVolume{}, err
	}
	defer conn.Disconnect()

	info, err := conn.GetConnectionInfo()
	if err != nil {
		return workshop.SdkVolume{}, err
	}

	vol, _, err := conn.GetStoragePoolVolume(storagePool, "custom", sdk.VolumeName(setup.Name, setup.Revision))
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return workshop.SdkVolume{}, workshop.ErrVolumeNotFound
	}
	if err != nil {
		return workshop.SdkVolume{}, err
	}

	size := volumeSize(conn, vol.Name)
	return sdkVolume(vol, info.Project, size)
}

func volumeSize(conn lxd.InstanceServer, name string) uint64 {
	state, err := conn.GetStoragePoolVolumeState(storagePool, "custom", name)
	if err != nil {
		logger.Debugf("failed to retrieve volume state for %q: %v", name, err)
		return 0
	}

	if state.Usage != nil {
		return state.Usage.Used
	}

	return 0
}

func sdkVolume(volume *api.StorageVolume, lxdProject string, size uint64) (workshop.SdkVolume, error) {
	revision, err := sdk.ParseRevision(volume.Config["user.sdk.revision"])
	if err != nil {
		return workshop.SdkVolume{}, err
	}

	meta := sdk.Meta{
		Setup: sdk.Setup{
			Name:      volume.Config["user.sdk.name"],
			PackageID: volume.Config["user.sdk.package-id"],
			Revision:  revision,
			Sha3_384:  volume.Config["user.sha3-384"],
		},
		SdkYAML: volume.Config["user.sdk.meta"],
	}

	if sdk.IsSystem(meta.Name) {
		meta.Source = sdk.SystemSource
	} else if meta.Revision.Local() {
		meta.Source = sdk.TrySource
	}

	workshops := make(map[string][]string)
	for _, u := range volume.UsedBy {
		parsedURL, err := url.Parse(u)
		if err != nil {
			return workshop.SdkVolume{}, err
		}
		entityType, projectName, _, pathArgs, err := entity.ParseURL(*parsedURL)
		if err != nil {
			return workshop.SdkVolume{}, err
		}
		if entityType != entity.TypeInstance || len(pathArgs) == 0 {
			logger.Debugf("URL %q does not point to an instance, skipping", parsedURL.String())
			continue
		}
		if projectName != lxdProject {
			// Ignore SDK snapshots, and workshops owned by other users.
			continue
		}
		wp, pid := workshopProjectId(pathArgs[0])
		workshops[pid] = append(workshops[pid], wp)
	}

	sk := workshop.SdkVolume{
		Meta:      meta,
		Workshops: workshops,
		Size:      size,
	}
	return sk, nil
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
		if err := s.mkdir(ctx, name, mount.Where, mount.Mode); err != nil {
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

	if _, exist := inst.Devices[mount.Name]; exist {
		return fmt.Errorf("%q SDK is already installed", setup.Name)
	}

	maxInstallOrder := 0
	for key, device := range inst.Devices {
		s, err := maybeSdkInstallation(key, device)
		if err != nil {
			return err
		}
		if s != nil {
			maxInstallOrder = max(maxInstallOrder, s.InstallOrder)
		}
	}

	installation := workshop.SdkInstallation{
		Setup:        setup,
		InstallOrder: maxInstallOrder + 1,
		InstalledAt:  workshop.InstallTimeNow().UTC(),
	}
	device, err := sdkToLxdDisk(installation, mount)
	if err != nil {
		return err
	}
	inst.Devices[mount.Name] = device

	op, err := conn.UpdateInstance(inst.Name, inst.Writable(), etag)
	if err != nil {
		return err
	}
	return op.WaitContext(ctx)
}

func (s *Backend) mkdir(ctx context.Context, name string, path string, perm os.FileMode) error {
	fs, err := s.WorkshopFs(ctx, name)
	if err != nil {
		return err
	}
	defer fs.Close()

	return fs.MkdirAll(path, perm)
}

func sdkToLxdDisk(sk workshop.SdkInstallation, mount workshop.Mount) (map[string]string, error) {
	device := mountToLxdDisk(mount)

	device["user.sdk.channel"] = sk.Channel
	device["user.sdk.package-id"] = sk.PackageID
	device["user.sdk.revision"] = sk.Revision.String()
	device["user.sdk.sha3-384"] = sk.Sha3_384
	device["user.sdk.install-order"] = strconv.FormatInt(int64(sk.InstallOrder), 10)

	source, err := sk.Source.MarshalText()
	if err != nil {
		return nil, err
	}
	device["user.sdk.source"] = string(source)

	installedAt, err := timeutil.TimeUTC(sk.InstalledAt).MarshalText()
	if err != nil {
		return nil, err
	}
	device["user.sdk.installed-at"] = string(installedAt)

	return device, nil
}

func maybeSdkInstallation(key string, device map[string]string) (*workshop.SdkInstallation, error) {
	name, found := strings.CutPrefix(key, workshop.SdkDeviceName(""))
	if !found {
		return nil, nil
	}

	installOrder, err := strconv.ParseInt(device["user.sdk.install-order"], 10, 0)
	if err != nil {
		return nil, err
	}
	s := &workshop.SdkInstallation{
		Setup: sdk.Setup{
			Name:      name,
			PackageID: device["user.sdk.package-id"],
			Channel:   device["user.sdk.channel"],
			Sha3_384:  device["user.sdk.sha3-384"],
		},
		InstallOrder: int(installOrder),
	}
	if err := s.Source.UnmarshalText([]byte(device["user.sdk.source"])); err != nil {
		return nil, err
	}
	if err := s.Revision.UnmarshalText([]byte(device["user.sdk.revision"])); err != nil {
		return nil, err
	}
	installedAt := device["user.sdk.installed-at"]
	if installedAt == "" {
		// TODO: remove this after a short transition period.
		installedAt = device["user.sdk.install-time"]
	}
	installedUTC := (*timeutil.TimeUTC)(&s.InstalledAt)
	if err := installedUTC.UnmarshalText([]byte(installedAt)); err != nil {
		return nil, err
	}

	return s, nil
}

func sdkToSnapshotDevice(installOrder int, sk sdk.ContentID) map[string]string {
	return map[string]string{
		"type":                   "none",
		"user.sdk.sha3-384":      sk.Sha3_384,
		"user.sdk.is-volume":     strconv.FormatBool(sk.IsVolume),
		"user.sdk.install-order": strconv.FormatInt(int64(installOrder), 10),
	}
}

func (s *Backend) UninstallSdk(ctx context.Context, name, sk string) error {
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

	delete(inst.Devices, workshop.SdkDeviceName(sk))

	op, err := conn.UpdateInstance(inst.Name, inst.Writable(), etag)
	if err != nil {
		return err
	}
	return op.WaitContext(ctx)
}
