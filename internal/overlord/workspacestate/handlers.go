package workspace

import (
	"fmt"
	"path/filepath"

	util "github.com/canonical/workspace/internal"
	store "github.com/canonical/workspace/internal/fakestore"
	"github.com/canonical/workspace/internal/logger"
	"github.com/canonical/workspace/internal/overlord/projectstate"
	"github.com/canonical/workspace/internal/overlord/state"
	srv "github.com/canonical/workspace/internal/server"
	"gopkg.in/tomb.v2"
)

func (m *WorkspaceManager) undoCreateWorkspace(task *state.Task, tomb *tomb.Tomb) error {
	project, workspace, err := ProjectAndWorkspace(task)
	if err != nil {
		return err
	}

	return m.server.DeleteWorkspaceInstance(workspace, project.ProjectId)
}

func (m *WorkspaceManager) doCreateWorkspace(task *state.Task, tomb *tomb.Tomb) error {
	project, workspace, err := ProjectAndWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	var base string
	err = task.Get("base", &base)

	if err != nil {
		return fmt.Errorf("cannot get workspace base for task %q: %v", task.ID(), err)
	}

	fmt.Printf("Setting up workspace \"%s\"...\n", workspace)
	/* Launch a workspace with the required base */
	return m.server.LaunchWorkspaceInstance(workspace,
		base, project.ProjectId)
}

func (m *WorkspaceManager) doAddDevice(task *state.Task, tomb *tomb.Tomb) error {
	project, workspace, err := ProjectAndWorkspace(task)
	if err != nil {
		return err
	}

	/* Configure workspace core properties: project directory */
	var prjMount = srv.WorkspaceDevice{
		Name:       projectstate.ProjectDevice,
		Properties: map[string]string{"type": "disk", "source": project.Path, "path": "/project"},
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	if err = m.server.AddWorkspaceDevice(workspace, project.ProjectId, prjMount); err != nil {
		return err
	}
	return nil
}

func (m *WorkspaceManager) doSetState(task *state.Task, tomb *tomb.Tomb) error {
	project, workspace, err := ProjectAndWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	var state string
	err = task.Get("workspace-state", &state)

	if err != nil {
		return fmt.Errorf("cannot get workspace base for task %q: %v", task.ID(), err)
	}

	/* Start the workspace. TODO: make sure that we have it ready before attempting to proceed */
	return m.server.SetWorkspaceState(workspace, project.ProjectId, state)
}

func (m *WorkspaceManager) doInstallSDK(task *state.Task, tomb *tomb.Tomb) error {
	project, workspace, err := ProjectAndWorkspace(task)
	if err != nil {
		return err
	}

	blob, err := sdkData(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	fmt.Printf("Setting up SDK \"%s\" from %s revision %d...\n", blob.Name, blob.Channel, blob.Revision)

	sdkMount := sdkBlobDevice(blob)

	err = m.server.AddWorkspaceDevice(workspace, project.ProjectId, sdkMount)
	if err != nil {
		return err
	}

	cleanup := func() {
		/* Make sure the SDK file will be unmounted once installed into the workspace */
		if err := m.server.RemoveWorkspaceDevice(workspace, project.ProjectId, sdkMount.Name); err != nil {
			logger.Debugf("cannot unmount SDK blob %q from workspace %q: %v", sdkMount.Name, workspace, err)
		}
	}

	defer cleanup()

	/* Unpack the SDK to the desired location in the workspace
	   Note: the following command requires ~ tar >= 1.29 due to --one-top-level */
	args := srv.ExecArgs{User: "root", Command: []string{
		"tar",
		"--extract",
		"--file",
		sdkMount.Properties["path"],
		"--one-top-level=" + filepath.Join(util.WorkspaceSdksDir, blob.Name),
		"--no-same-owner",
	}, Stdin: nil, Stdout: nil, Stderr: nil}
	done, err := m.server.Exec(workspace, project.ProjectId, &args)

	/* The server will close this channel when exec is finished and no i/o remains outstanding */
	<-done

	return err
}

func (m *WorkspaceManager) undoInstallSdk(task *state.Task, tomb *tomb.Tomb) error {
	project, workspace, err := ProjectAndWorkspace(task)
	if err != nil {
		return err
	}

	blob, err := sdkData(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()
	sdkMount := sdkBlobDevice(blob)

	args := srv.ExecArgs{User: "root", Command: []string{
		"rm",
		"-rf",
		"--",
		filepath.Join(util.WorkspaceSdksDir, blob.Name),
	}, Stdin: nil, Stdout: nil, Stderr: nil}
	done, err := m.server.Exec(workspace, project.ProjectId, &args)

	<-done

	if err != nil {
		logger.Debugf("cannot remove SDK %q from workspace %q, reason: %v", sdkMount.Name, workspace, err)
		return fmt.Errorf("cannot undo SDK %q installation: %w", sdkMount.Name, err)
	}

	return nil
}

func sdkData(task *state.Task) (*store.SdkBlob, error) {
	st := task.State()
	st.Lock()
	defer st.Unlock()

	var retrieveId string
	var blob store.SdkBlob

	err := task.Get("sdk-retrieve-task", &retrieveId)

	if err != nil {
		return nil, err
	}

	retrieve := task.State().Task(retrieveId)
	if retrieve == nil {
		return nil, fmt.Errorf("internal error: no corresponding retrieve-sdk task found")
	}

	if err = retrieve.Get("sdk-blob", &blob); err != nil {
		return nil, err
	}
	return &blob, nil
}

func sdkBlobDevice(sdk *store.SdkBlob) srv.WorkspaceDevice {
	filename := store.ToSdkFilename(sdk.Name, sdk.Revision)

	/* Bind-mount the SDK to the workspace */
	return srv.WorkspaceDevice{
		Name: sdk.Name,
		Properties: map[string]string{"type": "disk", "source": filename,
			"path": filepath.Join("/root", filepath.Base(filename))},
	}
}

func ProjectAndWorkspace(task *state.Task) (*projectstate.ProjectKey, string, error) {
	st := task.State()
	var project projectstate.ProjectKey
	var name string

	st.Lock()
	err := task.Change().Get("project-key", &project)
	st.Unlock()

	if err != nil {
		return nil, "", fmt.Errorf("cannot get project for task %q: %v", task.ID(), err)
	}

	st.Lock()
	err = task.Change().Get("workspace", &name)
	st.Unlock()

	if err != nil {
		return nil, "", fmt.Errorf("cannot get workspace for task %q: %v", task.ID(), err)
	}

	return &project, name, nil
}
