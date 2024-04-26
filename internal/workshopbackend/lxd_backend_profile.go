package workshopbackend

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/canonical/lxd/shared/api"
	"golang.org/x/exp/slices"

	"github.com/canonical/workshop/internal/osutil"
)

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

		if dev.Type() == SshAgentProxy {
			// add SSH_AUTH_SOCK variable to the workshop's environment
			if err = setSshAuthSock(fs, dev, workshop); err != nil {
				return err
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
	oldProfile, etag, err := conn.GetProfile(lxdname)
	if err != nil && !api.StatusErrorCheck(err, http.StatusNotFound) {
		return err
	} else if api.StatusErrorCheck(err, http.StatusNotFound) {
		if err = conn.CreateProfile(api.ProfilesPost{ProfilePut: newProfile, Name: lxdname}); err != nil {
			return err
		}
	} else {
		// Find the difference between a set of old and new devices to detect if any
		// clean up is required when a new profile will be assigned (updated).
		for key, dev := range oldProfile.Devices {
			if _, ok := newProfile.Devices[key]; !ok {
				if dev["type"] == "proxy" {
					unsetSshAuthSock(fs, key)
				}
			}
		}
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

func setSshAuthSock(fs WorkshopFs, dev Device, workshop string) error {
	env, err := fs.Create(filepath.Join("/etc/profile.d", dev.name+".sh"))
	if err != nil {
		return fmt.Errorf("cannot set SSH_AUTH_SOCK for %q: %w", workshop, err)
	}

	_, err = env.Write([]byte("export SSH_AUTH_SOCK=" + strings.TrimPrefix(dev.properties["listen"], "unix:")))
	if err != nil {
		return fmt.Errorf("cannot set SSH_AUTH_SOCK for %q: %w", workshop, err)
	}
	_ = env.Close()
	return nil
}

func unsetSshAuthSock(fs WorkshopFs, name string) {
	_ = fs.Remove(filepath.Join("/etc/profile.d", name+".sh"))
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

	// 1. Unassign the profile from the workshop
	lxdname := profileName(projectId, workshop, profile)
	if idx := slices.Index(inst.Profiles, lxdname); idx != -1 {
		inst.Profiles = slices.Delete(inst.Profiles, idx, idx+1)
		op, err := conn.UpdateInstance(InstanceName(workshop, projectId), inst.Writable(), etag)
		if err != nil {
			return err
		}
		if err = op.Wait(); err != nil {
			return err
		}
	}

	// 2. Delete the profile
	err = conn.DeleteProfile(profileName(projectId, workshop, profile))
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return ErrSdkProfileNotFound
	}
	return err
}
