package sdkstate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"gopkg.in/tomb.v2"

	"github.com/canonical/workshop/internal/interfaces/policy"
	"github.com/canonical/workshop/internal/logger"
	. "github.com/canonical/workshop/internal/overlord/handlersetup"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/progress"
	"github.com/canonical/workshop/internal/revert"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/sdk/system"
	"github.com/canonical/workshop/internal/workshop"
)

func SdkSetup(task *state.Task, w string) (sdk.Setup, error) {
	st := task.State()
	st.Lock()
	defer st.Unlock()

	// Some tasks, notably uninstall-sdk, store the sdk.Setup directly,
	// since it isn't stored anywhere else. Note that the uninstall-sdk
	// handler only needs the SDK name, but the cleanup handler needs more.
	var setup sdk.Setup
	if err := task.Get("sdk-setup", &setup); err == nil {
		return setup, nil
	}

	// Tasks like install-sdk can reuse the sdk.Setup
	// from the launch or refresh Change.
	var sk string
	if err := task.Get("sdk", &sk); err != nil {
		return sdk.Setup{}, err
	}

	setups, err := WorkshopSdks(task.Change(), w)
	if err != nil {
		return sdk.Setup{}, err
	}

	idx := slices.IndexFunc(setups, func(s sdk.Setup) bool {
		return s.Name == sk
	})
	if idx < 0 {
		return sdk.Setup{}, fmt.Errorf("internal error: %q workshop has no %q SDK", w, sk)
	}
	return setups[idx], nil
}

func sdkName(task *state.Task) (string, error) {
	st := task.State()
	st.Lock()
	defer st.Unlock()

	var sk string
	if err := task.Get("sdk", &sk); err != nil {
		return "", err
	}
	return sk, nil
}

// replaceSdkSetup updates a launch or refresh change with the latest SDK
// revision, if it changed between the planning phase and retrieval. TODO:
// remove this once the Store supports downloading a specific revision.
func replaceSdkSetup(change *state.Change, w string, setup sdk.Setup) {
	setups, err := WorkshopSdks(change, w)
	if err != nil {
		// Nothing to replace.
		return
	}

	for i, s := range setups {
		if s.Name == setup.Name {
			setups[i] = setup
			change.Set(WorkshopSdksKey(w), setups)
			break
		}
	}
}

func (m *SdkManager) doRetrieveSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	rec, err := SdkSetup(task, w)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	st := task.State()
	reporter := &progress.Reporter{
		Name: task.ID(),
		Report: func(label string, done, total int) {
			st.Lock()
			task.SetProgress(label, done, total)
			st.Unlock()
		},
	}

	var meta *sdk.Meta
	if rec.Source == sdk.SystemSource {
		meta, err = system.RetrieveSystemSdk(rec, reporter)
	} else {
		st.Lock()
		store := sdk.GcsStoreService(st)
		st.Unlock()

		meta, err = store.DownloadSdk(ctx, rec, reporter)
	}
	if err != nil {
		return err
	}

	// TODO: We should be downloading a specific revision. Remove this when
	// the Store supports that (it will probably have to be removed anyway
	// since DownloadSdk won't return metadata after that).
	if meta.Revision != rec.Revision {
		st.Lock()
		change := task.Change()
		replaceSdkSetup(change, w, meta.Setup)
		base, err := WorkshopBase(change, w)
		st.Unlock()
		if err != nil {
			return err
		}

		if err := workshop.ValidateSdkInfo(project.ProjectId, w, base.Name, meta.Name, meta.SdkYAML); err != nil {
			return err
		}
	}

	file, err := os.Open(meta.Filepath())
	if err != nil {
		return err
	}
	defer file.Close()
	err = m.backend.ImportSdk(ctx, *meta, file)
	if errors.Is(err, workshop.ErrVolumeAlreadyExists) {
		logger.Debugf("SDK Manager on maybeCreateVolume: reuse existing SDK volume %q", sdk.VolumeName(meta.Name, meta.Revision))
		return nil
	}

	return err
}

func (m *SdkManager) doInstallSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	sdkSetup, err := SdkSetup(task, w)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	rev := revert.New()
	defer rev.Fail()

	if err := m.backend.InstallSdk(ctx, w, sdkSetup); err != nil {
		return err
	}
	cleanupCtx := context.WithoutCancel(ctx)
	rev.Add(func() {
		cleanupCtx, cancel := context.WithTimeout(cleanupCtx, 30*time.Second)
		defer cancel()

		if reverr := m.backend.UninstallSdk(cleanupCtx, w, sdkSetup.Name); reverr != nil {
			logger.Noticef("On doInstallSdk: cannot uninstall %q SDK on cleanup: %v", sdkSetup.Name, reverr)
		}
	})

	// add SDK's plugs and slots
	if err := m.registerSdk(ctx, w, sdkSetup.Name); err != nil {
		return err
	}

	rev.Success()
	return nil
}

func (m *SdkManager) doUninstallSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	sk, err := sdkName(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	rev := revert.New()
	defer rev.Fail()

	if err := m.repo.RemoveSdk(project.ProjectId, w, sk); err != nil {
		return err
	}
	cleanupCtx := context.WithoutCancel(ctx)
	rev.Add(func() {
		cleanupCtx, cancel := context.WithTimeout(cleanupCtx, 30*time.Second)
		defer cancel()

		if reverr := m.registerSdk(cleanupCtx, w, sk); reverr != nil {
			logger.Noticef("On doUninstallSdk: cannot re-register %q SDK on cleanup: %v", sk, reverr)
		}
	})

	if err := m.backend.UninstallSdk(ctx, w, sk); err != nil {
		return err
	}

	rev.Success()
	return nil
}

func (m *SdkManager) registerSdk(ctx context.Context, w, sk string) error {
	wp, err := m.backend.Workshop(ctx, w)
	if err != nil {
		return err
	}

	info, err := wp.SdkInfo(ctx, sk)
	if err != nil {
		return err
	}

	if len(info.BadInterfaces) > 0 {
		return fmt.Errorf("%s", sdk.BadInterfacesSummary(info))
	}

	if err = policy.CheckInterfaces(info); err != nil {
		return err
	}

	return m.repo.AddSdk(info)
}

func (m *SdkManager) doSnapshotSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	sk, err := sdkName(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	snapshot, err := WorkshopSnapshot(task.Change(), w, sk)
	st.Unlock()
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	return m.backend.TakeSnapshot(ctx, w, *snapshot)
}

type SdkVolumeCooldownTimeKey string

// doDeleteUnusedSdkVolumes is a cleanup handler that deletes unused SDK volumes.
// It is called periodically to ensure that SDK volumes that are no longer
// needed are removed from the system. The cleanup is done only if there are no
// tasks in progress that use the same SDK volume. The cleanup is also
// throttled to avoid deleting volumes too frequently. The cooldown time is
// defined by the sdkVolumeCooldownTime constant.
//
// There are multiple scenarios when this cleanup is triggered:
//   - A workshop was removed and an SDK volume was detached from it (uninstall-sdk task)
//   - An SDK volume was detached during a refresh change,
//     and is no longer used by any other workshop (uninstall-sdk or, if refresh failed, install-sdk task)
//   - A workshop launch failed (install-sdk task)
func (m *SdkManager) doDeleteUnusedSdkVolumes(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, w, err := UserProjectWorkshop(task)
	if err != nil {
		// Log as an internal error, no need to retry again.
		logger.Debugf("On SdkManager.Cleanup: internal error: %v", err)
		return nil
	}

	sdkSetup, err := SdkSetup(task, w)
	if err != nil {
		logger.Debugf("On SdkManager.Cleanup: the %q task is not associated with a SDK setup", task.ID())
		return nil
	}

	if !sdkSetup.IsVolume() || sdkSetup.Source == sdk.SystemSource {
		return nil
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	st := task.State()
	st.Lock()
	defer st.Unlock()

	if task.Kind() == "install-sdk" && task.Status() == state.DoneStatus {
		// If the launch or refresh was successful, no need to clean up the SDK volume.
		return nil
	}

	vk := SdkVolumeCooldownTimeKey(sdk.VolumeName(sdkSetup.Name, sdkSetup.Revision))
	cooldownStart, ok := st.Cached(vk).(time.Time)
	if !ok {
		// No cooldown start time cached means that clean up has just started or
		// workshopd was restarted.
		st.Cache(vk, task.ReadyTime())
		// We need to return here to give other competing cleanup tasks a chance
		// to find out who was the last one to initiate the cleanup.
		return fmt.Errorf("new cooldown start time for %q SDK volume: %v", sdk.VolumeName(sdkSetup.Name, sdkSetup.Revision), task.ReadyTime())
	} else {
		// Imagine the situation when a change with multiple tasks that use this
		// volume is in progress. Every uninstall-sdk task will initiate a
		// cleanup after the change is ready. All cleanups are unarranged and
		// can run concurrently. We need to ensure that only one of them will
		// continue to track the volume cooldown time.
		readyTime := task.ReadyTime()
		if readyTime.Before(cooldownStart) {
			// Another more recent task initiated cleanup for this SDK
			// volume, no need to continue.
			return nil
		}
		if readyTime.After(cooldownStart) {
			// Update the cooldown start time to the current task ready time.
			st.Cache(vk, readyTime)
			// We need to return here to give other competing cleanup tasks
			// a chance to find out who was the last one to initiate the
			// cleanup.
			return fmt.Errorf("new cooldown start time for %q SDK volume: %v", sdk.VolumeName(sdkSetup.Name, sdkSetup.Revision), readyTime)
		}
	}

	if time.Since(cooldownStart) < sdkVolumeCooldownTime {
		remaining := sdkVolumeCooldownTime - time.Since(cooldownStart)
		return fmt.Errorf("cooldown period for %q SDK volume has not elapsed yet, time remaining: %s",
			sdk.VolumeName(sdkSetup.Name, sdkSetup.Revision), remaining.Round(time.Second))
	}

	// Check if there are any tasks in progress that use the same SDK volume.
	// Remember, cleanup tasks are attempted on every Ensure call, which can be
	// either periodic or triggered by a new API request. If it's the latter,
	// there could be a chance that the volume will be used again.
	changes := st.Changes()
	blocking := []string{"launch", "refresh", "remove"}
	for _, change := range changes {
		// Other changes like "exec" do not affect SDK volumes, so we can skip them.
		if slices.Contains(blocking, change.Kind()) && !change.IsReady() {
			return &state.Retry{}
		}
	}

	// Delete volume ignores ErrVolumeNotFound.
	err = m.backend.DeleteSdk(ctx, sdkSetup)
	if err == nil || errors.Is(err, workshop.ErrVolumeInUse) {
		if errors.Is(err, workshop.ErrVolumeInUse) {
			logger.Debugf("On SdkManager.Cleanup: the %q SDK volume is still in use, skip clean up", sdk.VolumeName(sdkSetup.Name, sdkSetup.Revision))
		} else {
			logger.Debugf("On SdkManager.Cleanup: the %q SDK volume was deleted", sdk.VolumeName(sdkSetup.Name, sdkSetup.Revision))
		}
		st.Cache(vk, nil)
		return nil
	}

	return err
}
