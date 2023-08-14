package workspacestate

import (
	"fmt"

	. "github.com/canonical/workspace/internal/overlord/statecontext"
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

	return m.backend.DeleteWorkspace(ctx, workspace)
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

	return m.backend.AddWorkspaceDevice(ctx, workspace, prjMount)
}

func (m *WorkspaceManager) undoMountProject(task *state.Task, tomb *tomb.Tomb) error {
	return nil
}

func (m *WorkspaceManager) doStart(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	return m.backend.StartWorkspace(ctx, workspace)
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

	return m.backend.DeleteWorkspace(ctx, workspace)
}

func (m *WorkspaceManager) doRemoveWorkspaceStash(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	return m.backend.RemoveWorkspaceStash(ctx, workspace)
}

func (m *WorkspaceManager) doStashWorkspace(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	return m.backend.StashWorkspace(ctx, workspace)
}

func (m *WorkspaceManager) undoStashWorkspace(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	return m.backend.UnstashWorkspace(ctx, workspace)
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

	return m.backend.StopWorkspace(ctx, workspace, false)
}

func (m *WorkspaceManager) doCreateStateStorage(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	return m.backend.CreateStateStorage(ctx, workspace)
}

func (m *WorkspaceManager) doRemoveStateStorage(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	return m.backend.DeleteStateStorage(ctx, workspace)
}
