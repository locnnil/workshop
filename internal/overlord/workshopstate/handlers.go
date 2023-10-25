package workshopstate

import (
	"context"
	"fmt"
	"time"

	. "github.com/canonical/workshop/internal/overlord/statecontext"
	"github.com/canonical/workshop/internal/workshopbackend"

	"github.com/canonical/workshop/internal/overlord/state"

	"gopkg.in/tomb.v2"
)

var StopLogInterval = 30 * time.Second

var StopWorkspace = (workshopbackend.WorkspaceBackend).StopWorkspace

func (m *WorkspaceManager) undoCreateWorkspace(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	return m.backend.RemoveWorkspace(ctx, workshop)
}

func (m *WorkspaceManager) doCreateWorkspace(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workshop, err := UserProjectWorkspace(task)
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
		return fmt.Errorf("cannot get workshop base for task %q: %v", task.ID(), err)
	}

	/* Launch a workshop with the required base */
	return m.backend.LaunchWorkspace(ctx, workshop,
		base)
}

func (m *WorkspaceManager) doMountProject(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	/* Configure workshop core properties: project directory */
	var prjMount = workshopbackend.WorkspaceDevice{
		Name:       workshopbackend.ProjectPathDevice,
		Properties: map[string]string{"type": "disk", "source": prj.Path, "path": "/project"},
	}

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	return m.backend.AddWorkspaceDevice(ctx, workshop, prjMount)
}

func (m *WorkspaceManager) undoMountProject(task *state.Task, tomb *tomb.Tomb) error {
	return nil
}

func (m *WorkspaceManager) doStart(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workshop, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project)
	defer cancel()

	return m.backend.StartWorkspace(ctx, workshop)
}

func (m *WorkspaceManager) doRemoveWorkspace(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	return m.backend.RemoveWorkspace(ctx, workshop)
}

func (m *WorkspaceManager) doRemoveWorkspaceStash(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	return m.backend.RemoveWorkspaceStash(ctx, workshop)
}

func (m *WorkspaceManager) doStashWorkspace(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	return m.backend.StashWorkspace(ctx, workshop)
}

func (m *WorkspaceManager) undoStashWorkspace(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	return m.backend.UnstashWorkspace(ctx, workshop)
}

func (m *WorkspaceManager) doStop(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkspace(task)
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
		stopped <- StopWorkspace(m.backend, stopctx, workshop, force)
	}()

	for {
		select {
		case err = <-stopped:
			return err
		case <-time.After(StopLogInterval):
			st.Lock()
			task.Logf("Still waiting for %q to stop; no change in the last 30 seconds...", workshop)
			st.Unlock()
		}
	}
}

func (m *WorkspaceManager) doCreateStateStorage(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	return m.backend.CreateStateStorage(ctx, workshop)
}

func (m *WorkspaceManager) doRemoveStateStorage(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkspace(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	return m.backend.DeleteStateStorage(ctx, workshop)
}
