package lxdbackend

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"gopkg.in/yaml.v3"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/revert"
	"github.com/canonical/workshop/internal/workshop"
)

func (s *Backend) Snapshot(ctx context.Context, name, sk string) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	op, err := conn.CreateInstanceSnapshot(InstanceName(name, projectId), api.InstanceSnapshotsPost{
		Name: name + "." + sk,
	})
	if err != nil {
		return err
	}
	return op.Wait()
}

func (s *Backend) Restore(ctx context.Context, name, sk string, file *workshop.File) error {
	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}

	instPut := inst.Writable()
	instPut.Restore = name + "." + sk
	op, err := conn.UpdateInstance(inst.Name, instPut, etag)
	if err != nil {
		return err
	}
	if err = op.Wait(); err != nil {
		return err
	}

	restored, etag, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		return err
	}

	f, err := yaml.Marshal(file)
	if err != nil {
		return err
	}

	// The restored snapshot will have an updated file.
	// Similar to how a launched workshop is associated with its definition.
	restored.Config[workshop.ConfigWorkshopFile] = string(f)

	op, err = conn.UpdateInstance(inst.Name, restored.Writable(), etag)
	if err != nil {
		return err
	}
	if err = op.Wait(); err != nil {
		return err
	}
	return nil
}

func (s *Backend) StashWorkshop(ctx context.Context, name string) error {
	rev := revert.New()
	defer rev.Fail()

	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	if err := s.stopWorkshop(conn, ctx, name, true); err != nil {
		return err
	}

	rev.Add(func() {
		if rerr := s.startWorkshop(conn, ctx, name); rerr != nil {
			logger.Debugf("On StashWorkshop: Cannot restart %q workshop after failed stash operation: %v", name, rerr)
		}
	})

	user, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key %s not found", workshop.ContextUser)
	}

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	instance := InstanceName(name, projectId)
	stashed := instanceStashName(name, projectId)

	sourceProject, err1 := lxdProjectName(user)
	targetProject, err2 := lxdLayersProjectName(user)
	if err = cmp.Or(err1, err2); err != nil {
		return err
	}

	if err = s.copyInstance(conn, instance, stashed, sourceProject, targetProject, false); err != nil {
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

	conn, err := s.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	instance := InstanceName(name, projectId)
	stash := instanceStashName(name, projectId)

	sourceProject, err1 := lxdLayersProjectName(user)
	targetProject, err2 := lxdProjectName(user)
	if err = cmp.Or(err1, err2); err != nil {
		return err
	}

	if err := s.copyInstance(conn, stash, instance, sourceProject, targetProject, true); err != nil {
		return err
	}

	return s.startWorkshop(conn, ctx, name)
}

// Copies the instance between LXD projects.
func (s *Backend) copyInstance(conn lxd.InstanceServer, srcName, dstName, sourceProject, targetProject string, refresh bool) error {
	conn = conn.UseProject(sourceProject)
	srcInst, _, err := conn.GetInstance(srcName)
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return workshop.ErrWorkshopNotLaunched
		}
		return err
	}

	// Clear the config and device overrides. LXD will still copy most of
	// these from the source instance, but it will omit options which are
	// unique to the source instance, like MAC addresses and UUIDs.
	config := srcInst.Config
	devices := srcInst.Devices
	srcInst.Config = nil
	srcInst.Devices = nil

	dest := conn.UseProject(targetProject)
	rop, err := dest.CopyInstance(conn, *srcInst, &lxd.InstanceCopyArgs{Name: dstName, Refresh: refresh})
	if err != nil {
		return err
	}
	if err = rop.Wait(); err != nil {
		return err
	}

	if !refresh {
		// LXD created a new instance with copied config and devices.
		return nil
	}

	// LXD replaced an existing instance's root disk, but it's our job to
	// copy the config and devices.
	dstInst, etag, err := dest.GetInstance(dstName)
	if err != nil {
		return err
	}

	// The main use case is to restore an instance from a backup, so we
	// replace all options which should be present in the backup. Other
	// options are preserved. Most of these will be constant for the
	// instance's lifetime, and we assume they are all managed by LXD.
	maps.DeleteFunc(dstInst.Config, func(k, v string) bool { return includeWhenCopying(k) })
	maps.DeleteFunc(config, func(k, v string) bool { return !includeWhenCopying(k) })
	if dstInst.Config == nil {
		dstInst.Config = config
	} else {
		maps.Copy(dstInst.Config, config)
	}

	dstInst.Devices = devices

	op, err := dest.UpdateInstance(dstName, dstInst.Writable(), etag)
	if err != nil {
		return err
	}
	return op.Wait()
}

// Based on LXD's InstanceIncludeWhenCopying. These are the options which
// CopyInstance adds to a newly created instance by default (i.e. when
// api.Instance.Config is empty).
func includeWhenCopying(key string) bool {
	suffix, found := strings.CutPrefix(key, "volatile.")
	return !found || slices.Contains([]string{"base_image", "last_state.idmap"}, suffix)
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

	layers, err := lxdLayersProjectName(user)
	if err != nil {
		return err
	}

	conn = conn.UseProject(layers)

	op, err := conn.DeleteInstance(instanceStashName(name, projectId))
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return nil
		}
		return err
	}
	if err = op.Wait(); err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return nil
		}
		return err
	}
	return nil
}
