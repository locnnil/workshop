package workspacestate

import (
	"fmt"

	"github.com/canonical/workspace/internal/overlord/projectstate"
	. "github.com/canonical/workspace/internal/overlord/sharedstate"

	"github.com/canonical/workspace/internal/overlord/state"
	backend "github.com/canonical/workspace/internal/workspacebackend"

	"gopkg.in/tomb.v2"
)

func (m *WorkspaceManager) undoCreateWorkspace(task *state.Task, tomb *tomb.Tomb) error {
	project, workspace, err := ProjectAndWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	return m.backend.DeleteWorkspaceInstance(workspace, project.ProjectId)
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
	return m.backend.LaunchWorkspaceInstance(workspace,
		base, project.ProjectId)
}

func (m *WorkspaceManager) doMountProject(task *state.Task, tomb *tomb.Tomb) error {
	project, workspace, err := ProjectAndWorkspace(task)
	if err != nil {
		return err
	}

	/* Configure workspace core properties: project directory */
	var prjMount = backend.WorkspaceDevice{
		Name:       projectstate.ProjectDevice,
		Properties: map[string]string{"type": "disk", "source": project.Path, "path": "/project"},
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	if err = m.backend.AddWorkspaceDevice(workspace, project.ProjectId, prjMount); err != nil {
		return err
	}
	return nil
}

func (m *WorkspaceManager) undoMountProject(task *state.Task, tomb *tomb.Tomb) error {
	return nil
}

func (m *WorkspaceManager) doStart(task *state.Task, tomb *tomb.Tomb) error {
	project, workspace, err := ProjectAndWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	/* Start the workspace. TODO: make sure that we have it ready before attempting to proceed */
	err = m.backend.SetWorkspaceState(workspace, project.ProjectId, "start")
	if err != nil {
		return err
	}

	/* Wait until system is up an running before returning */
	args := backend.ExecArgs{
		User: "root",
		Command: []string{
			"bash", "-c", "while " +
				"[ \"$(systemctl is-system-running 2>/dev/null)\" != \"running\" ] && " +
				"[ \"$(systemctl is-system-running 2>/dev/null)\" != \"degraded\" ]; do :; done",
		},
		WorkDir: "/",
		Stdin:   nil,
		Stdout:  nil,
		Stderr:  nil}

	if done, err := m.backend.Exec(workspace, project.ProjectId, &args); err != nil {
		return err
	} else {
		<-done
	}
	return nil
}

func (m *WorkspaceManager) undoStart(task *state.Task, tomb *tomb.Tomb) error {
	project, workspace, err := ProjectAndWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	/* Start the workspace. TODO: make sure that we have it ready before attempting to proceed */
	err = m.backend.SetWorkspaceState(workspace, project.ProjectId, "stop")
	if err != nil {
		return err
	}

	return nil
}
