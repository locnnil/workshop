package workspacestate

import (
	"context"
	"fmt"
	"time"

	. "github.com/canonical/workspace/internal/overlord/statecontext"
	"github.com/canonical/workspace/internal/workspacebackend"

	"github.com/canonical/workspace/internal/overlord/state"

	"gopkg.in/tomb.v2"
)

var StopLogInterval = 30 * time.Second

var StopWorkspace = (workspacebackend.WorkspaceBackend).StopWorkspace

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

	return m.backend.RemoveWorkspace(ctx, workspace)
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

func (m *WorkspaceManager) doRemoveWorkspace(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	st := task.State()
	st.Lock()
	defer st.Unlock()

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	return m.backend.RemoveWorkspace(ctx, workspace)
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

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	var force bool
	st := task.State()
	st.Lock()
	// false is by default
	_ = task.Get("force", &force)
	st.Unlock()

	var stopped = make(chan error)
	stopctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		// LXD has an internal timeout (30 seconds) for the operation,
		// if exceeded, the dealine error will be returned
		stopped <- StopWorkspace(m.backend, stopctx, workspace, force)
	}()

	for {
		select {
		case err = <-stopped:
			return err
		case <-time.After(StopLogInterval):
			st.Lock()
			task.Logf("Still waiting for %q to stop; no change in the last 30 seconds...", workspace)
			st.Unlock()
		}
	}
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

func (m *WorkspaceManager) doExecCommand(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workspace, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	var setup workspacebackend.ExecArgs
	st := task.State()
	st.Lock()
	err = task.Get("exec-setup", &setup)
	st.Unlock()
	if err != nil {
		return fmt.Errorf("cannot get exec setup object for task %q: %v", task.ID(), err)
	}

	exectx, err := m.backend.Exec(ctx, workspace, &workspacebackend.Execution{ExecArgs: setup})
	if err != nil {
		return err
	}
	st.Lock()
	task.Set("control", exectx.DescriptorWebsockets["control"])
	task.Set("stdio", exectx.DescriptorWebsockets["stdio"])
	if !setup.Interactive {
		task.Set("stdout", exectx.DescriptorWebsockets["stdout"])
		task.Set("stderr", exectx.DescriptorWebsockets["stderr"])
	}
	st.Unlock()

	m.execChannelsLock.Lock()
	if execCh, ok := m.execChannels[task.ID()]; ok {
		close(execCh)
		delete(m.execChannels, task.ID())
	}
	m.execChannelsLock.Unlock()

	if setup.Timeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, setup.Timeout)
		defer cancel()
	}

	err = exectx.WaitExecution(ctx)
	st.Lock()
	var status = 0
	// only set the error exit status in the task's metadata if the error
	// belongs to the command execution (e.g. not an LXD error)
	if err == nil {
		task.Set("api-data", map[string]interface{}{
			"exit-code": status,
		})
	} else {
		if execerr, ok := err.(*workspacebackend.ErrExec); ok {
			status = execerr.Status
			task.Set("api-data", map[string]interface{}{
				"exit-code": status,
			})
		}
	}
	st.Unlock()

	return err
}
