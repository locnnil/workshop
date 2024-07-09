package lxdbackend

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/canonical/lxd/shared/api"
	"golang.org/x/exp/slices"

	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/workshop"
)

func profileName(pid, workshop, sdk string) string {
	return strings.Join([]string{InstanceName(workshop, pid), sdk}, "-")
}

func (s *Backend) AssignProfile(ctx context.Context, w string, profile workshop.SdkProfile) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	fs, err := s.WorkshopFs(ctx, w)
	if err != nil {
		return err
	}
	defer fs.Close()
	for _, dev := range profile.Devices {
		if dev.Type == workshop.BindMount {
			// confirm the target path exists
			target := dev.Properties["path"]
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

		if dev.Type == workshop.SshAgentProxy {
			// add SSH_AUTH_SOCK variable to the workshop's environment
			if err = setSshAuthSock(fs, dev, w); err != nil {
				return err
			}
		}
	}

	lxdname := profileName(projectId, w, profile.Name())
	lxddevs := make(map[string]map[string]string)
	for _, dev := range profile.Devices {
		if lxddevs[dev.Name] == nil {
			lxddevs[dev.Name] = make(map[string]string)
		}
		maps.Copy(lxddevs[dev.Name], dev.Properties)
	}
	newProfile := api.ProfilePut{
		Devices:     lxddevs,
		Description: fmt.Sprintf("%q SDK profile for %q workshop", profile.Name(), w),
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

	inst, etag, err := conn.GetInstance(InstanceName(w, projectId))
	if err != nil {
		return err
	}

	if slices.Index(inst.Profiles, lxdname) == -1 {
		// Assigning the profile for the first time
		put := inst.InstancePut
		put.Profiles = append(put.Profiles, lxdname)
		op, err := conn.UpdateInstance(InstanceName(w, projectId), put, etag)
		if err != nil {
			return err
		}

		return op.WaitContext(ctx)
	}

	return nil
}

func setSshAuthSock(fs workshop.WorkshopFs, dev workshop.Device, workshop string) error {
	env, err := fs.Create(filepath.Join("/etc/profile.d", dev.Name+".sh"))
	if err != nil {
		return fmt.Errorf("cannot set SSH_AUTH_SOCK for %q: %w", workshop, err)
	}

	_, err = env.Write([]byte("export SSH_AUTH_SOCK=" + strings.TrimPrefix(dev.Properties["listen"], "unix:")))
	if err != nil {
		return fmt.Errorf("cannot set SSH_AUTH_SOCK for %q: %w", workshop, err)
	}
	_ = env.Close()
	return nil
}

func unsetSshAuthSock(fs workshop.WorkshopFs, name string) {
	_ = fs.Remove(filepath.Join("/etc/profile.d", name+".sh"))
}

func (s *Backend) RemoveProfile(ctx context.Context, w string, profile string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(InstanceName(w, projectId))
	if err != nil {
		return err
	}

	// 1. Unassign the profile from the workshop
	lxdname := profileName(projectId, w, profile)
	if idx := slices.Index(inst.Profiles, lxdname); idx != -1 {
		inst.Profiles = slices.Delete(inst.Profiles, idx, idx+1)
		op, err := conn.UpdateInstance(InstanceName(w, projectId), inst.Writable(), etag)
		if err != nil {
			return err
		}
		if err = op.Wait(); err != nil {
			return err
		}
	}

	// 2. Delete the profile
	err = conn.DeleteProfile(profileName(projectId, w, profile))
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return workshop.ErrSdkProfileNotFound
	}
	return err
}

func (s *Backend) Profile(ctx context.Context, wp, profile string) (workshop.SdkProfile, error) {
	var pr = workshop.NewSdkProfile(profile)
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return pr, err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return pr, fmt.Errorf("context key project-id not found")
	}

	lxdname := profileName(projectId, wp, profile)
	lxdp, _, err := conn.GetProfile(lxdname)
	if err != nil {
		return pr, err
	}
	for name, dev := range lxdp.Devices {
		switch dev["type"] {
		case "disk":
			pr.Devices[name] = Mount(name, dev["source"], dev["path"])
		case "gpu":
			pr.Devices[name] = Gpu(name)
		case "proxy":
			pr.Devices[name] = SshAgent(name, dev["connect"], dev["listen"])
		}
	}
	return pr, nil
}
