package sdkstate

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/canonical/workspace/internal/dirs"
	store "github.com/canonical/workspace/internal/fakestore"
	"github.com/canonical/workspace/internal/logger"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/sdk"
	"github.com/canonical/workspace/internal/workspacebackend"

	. "github.com/canonical/workspace/internal/overlord/statecontext"

	"github.com/spf13/afero"

	"gopkg.in/tomb.v2"
)

func SdkSetup(task *state.Task) (*sdk.SdkInfo, error) {
	st := task.State()
	st.Lock()
	defer st.Unlock()

	var retrieveId string
	var blob sdk.SdkInfo

	err := task.Get("sdk-retrieve-task", &retrieveId)

	if err != nil {
		return nil, err
	}

	retrieve := task.State().Task(retrieveId)
	if retrieve == nil {
		return nil, fmt.Errorf("internal error: no corresponding retrieve-sdk task found")
	}

	if err = retrieve.Get("sdk-setup", &blob); err != nil {
		return nil, err
	}
	return &blob, nil
}

func (m *SdkManager) doRetrieveSdk(task *state.Task, tomb *tomb.Tomb) error {
	st := task.State()
	var sdk workspacebackend.Sdk

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

func sdkBlobDevice(sdk *sdk.SdkInfo) workspacebackend.WorkspaceDevice {
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

	blob, err := SdkSetup(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	sdkMount := sdkBlobDevice(blob)

	err = m.backend.AddWorkspaceDevice(ctx, workspace, sdkMount)
	if err != nil {
		return err
	}

	cleanup := func() {
		/* Make sure the SDK file will be unmounted once installed into the workspace */
		if err := m.backend.RemoveWorkspaceDevice(ctx, workspace, sdkMount.Name); err != nil {
			logger.Debugf("cannot unmount SDK %q from workspace %q: %v", sdkMount.Name, workspace, err)
		}
	}

	defer cleanup()

	/* example: /var/lib/workspace/sdk/cuda/712/ */
	sdkPath := filepath.Join(sdk.WorkspaceSdksDir, blob.Name,
		strconv.Itoa(int(blob.Revision)))

	/* create a memory out/err to log the hook output into the task's log */
	memFs := afero.NewMemMapFs()
	out, err := memFs.Create(workspacebackend.InstanceName(workspace, project.ProjectId))
	if err != nil {
		return err
	}

	/* Unpack the SDK to the desired location in the workspace
	   Note: the following command requires ~ tar >= 1.29 due to --one-top-level */
	args := workspacebackend.ExecArgs{
		User: "root",
		Command: []string{
			"tar",
			"--extract",
			"--file",
			sdkMount.Properties["path"],
			"--one-top-level=" + sdkPath,
			"--no-same-owner",
		},
		WorkDir: "/",
		Stdin:   nil,
		Stdout:  out,
		Stderr:  out}
	done, err := m.backend.Exec(ctx, workspace, &args)

	/* The server will close this channel when exec is finished and no i/o remains outstanding */
	<-done

	if err != nil {
		hookLog, _ := afero.ReadFile(memFs, out.Name())
		return fmt.Errorf("%v: %v", err, string(hookLog))
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

	st := task.State()
	st.Lock()
	defer st.Unlock()
	sdkMount := sdkBlobDevice(blob)

	fs, err := m.backend.GetWorkspaceFs(ctx, workspace)
	if err != nil {
		return err
	}
	defer fs.Close()

	err = fs.RemoveAll(filepath.Join(sdk.WorkspaceSdksDir, blob.Name))
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

	blob, err := SdkSetup(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	props, err := m.backend.GetWorkspace(ctx, workspace)
	if err != nil {
		return err
	}

	return props.LinkSdk(ctx, blob)
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

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	props, err := m.backend.GetWorkspace(ctx, workspace)
	if err != nil {
		return err
	}

	return props.UnlinkSdk(ctx, blob)
}
