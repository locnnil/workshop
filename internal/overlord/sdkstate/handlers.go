package sdkstate

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/canonical/workshop/internal/dirs"
	store "github.com/canonical/workshop/internal/fakestore"
	"github.com/canonical/workshop/internal/interfaces/policy"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workspacebackend"

	. "github.com/canonical/workshop/internal/overlord/statecontext"

	"github.com/spf13/afero"

	"gopkg.in/tomb.v2"
)

func SdkSetup(task *state.Task) (sdk.Setup, error) {
	st := task.State()
	st.Lock()
	defer st.Unlock()

	var retrieveId string
	var blob sdk.Setup

	err := task.Get("sdk-retrieve-task", &retrieveId)

	if err != nil {
		return sdk.Setup{}, err
	}

	retrieve := task.State().Task(retrieveId)
	if retrieve == nil {
		return sdk.Setup{}, fmt.Errorf("internal error: no corresponding retrieve-sdk task found")
	}

	if err = retrieve.Get("sdk-setup", &blob); err != nil {
		return sdk.Setup{}, err
	}
	return blob, nil
}

func (m *SdkManager) doRetrieveSdk(task *state.Task, tomb *tomb.Tomb) error {
	st := task.State()
	var sdk workspacebackend.SdkRecord

	st.Lock()
	err := task.Get("sdk-setup", &sdk)
	st.Unlock()

	if err != nil {
		return err
	}

	client := store.NewStoreClient()

	blob, err := client.RetrieveSdk(sdk.Name, sdk.Channel, dirs.SdkDir)
	if err != nil {
		return err
	}

	st.Lock()
	task.Set("sdk-setup", blob)
	st.Unlock()

	return nil
}

func sdkBlobDevice(sdk sdk.Setup) workspacebackend.WorkspaceDevice {
	/* Bind-mount the SDK to the workspace */
	return workspacebackend.WorkspaceDevice{
		Name: sdk.Name,
		Properties: map[string]string{"type": "disk", "source": sdk.Filename(),
			"path": filepath.Join("/root", filepath.Base(sdk.Filename()))},
	}
}

func (m *SdkManager) doInstallSDK(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	sdkSetup, err := SdkSetup(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	sdkMount := sdkBlobDevice(sdkSetup)

	if err = m.backend.AddWorkspaceDevice(ctx, workspace, sdkMount); err != nil {
		return err
	}

	cleanup := func() {
		/* Make sure the SDK file will be unmounted once installed into the workspace */
		if err := m.backend.RemoveWorkspaceDevice(ctx, workspace, sdkMount.Name); err != nil {
			logger.Debugf("cannot unmount SDK %q from workspace %q: %v", sdkMount.Name, workspace, err)
		}
	}

	defer cleanup()

	// example: /var/lib/workspace/sdk/cuda/712/
	sdkPath := filepath.Join(dirs.WorkspaceSdksDir, sdkSetup.Name,
		strconv.Itoa(int(sdkSetup.Revision)))

	// create a memory out/err to log the hook output into the task's log
	memFs := afero.NewMemMapFs()
	out, err := memFs.Create(workspacebackend.InstanceName(workspace, project.ProjectId))
	if err != nil {
		return err
	}

	// Unpack the SDK to the desired location in the workspace
	//   Note: the following command requires ~ tar >= 1.29 due to --one-top-level
	args := workspacebackend.Execution{
		ExecArgs: workspacebackend.ExecArgs{
			UserId:  0,
			GroupId: 0,
			Command: []string{
				"tar",
				"--extract",
				"--file",
				sdkMount.Properties["path"],
				"--one-top-level=" + sdkPath,
				"--no-same-owner",
			},
			WorkDir: "/",
		},
		ExecControls: workspacebackend.ExecControls{
			Stdin:  nil,
			Stdout: nil,
			Stderr: out,
		},
	}

	exectx, err := m.backend.Exec(ctx, workspace, &args)
	if err != nil {
		return err
	}

	err = exectx.WaitExecution(ctx)

	if err != nil {
		hookLog, _ := afero.ReadFile(memFs, out.Name())
		return fmt.Errorf("%w: %v", err, string(hookLog))
	}

	return err
}

func (m *SdkManager) undoInstallSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	blob, err := SdkSetup(task)
	if err != nil {
		return err
	}

	sdkMount := sdkBlobDevice(blob)

	fs, err := m.backend.GetWorkspaceFs(ctx, workspace)
	if err != nil {
		return err
	}
	defer fs.Close()

	err = fs.RemoveAll(filepath.Join(dirs.WorkspaceSdksDir, blob.Name))
	if err != nil {
		return fmt.Errorf("cannot undo SDK %q installation: %w", sdkMount.Name, err)
	}

	return nil
}

func (m *SdkManager) doLinkSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	setup, err := SdkSetup(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	inst, err := m.backend.GetWorkspace(ctx, workspace)
	if err != nil {
		return err
	}

	if err = inst.LinkSdk(ctx, setup); err != nil {
		return err
	}

	sdkInfo, err := inst.SdkInfo(ctx, setup)
	if err != nil {
		return nil
	}

	if err = policy.CheckInterfaces(sdkInfo); err != nil {
		return err
	}

	return nil
}

func (m *SdkManager) undoLinkSdk(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	blob, err := SdkSetup(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	inst, err := m.backend.GetWorkspace(ctx, workspace)
	if err != nil {
		return err
	}

	return inst.UnlinkSdk(ctx, blob)
}
