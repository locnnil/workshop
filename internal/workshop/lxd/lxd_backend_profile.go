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

	"github.com/canonical/workshop/internal/logger"
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

	uname, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key user not found")
	}

	user, err := workshop.LookupUsername(uname)
	if err != nil {
		return err
	}

	pid, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	p, err := s.loadProjectFromId(conn, ctx, pid)
	if err != nil {
		return err
	}

	fs, err := s.WorkshopFs(ctx, w)
	if err != nil {
		return err
	}
	defer fs.Close()
	for _, dev := range profile.Devices {
		if dev.Type == workshop.HostWorkshopMount {
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
				return fmt.Errorf(`%s:%s's "workshop-target" %s is not a directory`, profile.Sdk, dev.Name, target)
			}

			source := dev.Properties["source"]
			// A path relative to the project directory (from the slot
			// declared in a workshop).
			if filepath.IsLocal(source) {
				abs := filepath.Join(p.Path, source)
				// Ensure that the source path exists here. LXD allows to
				// require the source attribute when updating an instance
				// configuration but it would fail and still save changes to the
				// instace profile even if the source does not exist. For
				// Workshop that would mean that the interface connection would
				// fail but there will still be changes made to the instance
				// configuration which is not acceptable.
				if !osutil.IsDir(abs) {
					return fmt.Errorf(`%s:%s's "source" %s is not an existing directory`, profile.Sdk, dev.Name, abs)
				}
				dev.Properties["source"] = abs
				continue
			}

			// The dir is being dynamically created (no source attribute
			// provided by the slot).
			if !osutil.IsDir(source) {
				uid, gid, err := osutil.UidGid(user)
				if err != nil {
					return err
				}

				if err = osutil.MkdirAllChown(source, 0744, uid, gid); err != nil {
					return err
				}
			}
		}

		if dev.Type == workshop.SshAgentProxy {
			// add SSH_AUTH_SOCK variable to the workshop's environment
			if err = installSshAgent(fs, dev, w); err != nil {
				return err
			}
		}
	}

	lxdname := profileName(pid, w, profile.Name())
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
	// it can be assigned to the required workshop.
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
					removeSshAgent(fs, key)
				}
			}
		}
		if err := conn.UpdateProfile(lxdname, newProfile, etag); err != nil {
			return err
		}
	}

	inst, etag, err := conn.GetInstance(InstanceName(w, pid))
	if err != nil {
		return err
	}

	if !slices.Contains(inst.Profiles, lxdname) {
		// Assigning the profile for the first time.
		put := inst.InstancePut
		put.Profiles = append(put.Profiles, lxdname)
		op, err := conn.UpdateInstance(InstanceName(w, pid), put, etag)
		if err != nil {
			return err
		}

		return op.WaitContext(ctx)
	}

	return nil
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
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return pr, workshop.ErrSdkProfileNotFound
		}
		return pr, err
	}
	for name, dev := range lxdp.Devices {
		switch dev["type"] {
		case "disk":
			pr.Devices[name] = HostWorkshopMount(name, dev["source"], dev["path"])
		case "gpu":
			pr.Devices[name] = Gpu(name)
		case "proxy":
			pr.Devices[name] = SshAgent(name, dev["connect"], dev["listen"])
		default:
			logger.Noticef("On reading %q SDK profile: unknown device type: %s", profile, dev["type"])
		}
	}
	return pr, nil
}
