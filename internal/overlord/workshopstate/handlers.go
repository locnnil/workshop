package workshopstate

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/tomb.v2"

	. "github.com/canonical/workshop/internal/overlord/handlersetup"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/revert"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshopbackend"
)

var StopLogInterval = 30 * time.Second

var StopWorkshop = (workshopbackend.WorkshopBackend).StopWorkshop

func (m *WorkshopManager) undoCreateWorkshop(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	return m.backend.RemoveWorkshop(ctx, workshop)
}

func (m *WorkshopManager) installAgentSdk(wfs workshopbackend.WorkshopFs, base string) error {
	agentMetaDir := filepath.Join(sdk.SdkCurrentPath("agent"), "meta")
	if err := wfs.MkdirAll(agentMetaDir, 0655); err != nil {
		return err
	}

	// /var/lib/workshop/sdk/agent/current/meta
	file, err := wfs.OpenFile(filepath.Join(agentMetaDir, "sdk.yaml"), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return err
	}

	if _, err = file.Write([]byte(sdk.AgentSdkMeta(base))); err != nil {
		return err
	}
	return nil
}

func (m *WorkshopManager) doCreateWorkshop(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	st := task.State()

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	var wf workshopbackend.WorkshopFile
	st.Lock()
	err = task.Get("workshop-file", &wf)
	st.Unlock()

	if err != nil {
		return fmt.Errorf("internal error: %q workshop configuration is not found (task ID: %s)", workshop, task.ID())
	}

	var rev revert.Reverter
	defer rev.Fail()
	if err = m.backend.LaunchWorkshop(ctx, &wf); err != nil {
		return err
	}

	// clean up must not be cancelled if the parent was cancelled and the change
	// is winding down
	revertCtx := context.WithoutCancel(ctx)
	rev.Add(func() {
		_ = m.backend.RemoveWorkshop(revertCtx, workshop)
	})

	wfs, err := m.backend.WorkshopFs(ctx, workshop)
	if err != nil {
		return err
	}
	defer wfs.Close()

	if err = m.installAgentSdk(wfs, wf.Base); err != nil {
		return err
	}
	rev.Success()
	return nil
}

func (m *WorkshopManager) doMountProject(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	// Configure workshop core properties: project directory
	var prjMount = workshopbackend.Mount(workshopbackend.LxdConfigProjectPathDevice, prj.Path, workshopbackend.WorkshopProjectPath)
	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	return m.backend.AddWorkshopDevice(ctx, workshop, prjMount)
}

func (m *WorkshopManager) undoMountProject(task *state.Task, tomb *tomb.Tomb) error {
	return nil
}

func (m *WorkshopManager) doStart(task *state.Task, tomb *tomb.Tomb) error {
	user, project, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, project.ProjectId)
	defer cancel()

	st := task.State()
	st.Lock()
	task.Set("force", true)
	st.Unlock()

	return m.backend.StartWorkshop(ctx, workshop)
}

func (m *WorkshopManager) doStop(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
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
		stopped <- StopWorkshop(m.backend, stopctx, workshop, force)
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

func (m *WorkshopManager) doRemoveWorkshop(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	if err := m.backend.RemoveWorkshop(ctx, workshop); err != nil {
		return err
	}

	if err = m.cleanUpWorkshopAfterRemoval(user, prj.ProjectId, workshop); err != nil {
		st := task.State()
		st.Lock()
		defer st.Unlock()
		task.Logf("%v", err)
	}

	return nil
}

func (m *WorkshopManager) doRemoveWorkshopStash(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	return m.backend.RemoveWorkshopStash(ctx, workshop)
}

func (m *WorkshopManager) doStashWorkshop(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	if err = m.backend.StashWorkshop(ctx, workshop); err != nil {
		return err
	}
	return nil
}

func (m *WorkshopManager) undoStashWorkshop(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	if err = m.backend.UnstashWorkshop(ctx, workshop); err != nil {
		return err
	}
	return nil
}

func (m *WorkshopManager) doCreateStateStorage(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	return m.backend.CreateStateStorage(ctx, workshop)
}

func (m *WorkshopManager) doRemoveStateStorage(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	return m.backend.DeleteStateStorage(ctx, workshop)
}

type cleanupError struct {
	errs []error
}

func (e *cleanupError) Error() string {
	return fmt.Sprintf("workshop cleanup errors: %v", e.errs)
}

func (m *WorkshopManager) cleanUpWorkshopAfterRemoval(user, projectId, workshop string) error {
	usr, err := workshopbackend.LookupUsername(user)
	if err != nil {
		return err
	}

	var errors []error
	projectContent := filepath.Join(usr.HomeDir, ".local", "share", "workshop", "project", projectId, "content")
	var contentDirs []fs.DirEntry
	if contentDirs, err = os.ReadDir(projectContent); err != nil {
		errors = append(errors, fmt.Errorf("%q workshop content directory is not available: %v", workshop, err))
	}

	// Remove all the possible workshop default content interface 'source'
	// locations that could have existed over the workshop's lifecycle. These
	// are not only the ones that exist by the time we remove the workshop.
	// Imagine the following scenario. An SDK added to a workshop and created a
	// content interface plug. Then, the SDK was removed from the workshop via
	// refresh. When we call 'workshop remove', the plug does not exist anymore
	// (nor the SDK profile for this plug); however, the content is still stored
	// on the host and must also be removed alongside the workshop.
	for _, dir := range contentDirs {
		// Remove all default content dirs that belong the workshop. These will be
		// named as <workshop>_<sdk>_<plug>.sdk
		if dir.IsDir() && strings.HasPrefix(filepath.Base(dir.Name()), workshop+"_") {
			if err := os.RemoveAll(filepath.Join(projectContent, dir.Name())); err != nil {
				errors = append(errors, err)
			}
		}
	}

	if len(errors) > 0 {
		return &cleanupError{errs: errors}
	}

	return nil
}
