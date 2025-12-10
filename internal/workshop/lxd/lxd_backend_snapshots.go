package lxdbackend

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/revert"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
)

// Classifies LXD config options into different domains which describe who is
// responsible for them and how they (should) behave when copying an instance.
type configDomain int

const (
	// Properties unique to a single instance. These are managed by LXD and
	// not shared with instance copies, e.g. volatile.uuid.
	uniqueProperty configDomain = iota
	// Properties managed by LXD and shared with copies, e.g. image.os.
	sharedProperty
	// Options managed by Workshop. Includes instance config options like
	// boot.autostart and workshop metadata like user.workshop.project-id.
	customOption
)

// Classifies options by key. Based on LXD's InstanceIncludeWhenCopying.
func optionDomain(key string) configDomain {
	if strings.HasPrefix(key, "image.") {
		return sharedProperty
	}

	suffix, found := strings.CutPrefix(key, "volatile.")
	if !found {
		return customOption
	}
	switch suffix {
	case "base_image", "last_state.idmap":
		return sharedProperty
	default:
		return uniqueProperty
	}
}

func (s *Backend) Snapshot(ctx context.Context, name, sk string) error {
	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	conn, snapshotConn, err := s.snapshotClients(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	inst, _, err := conn.GetInstance(InstanceName(name, projectId))
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return workshop.ErrWorkshopNotLaunched
		}
		return err
	}

	f, err := workshopFile(inst.Config)
	if err != nil {
		return fmt.Errorf("cannot load workshop: %v", err)
	}
	image := workshop.BaseImage{
		Name:        f.Base,
		Fingerprint: inst.Config[workshop.ConfigWorkshopBaseFingerprint],
	}

	// Override all writable attributes. The snapshot is essentially a base
	// image but is stored more efficiently. Config options and devices are
	// reconstructed from scratch during launch and refresh.
	config := configOverrides(projectId, name, sk, image, inst.Config)
	devices, err := deviceOverrides(inst.Devices)
	if err != nil {
		return err
	}
	inst.SetWritable(api.InstancePut{
		Architecture: inst.Architecture,
		Config:       config,
		Devices:      devices,
		Profiles:     []string{},
	})

	snapshot, err := sdkSnapshotName(sk)
	if err != nil {
		return err
	}
	args := lxd.InstanceCopyArgs{Name: snapshot, InstanceOnly: true}
	rop, err := snapshotConn.CopyInstance(conn, *inst, &args)
	if err != nil {
		return err
	}
	return rop.Wait()
}

func configOverrides(pid, name, sk string, image workshop.BaseImage, config map[string]string) map[string]string {
	// LXD will handle the options it owns. We remove all options set by
	// Workshop, except for a handful that identify the snapshot. We also
	// add security.protection.start to prevent it from starting.
	overrides := map[string]string{
		"security.protection.start":            "true",
		workshop.ConfigProjectId:               pid,
		workshop.ConfigWorkshopName:            name,
		workshop.ConfigWorkshopBase:            image.Name,
		workshop.ConfigWorkshopBaseFingerprint: image.Fingerprint,
		workshop.ConfigWorkshopSnapshotType:    "sdk",
		workshop.ConfigWorkshopSdk:             sk,
	}
	for k := range config {
		if _, ok := overrides[k]; !ok && optionDomain(k) == customOption {
			overrides[k] = ""
		}
	}
	return overrides
}

func deviceOverrides(devices map[string]map[string]string) (map[string]map[string]string, error) {
	// There's no easy way to remove these, so we make do by setting them
	// to {"type": "none"}. For SDKs we remember all metadata which might
	// affect the snapshot.
	overrides := make(map[string]map[string]string, len(devices))
	none := map[string]string{"type": "none"}
	for key, device := range devices {
		s, err := maybeSdkInstallation(key, device)
		if err != nil {
			return nil, err
		}
		if s != nil {
			overrides[key] = sdkToSnapshotDevice(s.InstallOrder, sdk.SetupId(s.Setup))
		} else if key != "root" {
			overrides[key] = none
		}
	}
	return overrides, nil
}

// Generates a random suffix for the SDK, to avoid clashing with SDK snapshots
// from other projects and workshops, while remaining within the 63 characters
// allowed for LXD instance names.
func sdkSnapshotName(sk string) (string, error) {
	bytes := make([]byte, 8)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return sk + "-" + hex.EncodeToString(bytes), nil
}

func (s *Backend) launchOrRebuildFromSnapshot(conn, snapshotConn lxd.InstanceServer, req api.InstancesPost, sdks []sdk.Id) error {
	projectId := req.Config[workshop.ConfigProjectId]
	name := req.Config[workshop.ConfigWorkshopName]

	// Find snapshots to keep and remove.
	lastSdk := sdks[len(sdks)-1].Name
	snapshotName, obstacles, err := s.snapshotNamesAfter(snapshotConn, projectId, name, lastSdk)
	if err != nil {
		return err
	}

	snapshot, _, err := snapshotConn.GetInstance(snapshotName)
	if err != nil {
		return err
	}
	// Snapshots should only contain the requested devices and SDK mounts.
	for key, device := range snapshot.Devices {
		installOrder, s, err := maybeSdkId(key, device)
		if err != nil {
			return err
		}
		if s != nil {
			if 0 < installOrder && installOrder <= len(sdks) && sdks[installOrder-1] == *s {
				// SDK will be reinstalled as a separate task.
				continue
			}
			return fmt.Errorf("internal error: snapshot %q has unexpected %q SDK", snapshot.Name, s.Name)
		} else if _, ok := req.Devices[key]; !ok {
			return fmt.Errorf("internal error: snapshot %q has unexpected device %q", snapshot.Name, key)
		}
	}

	// Remove the newer snapshots, as LXD would do. This is not necessary
	// to "restore the snapshot," i.e. replace the volume with a new one,
	// but we want to avoid having two snapshots for the same SDK and
	// workshop until we introduce a way to distinguish them.
	for _, obstacle := range obstacles {
		if err := s.deleteSnapshot(snapshotConn, obstacle); err != nil {
			return err
		}
	}

	overrides := overrideSnapshot(*snapshot, req.InstancePut)
	args := lxd.InstanceCopyArgs{
		Name:         req.Name,
		InstanceOnly: true,
		Refresh:      true,
	}
	rop, err := conn.CopyInstance(snapshotConn, overrides, &args)
	if err != nil {
		return err
	}
	if err = rop.Wait(); err != nil {
		return err
	}

	// If the workshop already existed, it still has the old config options
	// and devices. If it did not exist, the config and devices are mostly
	// correct, but we still need to remove placeholder SDK devices.
	inst, etag, err := conn.GetInstance(req.Name)
	if err != nil {
		return err
	}
	req.Config = mergeOptions(req.Config, snapshot.Config, inst.Config)
	req.Architecture = inst.Architecture
	op, err := conn.UpdateInstance(req.Name, req.InstancePut, etag)
	if err != nil {
		return err
	}
	return op.Wait()
}

func overrideSnapshot(snapshot api.Instance, req api.InstancePut) api.Instance {
	// Set all requested options and remove snapshot options that aren't
	// managed by LXD.
	req.Config = maps.Clone(req.Config)
	for k := range snapshot.Config {
		if _, ok := req.Config[k]; !ok && optionDomain(k) == customOption {
			req.Config[k] = ""
		}
	}
	// Ensure LXD doesn't copy profiles from the snapshot.
	if req.Profiles == nil {
		req.Profiles = []string{}
	}
	req.Architecture = snapshot.Architecture
	snapshot.SetWritable(req)
	return snapshot
}

func mergeOptions(req, source, target map[string]string) map[string]string {
	// Start with requested options.
	result := maps.Clone(req)
	// Next, add LXD-managed options we want to preserve from the workshop.
	// Ideally some of these probably shouldn't be preserved. See
	// https://github.com/canonical/lxd/issues/16667.
	for k, v := range target {
		if _, ok := result[k]; !ok && optionDomain(k) == uniqueProperty {
			result[k] = v
		}
	}
	// Finally, copy the remaining LXD-managed options from the snapshot.
	for k, v := range source {
		if _, ok := result[k]; !ok && optionDomain(k) == sharedProperty {
			result[k] = v
		}
	}
	return result
}

func (s *Backend) StashWorkshop(ctx context.Context, name string) error {
	rev := revert.New()
	defer rev.Fail()

	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	conn, snapshotConn, err := s.snapshotClients(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	if err := s.stopWorkshop(conn, ctx, name, true); err != nil {
		return err
	}

	rev.Add(func() {
		if rerr := s.startWorkshop(conn, ctx, name); rerr != nil {
			logger.Noticef("On StashWorkshop: Cannot restart %q workshop after failed stash operation: %v", name, rerr)
		}
	})

	snapshots, err := s.snapshotNames(snapshotConn, projectId, name, "sdk")
	if err != nil {
		return err
	}

	// Backup the workshop's SDK snapshots, modifying the instance name and
	// snapshot type to distinguish the backups from the originals.
	for _, snapshot := range snapshots {
		newname := "stash-" + snapshot
		config := map[string]string{workshop.ConfigWorkshopSnapshotType: "stash-sdk"}
		if err := s.copyInstance(snapshotConn, snapshotConn, snapshot, newname, false, config); err != nil {
			return err
		}
		rev.Add(func() {
			if rerr := s.deleteSnapshot(snapshotConn, newname); rerr != nil {
				logger.Noticef("On StashWorkshop: Cannot remove snapshot %q after failed stash operation: %v", newname, rerr)
			}
		})
	}

	// Backup the workshop itself.
	instance := InstanceName(name, projectId)
	stashed := instanceStashName(name, projectId)
	// Mark the copy as a stash to avoid confusing it with an SDK snapshot.
	config := map[string]string{workshop.ConfigWorkshopSnapshotType: "stash"}
	if err := s.copyInstance(conn, snapshotConn, instance, stashed, false, config); err != nil {
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

	conn, snapshotConn, err := s.snapshotClients(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	snapshots, err := s.snapshotNames(snapshotConn, projectId, name, "sdk")
	if err != nil {
		return err
	}
	stashSnapshots, err := s.snapshotNames(snapshotConn, projectId, name, "stash-sdk")
	if err != nil {
		return err
	}

	// Remove the snapshots that were created after stashing. These are the
	// ones which don't have backups.
	for _, snapshot := range snapshots {
		if slices.Contains(stashSnapshots, "stash-"+snapshot) {
			continue
		}
		if err := s.deleteSnapshot(snapshotConn, snapshot); err != nil {
			return err
		}
	}

	// Restore the snapshots that were deleted when restoring an SDK
	// snapshot. These come from the backups whose source no longer exists.
	for _, snapshot := range stashSnapshots {
		name := strings.TrimPrefix(snapshot, "stash-")
		if slices.Contains(snapshots, name) {
			continue
		}
		// Restore the original snapshot type (stash-sdk -> sdk).
		config := map[string]string{workshop.ConfigWorkshopSnapshotType: "sdk"}
		if err := s.copyInstance(snapshotConn, snapshotConn, snapshot, name, false, config); err != nil {
			return err
		}
	}

	// Restore the workshop itself.
	instance := InstanceName(name, projectId)
	stash := instanceStashName(name, projectId)
	// Avoid restoring the option which we added when stashing.
	config := map[string]string{workshop.ConfigWorkshopSnapshotType: ""}
	if err := s.copyInstance(snapshotConn, conn, stash, instance, true, config); err != nil {
		return err
	}

	return s.startWorkshop(conn, ctx, name)
}

// Find snapshot names for all SDKs in the given workshop or stash.
func (s *Backend) snapshotNames(snapshotConn lxd.InstanceServer, pid, w string, kind string) ([]string, error) {
	filters := []string{
		"config.user.workshop.project-id=" + pid,
		"config.user.workshop.name=" + w,
		"config.user.workshop.snapshot-type=" + kind,
	}
	snapshots, err := snapshotConn.GetInstancesWithFilter(api.InstanceTypeContainer, filters)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(snapshots))
	for _, snapshot := range snapshots {
		names = append(names, snapshot.Name)
	}
	return names, nil
}

// Find snapshot name for the given SDK and any subsequent SDKs.
func (s *Backend) snapshotNamesAfter(snapshotConn lxd.InstanceServer, pid, w, sk string) (string, []string, error) {
	filters := []string{
		"config.user.workshop.project-id=" + pid,
		"config.user.workshop.name=" + w,
		"config.user.workshop.snapshot-type=sdk",
	}
	snapshots, err := snapshotConn.GetInstancesWithFilter(api.InstanceTypeContainer, filters)
	if err != nil {
		return "", nil, err
	}

	idx := slices.IndexFunc(snapshots, func(inst api.Instance) bool {
		return inst.Config[workshop.ConfigWorkshopSdk] == sk
	})
	if idx < 0 {
		return "", nil, fmt.Errorf("%q SDK snapshot not found in %q workshop", sk, w)
	}
	devices := snapshots[idx].Devices

	// Any SDK which wasn't installed must have been added later.
	obstacles := make([]string, 0, len(snapshots))
	for _, snapshot := range snapshots {
		device := workshop.SdkDeviceName(snapshot.Config[workshop.ConfigWorkshopSdk])
		if _, ok := devices[device]; !ok {
			obstacles = append(obstacles, snapshot.Name)
		}
	}

	return snapshots[idx].Name, obstacles, nil
}

// Copies an instance from the src server to the dst server. The servers can be
// identical, different, or the same underlying server for different projects.
// If the target instance already exists, set refresh=true to overwrite it.
// Otherwise, refresh should be false to avoid unnecessary API calls. In both
// cases the non-volatile config options, and a couple of volatile ones, are
// copied to the target instance. Additional options can be added or overridden
// using the config argument. To prevent an option from being copied over, add
// it to the exclude list. The boot.autostart option is always set to false in
// the copy: the source instance might be running but the copy won't be.
func (s *Backend) copyInstance(src, dst lxd.InstanceServer, srcName, dstName string, refresh bool, config map[string]string) error {
	if config == nil {
		config = map[string]string{}
	}
	config["boot.autostart"] = "false"

	srcInst, _, err := src.GetInstance(srcName)
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return workshop.ErrWorkshopNotLaunched
		}
		return err
	}

	// Set the config and device overrides. LXD will copy most other
	// options from the source instance, but it will omit options which are
	// unique to the source instance, like MAC addresses and UUIDs.
	req := *srcInst
	req.Config = config
	req.Devices = nil

	args := lxd.InstanceCopyArgs{
		Name:         dstName,
		InstanceOnly: true,
		Refresh:      refresh,
	}

	rop, err := dst.CopyInstance(src, req, &args)
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

	// Otherwise, LXD replaced an existing instance's root disk, and it's
	// our job to copy the config and devices.
	dstInst, etag, err := dst.GetInstance(dstName)
	if err != nil {
		return err
	}

	// Remove all options that aren't unique to the target. The ones we do
	// want to set are added below. Ideally, we should also remove some
	// unique ones. See https://github.com/canonical/lxd/issues/16667.
	maps.DeleteFunc(dstInst.Config, func(k, v string) bool { return optionDomain(k) != uniqueProperty })

	// Add all options from the source, except ones unique to the target.
	maps.DeleteFunc(srcInst.Config, func(k, v string) bool { return optionDomain(k) == uniqueProperty })
	if dstInst.Config == nil {
		dstInst.Config = srcInst.Config
	} else {
		maps.Copy(dstInst.Config, srcInst.Config)
	}

	// Process config overrides.
	if dstInst.Config == nil {
		dstInst.Config = map[string]string{}
	}
	for k, v := range config {
		if v == "" {
			delete(dstInst.Config, k)
		} else {
			dstInst.Config[k] = v
		}
	}

	dstInst.Devices = srcInst.Devices

	op, err := dst.UpdateInstance(dstName, dstInst.Writable(), etag)
	if err != nil {
		return err
	}
	return op.Wait()
}

func (s *Backend) RemoveWorkshopStash(ctx context.Context, name string) error {
	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	_, conn, err := s.snapshotClients(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	snapshots, err := s.snapshotNames(conn, projectId, name, "stash-sdk")
	if err == nil {
		for _, snapshot := range snapshots {
			err1 := s.deleteSnapshot(conn, snapshot)
			err = cmp.Or(err, err1)
		}
	}

	err1 := s.deleteSnapshot(conn, instanceStashName(name, projectId))
	return cmp.Or(err, err1)
}

func (s *Backend) deleteSnapshot(snapshotConn lxd.InstanceServer, snapshot string) error {
	op, err := snapshotConn.DeleteInstance(snapshot)
	if err == nil {
		err = op.Wait()
	}
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		return nil
	}
	return err
}

func (s *Backend) snapshotClients(ctx context.Context) (lxd.InstanceServer, lxd.InstanceServer, error) {
	user, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return nil, nil, fmt.Errorf("context key %s not found", workshop.ContextUser)
	}

	project, err := lxdSnapshotsProjectName(user)
	if err != nil {
		return nil, nil, err
	}

	conn, err := s.LxdClient(ctx)
	if err != nil {
		return nil, nil, err
	}

	return conn, conn.UseProject(project), nil
}
