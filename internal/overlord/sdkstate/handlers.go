package sdkstate

import (
	"context"
	"errors"
	"fmt"

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
		Name: sdk.VolumeName(rec.Name, rec.Revision),
		Kind: "sdk",
		Sdk:  rec.Name,
	}
	err = m.backend.ImportVolume(ctx, volume, rec.Filepath())
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
