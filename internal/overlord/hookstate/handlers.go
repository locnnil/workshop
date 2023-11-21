package hookstate

import (
	"context"
	"errors"
	"fmt"

	"github.com/canonical/workshop/internal/overlord/state"
	. "github.com/canonical/workshop/internal/overlord/statecontext"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshopbackend"

	"github.com/spf13/afero"
	"gopkg.in/tomb.v2"
)

func (h *HookManager) doRunHook(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workshop, err := UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, user, prj)
	defer cancel()

	st := task.State()
	st.Lock()
	var hook HookSetup
	err = task.Get("hook-setup", &hook)
	st.Unlock()
	if err != nil {
		return err
	}

	if hook.HookType == SaveState || hook.HookType == RestoreState {
		err := h.backend.AddWorkshopDevice(ctx, workshop, workshopbackend.WorkshopDevice{
			Name: workshopbackend.WorkshopStateVolumeName(workshop, prj.ProjectId),
			Properties: map[string]string{"type": "disk",
				"pool":   "default",
				"path":   workshopbackend.WorkshopStateDir,
				"source": workshopbackend.WorkshopStateVolumeName(workshop, prj.ProjectId)},
		})
		if err != nil {
			return fmt.Errorf("cannot run hook %q for SDK %q: %w", hook.Type(), hook.Sdk.Name, err)
		}

		defer func() {
			h.backend.RemoveWorkshopDevice(ctx, workshop, workshopbackend.WorkshopStateVolumeName(workshop, prj.ProjectId))
		}()
	}

	switch hook.HookType {
	case SaveState:
		{
			fs, err := h.backend.WorkshopFs(ctx, workshop)
			if err != nil {
				return fmt.Errorf("cannot run hook \"save-sate\" for %q SDK: %v", hook.Sdk.Name, err)
			}
			err = fs.MkdirAll(hook.Environment["SDK_STATE_DIR"], 0755)
			fs.Close()
			if err != nil {
				return fmt.Errorf("cannot run hook \"save-sate\" for %q SDK: %v", hook.Sdk.Name, err)
			}
		}
		return h.executeHook(ctx, task, workshop, prj.ProjectId, &hook)
	case RestoreState:
		{
			fs, err := h.backend.WorkshopFs(ctx, workshop)
			if err != nil {
				return fmt.Errorf("cannot run hook \"restore-sate\" for %q SDK: %v", hook.Sdk.Name, err)
			}
			info, err := fs.Stat(hook.Environment["SDK_STATE_DIR"])
			fs.Close()
			if err != nil {
				return fmt.Errorf("cannot run hook \"restore-sate\" for %q SDK: %v", hook.Sdk.Name, err)
			}

			if !info.IsDir() {
				return fmt.Errorf("cannot run hook \"restore-sate\" for %q SDK: state storage path is not a directory", hook.Sdk.Name)
			}
		}
		return h.executeHook(ctx, task, workshop, prj.ProjectId, &hook)
	default:
		return h.executeHook(ctx, task, workshop, prj.ProjectId, &hook)
	}
}

func (h *HookManager) executeHook(ctx context.Context, task *state.Task, workshop, projectId string, hook *HookSetup) error {
	hookPath := sdk.SdkHookPath(hook.Sdk.Name, hook.Type())

	//
	wsFs, err := h.backend.WorkshopFs(ctx, workshop)
	if err != nil {
		return err
	}

	info, err := wsFs.Stat(hookPath)
	wsFs.Close()
	if errors.Is(err, afero.ErrFileNotFound) || !info.Mode().IsRegular() {
		return nil
	}

	/* create a memory out/err to log the hook output into the task's log */
	memFs := afero.NewMemMapFs()
	out, err := memFs.Create(workshopbackend.InstanceName(workshop, projectId))
	if err != nil {
		return err
	}

	args := workshopbackend.Execution{
		ExecArgs: workshopbackend.ExecArgs{
			UserId:  0,
			GroupId: 0,
			Command: []string{
				"bash",
				"-ue",
				"-o",
				"pipefail",
				"-c",
				hookPath,
			},
			Environment: hook.Environment,
			WorkDir:     sdk.SdkHooksDir(hook.Sdk.Name),
		},
		ExecControls: workshopbackend.ExecControls{
			Stdin:  nil,
			Stdout: out,
			Stderr: out,
		},
	}

	exectx, err := h.backend.Exec(ctx, workshop, &args)
	if err != nil {
		return err
	}

	err = exectx.WaitExecution(ctx)
	hookLog, _ := afero.ReadFile(memFs, out.Name())

	st := task.State()
	st.Lock()
	defer st.Unlock()
	if len(hookLog) > 0 {
		task.Logf(string(hookLog))
	}

	return err
}
