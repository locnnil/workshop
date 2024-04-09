package workshopbackend

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/exp/slices"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/revert"
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

func (p SdkProfile) lxdDevices() map[string]map[string]string {
	lxdDevs := make(map[string]map[string]string, len(p.devices))
	for _, d := range p.devices {
		lxdDevs[d.Name()] = d.properties
	}
	return lxdDevs
}

type DeviceType int

const (
	BindMount DeviceType = iota
	DiskVolume
	GPU
	Proxy
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

func Gpu(name string) Device {
	return Device{
		name:       name,
		deviceType: GPU,

		// The default workshop user must be able to acces the GPU device.
		// Workshop assigns the GPU devices to workshop.workshop. A more
		// traditional way here would be to add dri devices to the video/render
		// groups, but it requires an additional workshop exec to find out the
		// groups' ids at the LXD profile generation time. Given that we are
		// solving the problem of access in a confined environment and workshop
		// is a passwordless sudo user anyway, it was decided that it is OK if
		// the workshop user owns GPU devices.

		// On another note, the render and video groups are not assigned to the
		// card*/render* dri devices by LXD properly. Both will be assigned to
		// the group provided in "gid"; there is no way to assign video to card*
		// and render to render* devices.
		properties: map[string]string{"type": "gpu", "gputype": "physical", "uid": "1000", "gid": "1000"},
	}
}

// A network protocol proxy device, opens a port on the host or in a workhop.
// from, to are the source and destination addresses (paths in the case of unix sockets),
// see https://documentation.ubuntu.com/lxd/en/latest/reference/devices_proxy/#device-proxy-device-conf:bind
// bind denotes where the port is open (can be: instance, host)
func NetworkProxy(name string, from, to string, bind string) Device {
	return Device{
		name:       name,
		deviceType: Proxy,
		properties: map[string]string{"type": "proxy", "connect": from, "listen": to, "uid": "1000", "gid": "1000", "bind": bind},
	}
}

func profileName(pid, workshop, sdk string) string {
	return strings.Join([]string{InstanceName(workshop, pid), sdk}, "-")
}

func (w Device) lxdProperties() map[string]string {
	return w.properties
}

func (s *LxdBackend) AssignProfile(ctx context.Context, workshop string, profile SdkProfile) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

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
				if !osutil.IsDirNotExist(err) {
					return err
				}
				// FIXME: workaround LXD empty directory issue (which, if the
				// connection was disconnected earlier, was removed by LXD).
				if err = fs.Mkdir(target, os.ModePerm); err != nil {
					return err
				}
			} else if !info.IsDir() {
				return fmt.Errorf("cannot create a workshop mount with target %q: the target is not a directory", target)
			}
		}

		if dev.Type() == Proxy {
			// add SSH_AUTH_SOCK setup
			env, err := fs.Create(filepath.Join("/etc/profile.d", dev.name+".sh"))
			if err != nil {
				return fmt.Errorf("cannot set SSH_AUTH_SOCK for %q: %w", workshop, err)
			}

			_, err = env.Write([]byte("export SSH_AUTH_SOCK=" + strings.TrimPrefix(dev.properties["listen"], "unix:")))
			if err != nil {
				return fmt.Errorf("cannot set SSH_AUTH_SOCK for %q: %w", workshop, err)
			}
			_ = env.Close()
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

func (s *LxdBackend) lxdEmptyUnmountWorkaround(conn lxd.InstanceServer, ctx context.Context, projectId, workshop string, profile string) (*revert.Reverter, error) {
	// We have to workaround the LXD issue here as it would remove
	// the target directory on unmount if it was empty. It should
	// not touch the directories that it has not created.
	// https://github.com/canonical/lxd/issues/12648
	fs, err := s.WorkshopFs(ctx, workshop)
	if err != nil {
		return nil, err
	}

	lxdProfile, _, err := conn.GetProfile(profileName(projectId, workshop, profile))
	if err != nil {
		return nil, err
	}

	var revertWorkaround revert.Reverter
	revertWorkaround.Add(func() { fs.Close() })
	for _, mount := range lxdProfile.Devices {
		// only for the bind mounts
		if mount["type"] == "disk" && mount["pool"] == "" {
			target := mount["path"]
			info, err := fs.Stat(target)
			if err != nil {
				continue
			}
			revertWorkaround.Add(func() {
				// Making a new directory unconditionally can be done, because
				// the target must have existed for this profile to be created
				// in the first place.
				_ = fs.Mkdir(target, info.Mode())
			})
		}
	}

	return &revertWorkaround, nil
}

func (s *LxdBackend) RemoveProfile(ctx context.Context, workshop string, profile string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(InstanceName(workshop, projectId))
	if err != nil {
		return err
	}

	// TEMPORARY: Workaround LXD bug with empty dir unmounts
	revertWorkaround, err := s.lxdEmptyUnmountWorkaround(conn, ctx, projectId, workshop, profile)
	if err != nil {
		return err
	}
	defer revertWorkaround.Fail()

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
