package lxdbackend

import (
	"context"
	"fmt"
	"net/http"

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
	instance, _, err := conn.GetInstance(srcName)
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return workshop.ErrWorkshopNotLaunched
		}
		return err
	}

	dest := conn
	dest = dest.UseProject(targetProject)
	instance.Profiles = []string{}

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
