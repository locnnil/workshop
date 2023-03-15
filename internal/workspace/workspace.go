package workspace

import (
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
	. "github.com/canonical/workspace/internal/server"
	"gopkg.in/tomb.v2"
)

type WorkspaceManager struct {
	server WorkspaceServer
}

func NewWorkspaceManager(runner *state.TaskRunner, server WorkspaceServer) *WorkspaceManager {
	manager := &WorkspaceManager{
		server: server,
	}

	runner.AddHandler("create-workspace-base", manager.doStartBase, nil)
	runner.AddHandler("add-device", manager.doAddDevice, nil)
	runner.AddHandler("set-workspace-state", manager.doSetState, nil)

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
	err = task.Get("workspace-base", &base)
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
	var prjMount = WorkspaceDevice{
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

// func (w *WorkspaceInstance) Launch(client store.StoreClient) error {
// 	var err error

// 	fmt.Printf("Setting up workspace \"%s\"...\n", w.Name)

// 	/* Launch a workspace with the required base */
// 	if err := w.server.LaunchWorkspaceInstance(w.Name, w.Base, w.project.ProjectId()); err != nil {
// 		return err
// 	}

// 	/* Configure workspace core properties: project directory */
// 	var prjMount = srv.WorkspaceDevice{
// 		Name:       ProjectDevice,
// 		Properties: map[string]string{"type": "disk", "source": w.project.ProjectDirectory(), "path": "/project"},
// 	}

// 	if err = w.server.AddWorkspaceDevice(w.Name, w.project.ProjectId(), prjMount); err != nil {
// 		return err
// 	}

// 	/* Start the workspace. TODO: make sure that we have it ready before attempting to proceed */
// 	if err = w.Start(); err != nil {
// 		return err
// 	}

// 	for sdkName, sdk := range w.Sdks {

// 		/* Download SDK */
// 		blob, err := client.FetchSDK(sdkName, sdk.Channel, util.SdksDir)
// 		if err != nil {
// 			return err
// 		}
// 		fmt.Printf("Setting up SDK \"%s\" from %s revision %d...\n", sdkName, sdk.Channel, blob.Revision)

// 		/* Install SDK */
// 		err = w.installSDK(blob)
// 		if err != nil {
// 			return err
// 		}

// 		/* TODO: Run lifecycle hooks */
// 	}

// 	fmt.Printf("Workspace \"%s\" started.\n", w.Name)

// 	return nil
// }

// func (w *WorkspaceInstance) installSDK(blob store.SDKFile) error {
// 	/* Bind-mount the SDK to the workspace */
// 	var sdkMount = srv.WorkspaceDevice{
// 		Name: blob.Name,
// 		Properties: map[string]string{"type": "disk", "source": blob.Filename,
// 			"path": filepath.Join("/root", filepath.Base(blob.Filename))},
// 	}

// 	err := w.server.AddWorkspaceDevice(w.Name, w.project.ProjectId(), sdkMount)
// 	if err != nil {
// 		return err
// 	}

// 	/* Unpack the SDK to the desired location in the workspace
// 	   Note: the following command requires ~ tar >= 1.29 due to --one-top-level */

// 	args := srv.ExecArgs{User: "root", Command: []string{
// 		"tar",
// 		"--extract",
// 		"--file",
// 		sdkMount.Properties["path"],
// 		"--one-top-level=" + filepath.Join(util.WorkspaceSdksDir, blob.Name),
// 		"--no-same-owner",
// 	}, Stdin: nil, Stdout: nil, Stderr: nil}
// 	done, err := w.server.Exec(w.Name, w.project.ProjectId(), &args)

// 	/* The server will close this channel when exec is finished and no i/o remains outstanding */
// 	<-done

// 	/* Make sure the SDK file will be unmounted once installed into the workspace */
// 	w.server.RemoveWorkspaceDevice(w.Name, w.project.ProjectId(), sdkMount.Name)

// 	if err != nil {
// 		fmt.Printf("could not install \"%s\": %v", blob.Name, err)
// 	}
// 	return nil
// }

// func (w *WorkspaceInstance) Start() error {
// 	return w.server.SetWorkspaceState(w.Name, w.project.ProjectId(), "start")
// }
