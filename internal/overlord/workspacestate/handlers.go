package workspacestate

import (
	"fmt"

	. "github.com/canonical/workspace/internal/overlord/sthelper"
	"github.com/canonical/workspace/internal/workspacebackend"

	"github.com/canonical/workspace/internal/overlord/state"

	"gopkg.in/tomb.v2"
)

func (m *WorkspaceManager) undoCreateWorkspace(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	return m.backend.DeleteWorkspace(ctx, workspace, true)
}

func (m *WorkspaceManager) doCreateWorkspace(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	var base string
	st.Lock()
	err = task.Get("base", &base)
	st.Unlock()

	if err != nil {
		return fmt.Errorf("cannot get workspace base for task %q: %v", task.ID(), err)
	}

	/* Launch a workspace with the required base */
	return m.backend.LaunchWorkspace(ctx, workspace,
		base)
}

func (m *WorkspaceManager) doMountProject(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	/* Configure workspace core properties: project directory */
	var prjMount = workspacebackend.WorkspaceDevice{
		Name:       workspacebackend.ProjectPathDevice,
		Properties: map[string]string{"type": "disk", "source": prj.Path, "path": "/project"},
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	if err = m.backend.AddWorkspaceDevice(ctx, workspace, prjMount); err != nil {
		return err
	}
	return nil
}

func (m *WorkspaceManager) undoMountProject(task *state.Task, tomb *tomb.Tomb) error {
	return nil
}

func (m *WorkspaceManager) doStart(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	/* Start the workspace. TODO: make sure that we have it ready before attempting to proceed */
	err = m.backend.SetWorkspaceState(ctx, workspace, "start")
	st.Unlock()
	if err != nil {
		return err
	}

	/* Wait until system is up an running before returning */
	args := workspacebackend.ExecArgs{
		User: "root",
		Command: []string{
			"bash", "-eu", "-c", "while " +
				"[ \"$(systemctl is-system-running 2>/dev/null)\" != \"running\" ] && " +
				"[ \"$(systemctl is-system-running 2>/dev/null)\" != \"degraded\" ]; do :; done",
		},
		WorkDir: "/",
		Stdin:   nil,
		Stdout:  nil,
		Stderr:  nil}

	if done, err := m.backend.Exec(ctx, workspace, &args); err != nil {
		return err
	} else {
		<-done
	}
	return nil
}

func (m *WorkspaceManager) doDeleteWorkspace(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	err = m.backend.DeleteWorkspace(ctx, workspace, true)
	if err != nil {
		return err
	}
	return nil
}

func (m *WorkspaceManager) doDeleteUnavailableWorkspace(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	err = m.backend.DeleteUnavailableWorkspace(ctx, workspace)
	if err != nil {
		return err
	}
	return nil
}

func (m *WorkspaceManager) doMakeUnavailable(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	err = m.backend.MakeWorkspaceUnavailable(ctx, workspace)
	if err != nil {
		return err
	}
	return nil
}

func (m *WorkspaceManager) doMakeAvailable(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	err = m.backend.MakeWorkspaceAvailable(ctx, workspace)
	if err != nil {
		return err
	}
	return nil
}

func (m *WorkspaceManager) doStop(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	err = m.backend.SetWorkspaceState(ctx, workspace, "stop")
	if err != nil {
		return err
	}
	return nil
}
