package workshopbackend

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/workshop/internal/logger"
)

type SdkProfile struct {
	sdk     string
	devices map[string]Device
}

func NewSdkProfile(sdkName string) SdkProfile {
	return SdkProfile{
		sdk:     sdkName,
		devices: make(map[string]Device),
	}
}

func (s SdkProfile) Name() string {
	return s.sdk
}

func (s SdkProfile) AddDevice(dev Device) error {
	if _, ok := s.devices[dev.Name()]; ok {
		return fmt.Errorf("device %s already exists in the %s SDK profile", dev.Name(), s.Name())
	}
	s.devices[dev.Name()] = dev
	return nil
}

type DeviceType int

const (
	BindMount DeviceType = iota
	DiskVolume
)

type Device struct {
	name       string
	properties map[string]string
	deviceType DeviceType
}

func (d Device) Name() string {
	return d.name
}

func (d Device) Type() DeviceType {
	return d.deviceType
}

func Mount(name, source, target string) Device {
	return Device{name: name,
		deviceType: BindMount,
		properties: map[string]string{"type": "disk", "source": source,
			"path": target},
	}
}

func Volume(name, mountTo, volume string) Device {
	return Device{
		name:       name,
		deviceType: DiskVolume,
		properties: map[string]string{"type": "disk",
			"pool":   "default",
			"path":   mountTo,
			"source": volume},
	}
}

func profileName(pid, workshop, sdk string) string {
	return strings.Join([]string{InstanceName(workshop, pid), sdk}, "-")
}

func (w Device) lxdProperties() map[string]string {
	return w.properties
}

func (p SdkProfile) lxdDevices() map[string]map[string]string {
	lxdDevs := make(map[string]map[string]string, len(p.devices))
	for _, d := range p.devices {
		lxdDevs[d.Name()] = d.properties
	}
	return lxdDevs
}

func (s *LxdBackend) AssignProfile(ctx context.Context, workshop string, profile SdkProfile) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	fs, err := s.WorkshopFs(ctx, workshop)
	if err != nil {
		return err
	}
	defer fs.Close()
	for _, dev := range profile.devices {
		if dev.Type() == BindMount {
			// confirm the target path exists
			target := dev.properties["path"]
			if info, err := fs.Stat(target); err != nil {
				return fmt.Errorf("cannot create a workshop mount with target %q: %v", target, err)
			} else if !info.IsDir() {
				return fmt.Errorf("cannot create a workshop mount with target %q: the target is not a directory", target)
			}
			// We have to workaround the LXD issue here as it would remove
			// the target directory on unmount if it was empty. It should
			// not touch the directories that it has not created.
			// https://github.com/canonical/lxd/issues/12648
			_, err := fs.Create(filepath.Join(target, fmt.Sprintf(".workshop.%s.%s", projectId, workshop)))
			if err != nil && !errors.Is(err, os.ErrExist) {
				logger.Noticef(`Cannot not prevent "target" in %q workshop from removing: %v`, workshop, err)
			}
		}
	}

	lxdname := profileName(projectId, workshop, profile.Name())
	newProfile := api.ProfilePut{
		Devices:     profile.lxdDevices(),
		Description: fmt.Sprintf("%q SDK profile for %q workshop", profile.Name(), workshop),
	}

	// Either create or update an existing LXD profile for the SDK so that later
	// it can be assigned to the required workshop
	_, etag, err := conn.GetProfile(lxdname)
	if err != nil && !api.StatusErrorCheck(err, http.StatusNotFound) {
		return err
	} else if api.StatusErrorCheck(err, http.StatusNotFound) {
		if err := conn.CreateProfile(api.ProfilesPost{ProfilePut: newProfile, Name: lxdname}); err != nil {
			return err
		}
	} else {
		if err := conn.UpdateProfile(lxdname, newProfile, etag); err != nil {
			return err
		}
	}

	inst, etag, err := conn.GetInstance(InstanceName(workshop, projectId))
	if err != nil {
		return err
	}

	if slices.Index(inst.Profiles, lxdname) == -1 {
		// Assigning the profile for the first time
		put := inst.InstancePut
		put.Profiles = append(put.Profiles, lxdname)
		op, err := conn.UpdateInstance(InstanceName(workshop, projectId), put, etag)
		if err != nil {
			return err
		}

		return op.WaitContext(ctx)
	}

	return nil
}

func (s *LxdBackend) RemoveProfile(ctx context.Context, workshop string, profile string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(InstanceName(workshop, projectId))
	if err != nil {
		return err
	}

	// 1. Untie the profile from the workshop
	lxdname := profileName(projectId, workshop, profile)
	if idx := slices.Index(inst.Profiles, lxdname); idx != -1 {
		inst.Profiles = slices.Delete(inst.Profiles, idx, idx+1)
		op, err := conn.UpdateInstance(InstanceName(workshop, projectId), inst.Writable(), etag)
		if err != nil {
			return err
		}
		if err := op.Wait(); err != nil {
			return err
		}
	}

	// 2. Delete the profile
	err = conn.DeleteProfile(profileName(projectId, workshop, profile))
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return fmt.Errorf("workshop %q has no %q SDK profile", workshop, profile)
	}
	return err
}
