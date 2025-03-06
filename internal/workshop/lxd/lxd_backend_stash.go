package lxdbackend

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/revert"
	"github.com/canonical/workshop/internal/workshop"
)

func (s *Backend) StashWorkshop(ctx context.Context, name string) error {
	rev := revert.New()
	defer rev.Fail()

	user, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", workshop.ContextUser)
	}

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	instance := InstanceName(name, projectId)
	stashedInsance := workshop.StashNamePrefix + instance
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	if err := s.updateInstanceState(conn, ctx, name, "stop", true); err != nil {
		return err
	}

	rev.Add(func() {
		err := s.updateInstanceState(conn, ctx, name, "start", false)
		if err != nil {
			logger.Debugf("Cannot restart %q workshop after failed stash operation", name)
		}
	})

	if err = s.copyInstance(conn, instance, stashedInsance, LxdProjectName(user), LxdSystemProjectName(user)); err != nil {
		return err
	}

	rev.Success()
	return nil
}

func (s *Backend) UnstashWorkshop(ctx context.Context, name string) error {
	user, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", workshop.ContextUser)
	}

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	instance := InstanceName(name, projectId)
	stashedInsance := workshop.StashNamePrefix + instance
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	if err := s.copyInstance(conn, stashedInsance, instance, LxdSystemProjectName(user), LxdProjectName(user)); err != nil {
		return err
	}

	if err := s.updateInstanceState(conn, ctx, name, "start", false); err != nil {
		return err
	}
	return nil
}

// Moves the instance between LXD projects.
func (s *Backend) copyInstance(conn lxd.InstanceServer, srcName, dstName, sourceProject, targetProject string) error {
	conn = conn.UseProject(sourceProject)
	instance, etag, err := conn.GetInstance(srcName)
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return workshop.ErrWorkshopNotLaunched
		}
		return err
	}

	dest := conn
	dest = dest.UseProject(targetProject)

	// Profiles need reseting before making a copy to workaround:
	// https://github.com/canonical/lxd/issues/15078#issue-2883386254
	oldProfiles := slices.Clone(instance.Profiles)
	instance.Profiles = []string{"default"}

	op, err := conn.UpdateInstance(instance.Name, instance.Writable(), etag)
	if err != nil {
		return err
	}
	if err = op.Wait(); err != nil {
		return err
	}

	rev := revert.New()
	defer rev.Fail()

	rev.Add(func() {
		instance.Profiles = oldProfiles
		op, rerr := conn.UpdateInstance(instance.Name, instance.Writable(), "")
		if rerr != nil {
			logger.Noticef("On copyInstance: cannot revert profiles assigned to %q workshop: %v", srcName, rerr)
		}
		if rerr = op.Wait(); rerr != nil {
			logger.Noticef("On copyInstance: cannot revert profiles assigned to %q workshop: %v", srcName, rerr)
		}
	})

	rop, err := dest.CopyInstance(conn, *instance, &lxd.InstanceCopyArgs{Name: dstName})
	if err != nil {
		return err
	}
	if err = rop.Wait(); err != nil {
		return err
	}

	return nil
}

func (s *Backend) RemoveWorkshopStash(ctx context.Context, name string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	user, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", workshop.ContextUser)
	}

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	conn = conn.UseProject(LxdSystemProjectName(user))
	iname := workshop.StashNamePrefix + InstanceName(name, projectId)

	op, err := conn.DeleteInstance(iname)
	if err != nil {
		return err
	}
	if err = op.Wait(); err != nil {
		return err
	}
	return nil
}
