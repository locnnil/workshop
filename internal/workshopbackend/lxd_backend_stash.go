package workshopbackend

import (
	"context"
	"fmt"
	"net/http"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/revert"
	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
)

var StashNamePrefix string = "stash-"

func (s *LxdBackend) StashWorkshop(ctx context.Context, name string) error {
	rev := revert.New()
	defer rev.Fail()

	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", ContextUser)
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	instance := InstanceName(name, projectId)
	stashedInsance := StashNamePrefix + instance
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	if err := s.updateInstanceState(conn, ctx, name, "stop", false); err != nil {
		return err
	}

	rev.Add(func() {
		err := s.updateInstanceState(conn, ctx, name, "start", false)
		if err != nil {
			logger.Debugf("Cannot restart %q workshop after failed stash operation", name)
		}
	})

	if err = s.moveInstanceAndProfiles(conn, ctx, instance, stashedInsance, LxdProjectName(user), LxdSystemProjectName(user)); err != nil {
		return err
	}

	rev.Success()
	return nil
}

func (s *LxdBackend) UnstashWorkshop(ctx context.Context, name string) error {
	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", ContextUser)
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	instance := InstanceName(name, projectId)
	stashedInsance := StashNamePrefix + instance
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	if err := s.moveInstanceAndProfiles(conn, ctx, stashedInsance, instance, LxdSystemProjectName(user), LxdProjectName(user)); err != nil {
		return err
	}

	if err := s.updateInstanceState(conn, ctx, name, "start", false); err != nil {
		return err
	}
	return nil
}

// Moves the instance between the project and stash area (or the other way around)
// instanceFrom - the instance's source name
// instanceTo - the instance's dest name (must be different due to LXD DNS conflicts)
// source - the LXD project name to move instance from
// target - the LXD project name to move instance to
func (s *LxdBackend) moveInstanceAndProfiles(conn lxd.InstanceServer, ctx context.Context, instanceFrom, instanceTo, source, target string) error {
	rev := revert.New()
	defer rev.Fail()

	conn = conn.UseProject(source)
	inst, etag, err := conn.GetInstance(instanceFrom)
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return ErrWorkshopNotFound
		}
		return err
	}

	var profiles = make([]string, len(inst.Profiles))
	copy(profiles, inst.Profiles)
	inst.Profiles = []string{"default"}

	// 1. Unassign all the isntances profiles except default
	if op, err := conn.UpdateInstance(inst.Name, inst.Writable(), etag); err != nil {
		return err
	} else if err = op.WaitContext(ctx); err != nil {
		return err
	}

	// if stashing to another project fails, we have to restore
	// the instance configuration as is.
	rev.Add(func() {
		inst.Profiles = profiles
		if op, err := conn.UpdateInstance(inst.Name, inst.Writable(), ""); err != nil {
			op.WaitContext(ctx)
		}
	})

	// 2. Stash the workshop
	// the new name must not be the same, otherwise the LXD's DNS will fail
	// the new instance creation; hence, the prefix.
	conn = conn.UseProject(source)
	if op, err := conn.MigrateInstance(instanceFrom, api.InstancePost{
		Name:      instanceTo,
		Project:   target,
		Migration: true,
	}); err != nil {
		return err
	} else if err = op.Wait(); err != nil {
		return err
	}

	rev.Success()

	// 3. Remove the SDK profiles as the workshop is successfully stashed now.
	conn = conn.UseProject(source)
	if err := s.removeProfiles(conn, profiles); err != nil {
		return err
	}
	return nil
}

func (s *LxdBackend) removeProfiles(conn lxd.InstanceServer, profiles []string) error {
	for _, p := range profiles {
		if p == "default" {
			continue
		}

		profile, _, err := conn.GetProfile(p)
		if err != nil {
			return err
		}

		if err := conn.DeleteProfile(profile.Name); err != nil {
			return err
		}
	}
	return nil
}

func (s *LxdBackend) RemoveWorkshopStash(ctx context.Context, name string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}

	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", ContextUser)
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	conn = conn.UseProject(LxdSystemProjectName(user))
	iname := StashNamePrefix + InstanceName(name, projectId)
	inst, _, err := conn.GetInstance(iname)
	if err != nil {
		return err
	}

	// 1. Remove the workshop instance
	op, err := conn.DeleteInstance(iname)
	if err != nil {
		return err
	} else if err = op.WaitContext(ctx); err != nil {
		return nil
	}

	// 2. Remove the stashed profiles
	if err := s.removeProfiles(conn, inst.Profiles); err != nil {
		return err
	}
	return op.WaitContext(ctx)
}
