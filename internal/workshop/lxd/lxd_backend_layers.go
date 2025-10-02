package lxdbackend

import (
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

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	conn, layerConn, err := s.layerClients(ctx)
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

	instance := InstanceName(name, projectId)
	stashed := instanceStashName(name, projectId)
	if err := s.createLayer(conn, layerConn, instance, stashed); err != nil {
		return err
	}

	rev.Success()
	return nil
}

func (s *Backend) UnstashWorkshop(ctx context.Context, name string) error {
	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	conn, layerConn, err := s.layerClients(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	instance := InstanceName(name, projectId)
	stash := instanceStashName(name, projectId)
	if err := s.restoreLayer(conn, layerConn, instance, stash); err != nil {
		return err
	}

	return s.startWorkshop(conn, ctx, name)
}

func (s *Backend) createLayer(conn, layerConn lxd.InstanceServer, inst, layer string) error {
	return s.copyInstance(conn, layerConn, inst, layer, false)
}

func (s *Backend) restoreLayer(conn, layerConn lxd.InstanceServer, inst, layer string) error {
	return s.copyInstance(layerConn, conn, layer, inst, true)
}

// Copies the instance between LXD projects.
func (s *Backend) copyInstance(src, dst lxd.InstanceServer, srcName, dstName string, refresh bool) error {
	srcInst, _, err := src.GetInstance(srcName)
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return workshop.ErrWorkshopNotLaunched
		}
		return err
	}

	// Clear the config and device overrides. LXD will still copy most of
	// these from the source instance, but it will omit options which are
	// unique to the source instance, like MAC addresses and UUIDs.
	req := *srcInst
	req.Config = nil
	req.Devices = nil

	rop, err := dst.CopyInstance(src, req, &lxd.InstanceCopyArgs{Name: dstName, Refresh: refresh})
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
	dstInst, etag, err := dst.GetInstance(dstName)
	if err != nil {
		return err
	}

	// The main use case is to restore an instance from a backup, so we
	// replace all options which should be present in the backup. Other
	// options are preserved. Most of these will be constant for the
	// instance's lifetime, and we assume they are all managed by LXD.
	maps.DeleteFunc(dstInst.Config, func(k, v string) bool { return includeWhenCopying(k) })
	maps.DeleteFunc(srcInst.Config, func(k, v string) bool { return !includeWhenCopying(k) })
	if dstInst.Config == nil {
		dstInst.Config = srcInst.Config
	} else {
		maps.Copy(dstInst.Config, srcInst.Config)
	}

	dstInst.Devices = srcInst.Devices

	op, err := dst.UpdateInstance(dstName, dstInst.Writable(), etag)
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
	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	_, conn, err := s.layerClients(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	return s.deleteLayer(conn, instanceStashName(name, projectId))
}

func (s *Backend) deleteLayer(layerConn lxd.InstanceServer, layer string) error {
	op, err := layerConn.DeleteInstance(layer)
	if err == nil {
		err = op.Wait()
	}
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return nil
	}
	return err
}

func (s *Backend) layerClients(ctx context.Context) (lxd.InstanceServer, lxd.InstanceServer, error) {
	user, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return nil, nil, fmt.Errorf("context key %s not found", workshop.ContextUser)
	}

	project, err := lxdLayersProjectName(user)
	if err != nil {
		return nil, nil, err
	}

	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, nil, err
	}

	return conn, conn.UseProject(project), nil
}
