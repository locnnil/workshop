package sdkstate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/canonical/workshop/internal/dirs"
	store "github.com/canonical/workshop/internal/fakestore"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshopbackend"

	. "github.com/canonical/workshop/internal/overlord/handlersetup"

	"github.com/spf13/afero"

	"gopkg.in/tomb.v2"
)

var InstallTimeNow = time.Now

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
	var sdk workshopbackend.SdkRecord

	st.Lock()
	err = task.Get("sdk-record", &sdk)
	st.Unlock()
	if err != nil {
		return err
	}

	ctx, _ := BackendContext(tomb, user, project.ProjectId)
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client := store.NewStoreClient()

	blob, err := client.RetrieveSdk(ctx, sdk.Name, sdk.Channel, dirs.SdkDir)
	if err != nil {
		return err
	}

	st.Lock()
	task.Set("sdk-setup", blob)
	st.Unlock()

	return nil
}

func (m *SdkManager) undoRetrieveSdk(task *state.Task, tomb *tomb.Tomb) error {
	st := task.State()
	st.Lock()
	var setup sdk.Setup
	err := task.Get("sdk-setup", &setup)
	st.Unlock()
	if err != nil {
		return err
	}
	return os.Remove(setup.Filename())
}

func (m *SdkManager) doInstallSDK(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	sdkSetup, err := SdkSetup(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	target := filepath.Join("/root", filepath.Base(sdkSetup.Filename()))
	sdkMount := workshopbackend.Mount(sdkSetup.Name, sdkSetup.Filename(), target)
	if err = m.backend.AddWorkshopDevice(ctx, workshop, sdkMount); err != nil {
		return err
	}

	cleanup := func() {
		// Make sure the SDK file will be unmounted once installed into the workshop
		if err := m.backend.RemoveWorkshopDevice(ctx, workshop, sdkMount.Name()); err != nil {
			logger.Debugf("cannot unmount SDK %q from workshop %q: %v", sdkMount.Name(), workshop, err)
		}
	}

	defer cleanup()

	// example: /var/lib/workshop/sdk/cuda/712/
	sdkPath := filepath.Join(dirs.WorkshopSdksDir, sdkSetup.Name,
		strconv.Itoa(int(sdkSetup.Revision)))

	// create a memory out/err to log the hook output into the task's log
	memFs := afero.NewMemMapFs()
	out, err := memFs.Create(workshopbackend.InstanceName(workshop, project.ProjectId))
	if err != nil {
		return err
	}

	// Unpack the SDK to the desired location in the workshop
	//   Note: the following command requires ~ tar >= 1.29 due to --one-top-level
	args := workshopbackend.Execution{
		ExecArgs: workshopbackend.ExecArgs{
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
		ExecControls: workshopbackend.ExecControls{
			Stdin:  nil,
			Stdout: nil,
			Stderr: out,
		},
	}

	exectx, err := m.backend.Exec(ctx, workshop, &args)
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
	user, project, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	sdkSetup, err := SdkSetup(task)
	if err != nil {
		return err
	}

	fs, err := m.backend.WorkshopFs(ctx, workshop)
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
	user, project, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	setup, err := SdkSetup(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	inst, err := m.backend.Workshop(ctx, workshop)
	if err != nil {
		return err
	}

	if err = inst.LinkSdk(ctx, setup); err != nil {
		return err
	}

	// validate that the SDK is of the acceptable type, no non-regular SDKs are
	// allowed from outside
	info, err := inst.SdkInfo(ctx, setup.Name)
	if err != nil {
		return err
	}

	if info.Type != sdk.Regular {
		return fmt.Errorf("unknown SDK type %q", info.Type.String())
	}

	return nil
}

func (m *SdkManager) undoLinkSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	sdkSetup, err := SdkSetup(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	inst, err := m.backend.Workshop(ctx, workshop)
	if err != nil {
		return err
	}

	return inst.UnlinkSdk(ctx, sdkSetup.Name)
}
