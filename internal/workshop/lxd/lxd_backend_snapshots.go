package lxdbackend

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/sha3"
	"encoding/hex"
	"errors"
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
	switch {
	case strings.HasPrefix(key, "image."):
		return sharedProperty
	case !strings.HasPrefix(key, "volatile."):
		return customOption
	case slices.Contains(api.InstanceRemoteCopyConfigKeyPolicy.Immutable, key):
		return sharedProperty
	default:
		return uniqueProperty
	}
}

func (s *Backend) TakeSnapshot(ctx context.Context, name string, snapshot workshop.Snapshot) error {
	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	if snapshot.IsBase() {
		return errors.New("internal error: attempted snapshot of base image")
	}
	sk := snapshot.Sdks[len(snapshot.Sdks)-1].Name

	digest, err := s.HashSnapshot(snapshot)
	if err != nil {
		return fmt.Errorf("internal error: hashing snapshot info: %w", err)
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

	newApi := snapshotConn.HasExtension("instance_refresh_config")

	if inst.Config == nil {
		inst.Config = map[string]string{}
	}
	config := map[string]string{
		"security.protection.start":            "true",
		workshop.ConfigProjectId:               projectId,
		workshop.ConfigWorkshopName:            name,
		workshop.ConfigWorkshopBase:            snapshot.Image.Name,
		workshop.ConfigWorkshopBaseFingerprint: snapshot.Image.Fingerprint,
		workshop.ConfigWorkshopSnapshotType:    "sdk",
		workshop.ConfigWorkshopSnapshotFormat:  SnapshotFormatRevision.String(),
		workshop.ConfigWorkshopSha3_384:        digest,
		workshop.ConfigWorkshopSdk:             sk,
	}
	mergeConfig(inst.Config, nil, config, newApi)

	if inst.Devices == nil {
		inst.Devices = map[string]map[string]string{}
	}
	if err := mergeDevices(inst.Devices, snapshot.Sdks, name, newApi); err != nil {
		return err
	}

	// Reset everything that isn't explicitly specified.
	inst.SetWritable(api.InstancePut{
		Architecture: inst.Architecture,
		Config:       inst.Config,
		Devices:      inst.Devices,
		Profiles:     []string{},
	})

	snapshotName, err := sdkSnapshotName(sk)
	if err != nil {
		return err
	}
	args := lxd.InstanceCopyArgs{Name: snapshotName, InstanceOnly: true}
	rop, err := snapshotConn.CopyInstance(conn, *inst, &args)
	if err != nil {
		return err
	}
	return rop.Wait()
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

	inst, _, err := conn.GetInstance(req.Name)
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		inst = &api.Instance{}
	} else if err != nil {
		return err
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

	newApi := conn.HasExtension("instance_refresh_config")

	if snapshot.Config == nil {
		snapshot.Config = map[string]string{}
	}
	var config map[string]string
	if !newApi {
		// The old API handles config options inconsistently. This
		// computes the form required for UpdateInstance.
		config = maps.Clone(snapshot.Config)
		mergeConfig(config, inst.Config, req.Config, true)
	}
	mergeConfig(snapshot.Config, inst.Config, req.Config, newApi)

	req.Architecture = snapshot.Architecture
	req.Config = snapshot.Config
	// Ensure LXD doesn't copy profiles from the snapshot.
	if req.Profiles == nil {
		req.Profiles = []string{}
	}
	snapshot.SetWritable(req.InstancePut)

	args := lxd.InstanceCopyArgs{
		Name:         req.Name,
		InstanceOnly: true,
		Refresh:      true,
	}
	rop, err := conn.CopyInstance(snapshotConn, *snapshot, &args)
	if err != nil {
		return err
	}
	if err := rop.Wait(); err != nil {
		return err
	}

	if newApi {
		return nil
	}

	// If the workshop already existed, it still has the old config options
	// and devices. If it did not exist, the config and devices are mostly
	// correct, but we still need to remove placeholder SDK devices.
	req.Config = config
	op, err := conn.UpdateInstance(req.Name, req.InstancePut, "")
	if err != nil {
		return err
	}
	return op.Wait()
}

func mergeConfig(source, target, config map[string]string, newApi bool) {
	if newApi {
		maps.DeleteFunc(source, func(k, v string) bool {
			return optionDomain(k) != sharedProperty
		})
	} else {
		// Omit options managed by LXD, to let it decide what to do.
		maps.DeleteFunc(source, func(k, v string) bool {
			return optionDomain(k) != customOption
		})
		// Tell LXD to remove all custom options.
		for k := range source {
			source[k] = ""
		}
	}

	for k, v := range target {
		if optionDomain(k) == uniqueProperty {
			source[k] = v
		}
	}

	maps.Copy(source, config)
}

func mergeDevices(source map[string]map[string]string, sdks []sdk.Id, w string, newApi bool) error {
	if newApi {
		maps.DeleteFunc(source, func(k string, v map[string]string) bool {
			return k != "root"
		})
	} else {
		none := map[string]string{"type": "none"}
		for key, device := range source {
			s, err := maybeSdkInstallation(key, device)
			if err != nil {
				return err
			}
			if s != nil {
				idx := s.InstallOrder - 1
				if idx < 0 || len(sdks) <= idx || sdks[idx].Name != s.Name {
					return fmt.Errorf("internal error: %q workshop has unexpected %q SDK", w, s.Name)
				}
				delete(source, key)
			} else if key != "root" {
				source[key] = none
			}
		}
	}

	for i, sk := range sdks {
		source[workshop.SdkDeviceName(sk.Name)] = sdkToSnapshotDevice(i+1, sk)
	}

	return nil
}

func (s *Backend) StashWorkshop(ctx context.Context, name string) error {
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

	rev := revert.New()
	defer rev.Fail()

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
	return s.copyInstance(snapshotConn, conn, stash, instance, true, config)
}

// Find snapshot names for all SDKs in the given workshop or stash.
func (s *Backend) snapshotNames(snapshotConn lxd.InstanceServer, pid, w string, kind string) ([]string, error) {
	args := lxd.GetInstancesArgs{
		InstanceType: api.InstanceTypeContainer,
		Filters: []string{
			"config.user.workshop.project-id=" + pid,
			"config.user.workshop.name=" + w,
			"config.user.workshop.snapshot-type=" + kind,
		},
	}
	snapshots, err := snapshotConn.GetInstances(args)
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
	args := lxd.GetInstancesArgs{
		InstanceType: api.InstanceTypeContainer,
		Filters: []string{
			"config.user.workshop.project-id=" + pid,
			"config.user.workshop.name=" + w,
			"config.user.workshop.snapshot-type=sdk",
		},
	}
	snapshots, err := snapshotConn.GetInstances(args)
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

	dstInst, _, err := dst.GetInstance(dstName)
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		dstInst = &api.Instance{}
	} else if err != nil {
		return err
	}

	if srcInst.Config == nil {
		srcInst.Config = map[string]string{}
	}
	maps.DeleteFunc(srcInst.Config, func(k, v string) bool {
		return optionDomain(k) == uniqueProperty
	})
	for k, v := range dstInst.Config {
		if optionDomain(k) == uniqueProperty {
			srcInst.Config[k] = v
		}
	}
	for k, v := range config {
		if v == "" {
			delete(srcInst.Config, k)
		} else {
			srcInst.Config[k] = v
		}
	}

	newApi := dst.HasExtension("instance_refresh_config")

	req := *srcInst
	if !newApi {
		// Set the config and device overrides. LXD will copy most other
		// options from the source instance, but it will omit options which are
		// unique to the source instance, like MAC addresses and UUIDs.
		req.Config = config
		req.Devices = nil
	}

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

	if newApi || dstInst.Name == "" {
		// LXD created a new instance with copied config and devices.
		return nil
	}

	// Otherwise, LXD replaced an existing instance's root disk, and it's
	// our job to copy the config and devices.
	req.Config = srcInst.Config
	req.Devices = srcInst.Devices

	op, err := dst.UpdateInstance(dstName, req.Writable(), "")
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
	op, err := snapshotConn.DeleteInstance(snapshot, false)
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

// Hard coded number which represents the current "snapshot format," i.e. the
// contents of the rootfs after installing a certain sequence of SDKs. This can
// be influenced by a number of factors, including:
// - Default workshop config (e.g. cloud-config)
// - Default workshop devices (e.g. apt cache)
// - SDK config and devices (e.g. volume mounts)
// - Direct modifications (e.g. mkdir /var/lib/workshop/run)
// If something like this changes, bump the revision number so that workshops
// constructed using the next release of Workshop see the changes.
var SnapshotFormatRevision = sdk.R(1)

func (s *Backend) HashSnapshot(snapshot workshop.Snapshot) (string, error) {
	digest, err := hex.DecodeString(snapshot.Image.Fingerprint)
	if err != nil {
		return "", err
	}

	hash := sha3.New384()
	if _, err := fmt.Fprintf(hash, "%s %s\x00%s", SnapshotFormatRevision, snapshot.Image.Name, digest); err != nil {
		return "", err
	}

	for _, sk := range snapshot.Sdks {
		// Different types of SDKs result in different workshops. There
		// are really only two types: those which use SDK volumes and
		// those which use local directories.
		kind := "local"
		if sk.IsVolume {
			kind = "volume"
		}

		digest, err := hex.DecodeString(sk.Sha3_384)
		if err != nil {
			return "", err
		}

		if _, err := fmt.Fprintf(hash, "%s %s\x00%s", kind, sk.Name, digest); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
