package lxdbackend

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/entity"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/sdk"
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

func (s *Backend) CreateVolume(ctx context.Context, info workshop.VolumeSetup) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	// Create the storage volume entry
	vol := api.StorageVolumesPost{}
	vol.Name = info.Name
	vol.Type = "custom"
	vol.ContentType = "filesystem"
	vol.Config = volumeSetupToConfig(info)

	err = conn.CreateStoragePoolVolume(storagePool, vol)
	if api.StatusErrorCheck(err, http.StatusConflict) {
		return workshop.ErrVolumeAlreadyExists
	}
	return err
}

func (s *Backend) AttachVolume(ctx context.Context, wp, name, where string, ro bool) error {
	return s.AddWorkshopMount(ctx, wp, workshop.Mount{Name: name, Type: workshop.Volume, What: name, Where: where, ReadOnly: ro})
}

func (s *Backend) DetachVolume(ctx context.Context, wp, name string) error {
	return s.RemoveWorkshopMount(ctx, wp, name)
}

func (s *Backend) DeleteVolume(ctx context.Context, name string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	if err = conn.DeleteStoragePoolVolume(storagePool, "custom", name); err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return nil
		}
		if api.StatusErrorCheck(err, http.StatusBadRequest) && strings.Contains(err.Error(), "still in use") {
			return workshop.ErrVolumeInUse
		}
		return err
	}

	return nil
}

func (s *Backend) Volumes(ctx context.Context, kind string) ([]workshop.VolumeInfo, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Disconnect()

	info, err := conn.GetConnectionInfo()
	if err != nil {
		return nil, err
	}

	filters := []string{
		"type=custom",
		fmt.Sprintf("config.user.kind=%s", kind),
	}
	vols, err := conn.GetStoragePoolVolumesWithFilter(storagePool, filters)
	if err != nil {
		return nil, err
	}

	infos := make([]workshop.VolumeInfo, 0, len(vols))
	for _, vol := range vols {
		size := volumeSize(conn, vol.Name)
		infos = append(infos, volumeToInfo(&vol, info.Project, size))
	}
	return infos, nil
}

func (s *Backend) Volume(ctx context.Context, name string) (workshop.VolumeInfo, error) {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return workshop.VolumeInfo{}, err
	}
	defer conn.Disconnect()

	info, err := conn.GetConnectionInfo()
	if err != nil {
		return workshop.VolumeInfo{}, err
	}

	vol, _, err := conn.GetStoragePoolVolume(storagePool, "custom", name)
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return workshop.VolumeInfo{}, workshop.ErrVolumeNotFound
	}
	if err != nil {
		return workshop.VolumeInfo{}, err
	}

	size := volumeSize(conn, vol.Name)
	return volumeToInfo(vol, info.Project, size), nil
}

func volumeSetupToConfig(info workshop.VolumeSetup) map[string]string {
	config := map[string]string{
		"user.kind": info.Kind,
	}
	if info.Sha3_384 != "" {
		config["user.sha3-384"] = info.Sha3_384
	}
	if info.Sdk != "" {
		config["user.sdk.name"] = info.Sdk
	}
	if !info.Revision.Unset() {
		config["user.sdk.revision"] = info.Revision.String()
	}
	if info.Metadata != "" {
		config["user.sdk.meta"] = info.Metadata
	}
	return config
}

func volumeToInfo(volume *api.StorageVolume, lxdProject string, size uint64) workshop.VolumeInfo {
	revision, err := sdk.ParseRevision(volume.Config["user.sdk.revision"])
	if err != nil {
		revision = sdk.Revision{}
	}

	workshops := make(map[string][]string)
	for _, u := range volume.UsedBy {
		parsedURL, err := url.Parse(u)
		if err != nil {
			logger.Debugf("Failed to parse URL %q: %v", u, err)
			continue
		}
		entityType, projectName, _, pathArgs, err := entity.ParseURL(*parsedURL)
		if err != nil {
			logger.Debugf("Failed to parse entity from URL %q: %v", parsedURL.String(), err)
			continue
		}
		if entityType != entity.TypeInstance || len(pathArgs) == 0 {
			logger.Debugf("URL %q does not point to an instance, skipping", parsedURL.String())
			continue
		}
		if projectName != lxdProject {
			// Ignore SDK layers, and workshops owned by other users.
			continue
		}
		wp, pid := workshopProjectId(pathArgs[0])
		workshops[pid] = append(workshops[pid], wp)
	}

	return workshop.VolumeInfo{
		VolumeSetup: workshop.VolumeSetup{
			Name:     volume.Name,
			Kind:     volume.Config["user.kind"],
			Sha3_384: volume.Config["user.sha3-384"],
			Sdk:      volume.Config["user.sdk.name"],
			Revision: revision,
			Metadata: volume.Config["user.sdk.meta"],
		},
		Workshops: workshops,
		Size:      size,
	}
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
