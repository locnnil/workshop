package workspace

import (
	"fmt"
	"path/filepath"

	util "github.com/canonical/workspace/internal"
	store "github.com/canonical/workspace/internal/fakestore"
	"github.com/canonical/workspace/internal/overlord/state"
	srv "github.com/canonical/workspace/internal/server"
	"gopkg.in/tomb.v2"
)

type WorkspaceManager struct {
	server srv.WorkspaceServer
}

func NewWorkspaceManager(runner *state.TaskRunner, server srv.WorkspaceServer) *WorkspaceManager {
	manager := &WorkspaceManager{
		server: server,
	}

	runner.AddHandler("create-workspace", manager.doStartBase, nil)
	runner.AddHandler("add-workspace-device", manager.doAddDevice, nil)
	runner.AddHandler("set-workspace-state", manager.doSetState, nil)
	runner.AddHandler("install-sdk", manager.doInstallSDK, nil)

	return manager
}

func (w *WorkspaceManager) Ensure() error {
	return nil
}

func (m *WorkspaceManager) doStartBase(task *state.Task, tomb *tomb.Tomb) error {
	project, workspace, err := m.changeData(task)
	if err != nil {
		return err
	}

	st := task.State()
	var base string

	st.Lock()
	err = task.Get("base", &base)
	st.Unlock()

	if err != nil {
		return fmt.Errorf("cannot get workspace base for task %q: %v", task.ID(), err)
	}

	fmt.Printf("Setting up workspace \"%s\"...\n", workspace)
	/* Launch a workspace with the required base */
	if err := m.server.LaunchWorkspaceInstance(workspace,
		base, project.ProjectId); err != nil {
		return err
	}

	return nil
}

func (m *WorkspaceManager) doAddDevice(task *state.Task, tomb *tomb.Tomb) error {
	project, workspace, err := m.changeData(task)
	if err != nil {
		return err
	}

	/* Configure workspace core properties: project directory */
	var prjMount = srv.WorkspaceDevice{
		Name:       ProjectDevice,
		Properties: map[string]string{"type": "disk", "source": project.Path, "path": "/project"},
	}

	if err = m.server.AddWorkspaceDevice(workspace, project.ProjectId, prjMount); err != nil {
		return err
	}
	return nil
}

func (m *WorkspaceManager) doSetState(task *state.Task, tomb *tomb.Tomb) error {
	project, workspace, err := m.changeData(task)
	if err != nil {
		return err
	}

	st := task.State()
	var state string

	st.Lock()
	err = task.Get("workspace-state", &state)
	st.Unlock()

	if err != nil {
		return fmt.Errorf("cannot get workspace base for task %q: %v", task.ID(), err)
	}

	/* Start the workspace. TODO: make sure that we have it ready before attempting to proceed */
	return m.server.SetWorkspaceState(workspace, project.ProjectId, state)
}

func (m *WorkspaceManager) doInstallSDK(task *state.Task, tomb *tomb.Tomb) error {
	project, workspace, err := m.changeData(task)
	if err != nil {
		return err
	}

	st := task.State()
	var retrieveId string
	var blob store.SdkBlob

	st.Lock()
	err = task.Get("sdk-retrieve-task", &retrieveId)
	st.Unlock()

	if err != nil {
		return err
	}
	st.Lock()
	retrieve := task.State().Task(retrieveId)
	st.Unlock()

	if retrieve == nil {
		return fmt.Errorf("internal error: no corresponding retrieve-sdk task found")
	}

	st.Lock()
	defer st.Unlock()
	if err = retrieve.Get("sdk-blob", &blob); err != nil {
		return err
	}

	fmt.Printf("Setting up SDK \"%s\" from %s revision %d...\n", blob.Name, blob.Channel, blob.Revision)

	filename := store.ToSdkFilename(blob.Name, blob.Revision)
	/* Bind-mount the SDK to the workspace */
	var sdkMount = srv.WorkspaceDevice{
		Name: blob.Name,
		Properties: map[string]string{"type": "disk", "source": filename,
			"path": filepath.Join("/root", filepath.Base(filename))},
	}

	err = m.server.AddWorkspaceDevice(workspace, project.ProjectId, sdkMount)
	if err != nil {
		return err
	}

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

	/* Make sure the SDK file will be unmounted once installed into the workspace */
	m.server.RemoveWorkspaceDevice(workspace, project.ProjectId, sdkMount.Name)

	if err != nil {
		return fmt.Errorf("could not install SDK \"%s\": %v", blob.Name, err)
	}
	return nil
}

func (*WorkspaceManager) changeData(task *state.Task) (*ProjectKey, string, error) {
	st := task.State()
	var project ProjectKey
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
