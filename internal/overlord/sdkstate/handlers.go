package sdkstate

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/afero"
	"gopkg.in/tomb.v2"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/interfaces/policy"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	. "github.com/canonical/workshop/internal/overlord/handlersetup"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/revert"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
	lxdbackend "github.com/canonical/workshop/internal/workshop/lxd"
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

	ctx, _ := BackendContext(tomb, user, project.ProjectId)
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	st.Lock()
	store := sdk.StoreService(st)
	st.Unlock()

	return store.DownloadSdk(ctx, rec)
}

func (m *SdkManager) doInstallAgentSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	wp, err := m.backend.Workshop(ctx, w)
	if err != nil {
		return err
	}

	return wp.InstallAgentSdk(ctx)
}

func (m *SdkManager) undoInstallAgentSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, w, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	wfs, err := m.backend.WorkshopFs(ctx, w)
	if err != nil {
		return err
	}
	defer wfs.Close()

	return wfs.RemoveAll(sdk.SdkRootPath("agent"))
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

	// The install tasks should hold the lock until the SDK is unpacked in the
	// workshop. There are could be multiple of them reading the file
	// concurrently and, hence, TryLock, so a writer (e.g. DownloadSdk) would
	// not corrupt the file before it is installed.
	fl, err := sdk.OpenLock(sdkSetup.Name)
	if err != nil {
		return err
	}
	if err = fl.TryLock(); err != nil && !errors.Is(err, osutil.ErrAlreadyLocked) {
		return err
	}
	defer fl.Close()

	target := filepath.Join("/root", filepath.Base(sdkSetup.Filename()))
	sdkMount := lxdbackend.Mount(sdkSetup.Name, sdkSetup.Filename(), target)
	if err = m.backend.AddWorkshopDevice(ctx, w, sdkMount); err != nil {
		return err
	}

	cleanup := func() {
		// Make sure the SDK file will be unmounted once installed into the workshop
		if err := m.backend.RemoveWorkshopDevice(ctx, w, sdkMount.Name); err != nil {
			logger.Debugf("cannot unmount SDK %q from workshop %q: %v", sdkMount.Name, w, err)
		}
	}

	defer cleanup()

	// example: /var/lib/workshop/sdk/cuda/712/
	sdkPath := filepath.Join(dirs.WorkshopSdksDir, sdkSetup.Name,
		strconv.Itoa(int(sdkSetup.Revision)))

	// create a memory out/err to log the hook output into the task's log
	memFs := afero.NewMemMapFs()
	out, err := memFs.Create(fmt.Sprintf("%s-%s", w, project.ProjectId))
	if err != nil {
		return err
	}

	// Unpack the SDK to the desired location in the workshop
	//   Note: the following command requires ~ tar >= 1.29 due to --one-top-level
	args := workshop.Execution{
		ExecArgs: workshop.ExecArgs{
			UserId:  0,
			GroupId: 0,
			Command: []string{
				"tar",
				"--extract",
				"--file",
				target,
				"--one-top-level=" + sdkPath,
				"--no-same-owner",
			},
			WorkDir: "/",
		},
		ExecControls: workshop.ExecControls{
			Stdin:  nil,
			Stdout: nil,
			Stderr: out,
		},
	}

	exectx, err := m.backend.Exec(ctx, w, &args)
	if err != nil {
		return err
	}

	if err = exectx.WaitExecution(ctx); err != nil {
		hookLog, _ := afero.ReadFile(memFs, out.Name())
		return fmt.Errorf("%w: %v", err, string(hookLog))
	}

	return err
}

func (m *SdkManager) undoInstallSdk(task *state.Task, tomb *tomb.Tomb) error {
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

	fs, err := m.backend.WorkshopFs(ctx, w)
	if err != nil {
		return err
	}
	defer fs.Close()

	err = fs.RemoveAll(filepath.Join(dirs.WorkshopSdksDir, sdkSetup.Name))
	if err != nil {
		return fmt.Errorf("cannot undo SDK %q installation: %w", sdkSetup.Name, err)
	}

	return nil
}

func (m *SdkManager) doLinkSdk(task *state.Task, tomb *tomb.Tomb) error {
	rev := revert.New()
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

	if err = wp.LinkSdk(ctx, setup); err != nil {
		return err
	}

	st := task.State()
	rev.Add(func() {
		if err := wp.UnlinkSdk(ctx, setup.Name); err != nil {
			st.Lock()
			task.Logf("Link SDK cleanup: could not unlink %q SDK: %v", setup.Name, err)
			st.Unlock()
		}
	})

	// validate that the SDK is of the acceptable type, no non-regular SDKs are
	// allowed from outside
	info, err := wp.SdkInfo(ctx, setup.Name)
	if err != nil {
		return err
	}

	// add SDK's plugs and slots
	if err := m.repo.AddSdk(info); err != nil {
		return err
	}
	rev.Add(func() {
		if err := m.repo.RemoveSdk(project.ProjectId, w, state.ErrNoState.Error()); err != nil {
			st.Lock()
			task.Logf("Link SDK cleanup: could not remove %q SDK: %v", setup.Name, err)
			st.Unlock()
		}
	})

	if err = policy.CheckInterfaces(info); err != nil {
		return err
	}
	if len(info.BadInterfaces) > 0 {
		st := task.State()
		st.Lock()
		task.Logf("%s", sdk.BadInterfacesSummary(info))
		st.Unlock()
	}

	rev.Success()
	return nil
}

func (m *SdkManager) undoLinkSdk(task *state.Task, tomb *tomb.Tomb) error {
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

	if err := m.repo.RemoveSdk(project.ProjectId, w, sdkSetup.Name); err != nil {
		return err
	}

	wp, err := m.backend.Workshop(ctx, w)
	if err != nil {
		return err
	}

	return wp.UnlinkSdk(ctx, sdkSetup.Name)
}
