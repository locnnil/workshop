package hookstate

import (
	"context"
	"errors"
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
	. "github.com/canonical/workspace/internal/overlord/statecontext"
	"github.com/canonical/workspace/internal/sdk"
	"github.com/canonical/workspace/internal/workspacebackend"

	"github.com/spf13/afero"
	"gopkg.in/tomb.v2"
)

func (h *HookManager) doRunHook(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, workspace, err := UserProjectWorkspace(task)
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
		err := h.backend.AddWorkspaceDevice(ctx, workspace, workspacebackend.WorkspaceDevice{
			Name: workspacebackend.WorkspaceStateVolumeName(workspace, prj.ProjectId),
			Properties: map[string]string{"type": "disk",
				"pool":   "default",
				"path":   workspacebackend.WorkspaceStateDir,
				"source": workspacebackend.WorkspaceStateVolumeName(workspace, prj.ProjectId)},
		})
		if err != nil {
			return fmt.Errorf("cannot run hook %q for SDK %q: %v", hook.Type(), hook.Sdk.Name, err)
		}

		defer func() {
			h.backend.RemoveWorkspaceDevice(ctx, workspace, workspacebackend.WorkspaceStateVolumeName(workspace, prj.ProjectId))
		}()
	}

	switch hook.HookType {
	case SaveState:
		{
			fs, err := h.backend.GetWorkspaceFs(ctx, workspace)
			if err != nil {
				return fmt.Errorf("cannot run hook \"save-sate\" for SDK %q: %v", hook.Sdk.Name, err)
			}
			err = fs.MkdirAll(hook.Environment["SDK_STATE_DIR"], 0755)
			fs.Close()
			if err != nil {
				return fmt.Errorf("cannot run hook \"save-sate\" for SDK %q: %v", hook.Sdk.Name, err)
			}
		}
		return h.executeHook(ctx, task, workspace, prj.ProjectId, &hook)
	case RestoreState:
		return h.executeHook(ctx, task, workspace, prj.ProjectId, &hook)
	default:
		return h.executeHook(ctx, task, workspace, prj.ProjectId, &hook)
	}
}

func (h *HookManager) executeHook(ctx context.Context, task *state.Task, workspace, projectId string, hook *HookSetup) error {
	hookPath := sdk.SdkHookPath(hook.Sdk.Name, hook.Type())
	{
		wsFs, err := h.backend.GetWorkspaceFs(ctx, workspace)
		if err != nil {
			return err
		}

		info, err := wsFs.Stat(hookPath)
		wsFs.Close()
		if errors.Is(err, afero.ErrFileNotFound) || !info.Mode().IsRegular() {
			return nil
		}
	}

	/* create a memory out/err to log the hook output into the task's log */
	memFs := afero.NewMemMapFs()
	outerr, err := memFs.Create(workspacebackend.InstanceName(workspace, projectId))
	if err != nil {
		return err
	}

	args := workspacebackend.ExecArgs{
		User: "root",
		Command: []string{
			"bash",
			"-xue",
			"-o",
			"pipefail",
			"-c",
			hookPath,
		},
		Environment: hook.Environment,
		WorkDir:     sdk.SdkHooksDir(hook.Sdk.Name),
	}

	args.Stdin = nil
	args.Stdin = outerr
	args.Stdout = outerr

	done, err := h.backend.Exec(ctx, workspace, &args)
	hookLog, _ := afero.ReadFile(memFs, outerr.Name())
	st := task.State()
	st.Lock()
	if err != nil {
		task.Errorf(string(hookLog))
	} else {
		task.Logf(string(hookLog))
	}
	st.Unlock()
	<-done

	return err
}
