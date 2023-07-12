package hookstate

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/canonical/workspace/internal/overlord/state"
	. "github.com/canonical/workspace/internal/overlord/statecontext"
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

	return h.executeHook(ctx, task, workspace, prj.ProjectId, &hook)
}

func (h *HookManager) executeHook(ctx context.Context, task *state.Task, workspace, projectId string, hook *HookSetup) error {
	wsFs, err := h.backend.GetWorkspaceFs(ctx, workspace)
	if err != nil {
		return err
	}
	defer wsFs.Close()

	hookPath := filepath.Join(workspacebackend.SdkHooksPath(hook.Sdk.Name), hook.Type())
	info, err := wsFs.Stat(hookPath)
	if errors.Is(err, afero.ErrFileNotFound) || !info.Mode().IsRegular() {
		return nil
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
		WorkDir: workspacebackend.SdkHooksPath(hook.Sdk.Name),
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
