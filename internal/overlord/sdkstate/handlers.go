package sdkstate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/tomb.v2"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/interfaces/policy"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	. "github.com/canonical/workshop/internal/overlord/handlersetup"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/progress"
	"github.com/canonical/workshop/internal/revert"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/sdk/system"
	"github.com/canonical/workshop/internal/workshop"
)

func SdkSetup(task *state.Task) (sdk.Setup, error) {
	st := task.State()
	st.Lock()
	defer st.Unlock()

	var retrieveId string
	var sdkSetup sdk.Setup

	err := task.Get("sdk-retrieve-task", &retrieveId)

	if err != nil {
		return sdk.Setup{}, err
	}

	retrieve := task.State().Task(retrieveId)
	if retrieve == nil {
		return sdk.Setup{}, fmt.Errorf("internal error: no corresponding retrieve-sdk task found")
	}

	if err = retrieve.Get("sdk-setup", &sdkSetup); err != nil {
		return sdk.Setup{}, err
	}
	return sdkSetup, nil
}

func (m *SdkManager) doRetrieveSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, _, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	st := task.State()
	var rec sdk.Setup

	st.Lock()
	err = task.Get("sdk-setup", &rec)
	st.Unlock()
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	reporter := &progress.Reporter{
		Name: task.ID(),
		Report: func(label string, done, total int) {
			st.Lock()
			task.SetProgress(label, done, total)
			st.Unlock()
		},
	}

	if rec.Source == sdk.SystemSource {
		if err := system.RetrieveSystemSdk(rec, reporter); err != nil {
			return err
		}
	} else {
		st.Lock()
		store := sdk.StoreService(st)
		st.Unlock()

		if err = store.DownloadSdk(ctx, rec, reporter); err != nil {
			return err
		}
	}

	volume := workshop.VolumeInfo{
		Name:     sdk.VolumeName(rec.Name, rec.Revision),
		Kind:     "sdk",
		Sdk:      rec.Name,
		Revision: rec.Revision,
	}
	file, err := os.Open(rec.Filepath())
	if err != nil {
		return err
	}
	defer file.Close()
	err = m.backend.ImportVolume(ctx, volume, file)
	if errors.Is(err, workshop.ErrVolumeAlreadyExists) {
		logger.Debugf("SDK Manager on maybeCreateVolume: reuse existing SDK volume %q", sdk.VolumeName(rec.Name, rec.Revision))
		return nil
	}

	return err
}

func (m *SdkManager) doInstallSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	sdkSetup, err := SdkSetup(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	// Directory: /var/lib/workshop/sdk/<name>/
	fs, err := m.backend.WorkshopFs(ctx, w)
	if err != nil {
		return err
	}
	defer fs.Close()
	if err = fs.MkdirAll(dirs.WorkshopSdksDir, 0755); err != nil {
		return err
	}

	wp, err := m.backend.Workshop(ctx, w)
	if err != nil {
		return err
	}

	rev := revert.New()
	defer rev.Fail()

	if err := m.mountSdk(ctx, user, project, w, sdkSetup); err != nil {
		return err
	}
	st := task.State()
	rev.Add(func() {
		if reverr := m.unmountSdk(ctx, w, sdkSetup); reverr != nil {
			st.Lock()
			task.Logf("Install SDK cleanup: could not unmount %q SDK: %v", sdkSetup.Name, reverr)
			st.Unlock()
		}
	})

	if err = wp.AddSdk(ctx, sdkSetup); err != nil {
		return err
	}

	rev.Success()
	return nil
}

func (m *SdkManager) mountSdk(ctx context.Context, user string, project *workshop.Project, w string, sdkSetup sdk.Setup) error {
	// Mount the SDK content at the workshop location.
	name := sdk.VolumeName(sdkSetup.Name, sdkSetup.Revision)
	sdkPath := sdk.SdkDir(sdkSetup.Name)

	if sdkSetup.IsVolume() {
		return m.backend.AttachVolume(ctx, w, name, sdkPath, true)
	}

	usr, env, err := osutil.UserAndEnv(user)
	if err != nil {
		return err
	}
	userDataDir := workshop.UserDataRootDir(usr.HomeDir, env)
	what := workshop.LocalSdkRevision(userDataDir, project.ProjectId, w, sdkSetup.Name, sdkSetup.Revision)

	mnt := workshop.Mount{Name: name, What: what, Where: sdkPath, MakeWhere: true, Type: workshop.HostWorkshop, ReadOnly: true}
	return m.backend.AddWorkshopMount(ctx, w, mnt)
}

func (m *SdkManager) doUninstallSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	sdkSetup, err := SdkSetup(task)
	if err != nil {
		return err
	}

	wp, err := m.backend.Workshop(ctx, w)
	if err != nil {
		return err
	}

	if err := wp.RemoveSdk(ctx, sdkSetup.Name); err != nil {
		return err
	}

	return m.unmountSdk(ctx, w, sdkSetup)
}

func (m *SdkManager) unmountSdk(ctx context.Context, w string, sdkSetup sdk.Setup) error {
	name := sdk.VolumeName(sdkSetup.Name, sdkSetup.Revision)
	if sdkSetup.IsVolume() {
		return m.backend.DetachVolume(ctx, w, name)
	}
	return m.backend.RemoveWorkshopMount(ctx, w, name)
}

func (m *SdkManager) doRegisterSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	setup, err := SdkSetup(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	wp, err := m.backend.Workshop(ctx, w)
	if err != nil {
		return err
	}

	info, err := wp.SdkInfo(ctx, setup.Name)
	if err != nil {
		return err
	}

	if len(info.BadInterfaces) > 0 {
		return fmt.Errorf("%s", sdk.BadInterfacesSummary(info))
	}

	if err = policy.CheckInterfaces(info); err != nil {
		return err
	}

	// add SDK's plugs and slots
	return m.repo.AddSdk(info)
}

func (m *SdkManager) doUnregisterSdk(task *state.Task, tomb *tomb.Tomb) error {
	_, project, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	setup, err := SdkSetup(task)
	if err != nil {
		return err
	}

	return m.repo.RemoveSdk(project.ProjectId, w, setup.Name)
}

type SdkVolumeCooldownTimeKey string

func sameVolume(task *state.Task, other *state.Task) bool {
	var sdkSetup sdk.Setup
	if err := task.Get("sdk-setup", &sdkSetup); err != nil {
		return false
	}

	var otherSdkSetup sdk.Setup
	if err := other.Get("sdk-setup", &otherSdkSetup); err != nil {
		return false
	}

	if !sdkSetup.IsVolume() || !otherSdkSetup.IsVolume() {
		return false
	}

	return sdkSetup.Name == otherSdkSetup.Name && sdkSetup.Revision == otherSdkSetup.Revision
}

func (m *SdkManager) doDeleteUnusedSdkVolumes(task *state.Task, tomb *tomb.Tomb) error {
	st := task.State()

	user, prj, _, err := UserProjectWorkshop(task)
	if err != nil {
		// Log as an internal error, no need to retry again.
		logger.Debugf("On SdkManager.Cleanup: internal error: %v", err)
		return nil
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	st.Lock()
	defer st.Unlock()

	if task.Kind() == "install-sdk" && task.Status() == state.DoneStatus {
		// If the launch or refresh was successful, no need to clean up the SDK volume.
		return nil
	}

	var sdkSetup sdk.Setup
	if err := task.Get("sdk-setup", &sdkSetup); err != nil {
		logger.Debugf("On SdkManager.Cleanup: the %q task is not associated with a SDK setup", task.ID())
		return nil
	}

	if !sdkSetup.IsVolume() || sdk.IsSystem(sdkSetup.Name) {
		return nil
	}

	vk := SdkVolumeCooldownTimeKey(sdk.VolumeName(sdkSetup.Name, sdkSetup.Revision))
	cooldownStart, ok := st.Cached(vk).(time.Time)
	if !ok {
		// No cooldown start time cached means that clean up has just started or
		// workshopd was restarted.
		st.Cache(vk, task.ReadyTime())
		return &state.Retry{}
	} else {
		// Imagine the situation when a change with multiple tasks that use this
		// volume is in progress. Every unregister-sdk task will initiate a
		// cleanup after the change is ready. All cleanups are unarranged and
		// can run concurrently. We need to ensure that only one of them will
		// continue to track the volume cooldown time.
		readyTime := task.ReadyTime()
		if !readyTime.Equal(cooldownStart) {
			if readyTime.Before(cooldownStart) {
				// Another more recent task initiated cleanup for this SDK
				// volume.
				return nil
			} else {
				// Update the cooldown start time to the current task ready time.
				st.Cache(vk, readyTime)
				return &state.Retry{}
			}
		}
	}

	// Check if there are any tasks in progress that use the same SDK volume.
	// Remember, cleanup tasks are attempted on every Ensure call, which can be
	// either periodic or triggered by a new API request. If it's the latter,
	// there could be a chance that the volume will be used again.
	changes := st.Changes()
	for _, change := range changes {
		// Other changes like "exec" do not affect SDK volumes, so we can skip them.
		if !change.IsReady() {
			if change.Kind() == "launch" {
				for _, t := range change.Tasks() {
					if t.Kind() == "install-sdk" {
						if sameVolume(task, t) {
							// Launch may end up using the same SDK volume if successful,
							// so skip the cleanup for now.
							return &state.Retry{}
						}
					}
				}
			}
			if change.Kind() == "refresh" || change.Kind() == "remove" {
				for _, t := range change.Tasks() {
					if t.Kind() == "unregister-sdk" {
						if sameVolume(task, t) {
							// Another task in the same change is using the same SDK volume.
							// We can skip the cleanup for now.
							logger.Debugf("On SdkManager.Cleanup: another task %q in the %q change is using the %q SDK volume, skip clean up",
								t.ID(), change.ID(), sdk.VolumeName(sdkSetup.Name, sdkSetup.Revision))
							st.Cache(vk, nil)
							return nil
						}
					}
				}
			}
		}
	}

	if time.Since(cooldownStart) < sdkVolumeCooldownTime {
		return &state.Retry{}
	}

	// Delete volume ignores ErrVolumeNotFound.
	err = m.backend.DeleteVolume(ctx, sdk.VolumeName(sdkSetup.Name, sdkSetup.Revision))
	if err == nil || errors.Is(err, workshop.ErrVolumeInUse) {
		if errors.Is(err, workshop.ErrVolumeInUse) {
			logger.Debugf("On SdkManager.Cleanup: the %q volume is still in use, skip clean up", sdk.VolumeName(sdkSetup.Name, sdkSetup.Revision))
		}
		st.Cache(vk, nil)
		return nil
	}

	return err
}
