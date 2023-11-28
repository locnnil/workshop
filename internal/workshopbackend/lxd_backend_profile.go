package workshopbackend

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/lxc/lxd/shared/api"
)

func profileName(pid, workshop, sdk string) string {
	return strings.Join([]string{InstanceName(workshop, pid), sdk}, "-")
}

func (w WorkshopDevice) lxdProperties() map[string]string {
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

	lxdname := profileName(projectId, workshop, profile.Name())
	newProfile := api.ProfilePut{
		Devices:     profile.lxdDevices(),
		Description: fmt.Sprintf("%s SDK profile for %s workshop", profile.Name(), workshop),
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
		// Assigning profile for the first time
		put := inst.InstancePut
		put.Profiles = append(put.Profiles, lxdname)
		op, err := conn.UpdateInstance(InstanceName(workshop, projectId), put, etag)
		if err != nil {
			return err
		}

		return op.WaitContext(ctx)
	}

	// Do nothing if the profile has been assigned already
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
