package hookstate

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/canonical/workspace/internal/overlord/state"
	. "github.com/canonical/workspace/internal/overlord/sthelper"
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

	switch hook.HookType {
	case workspacebackend.SetupBase:
		return h.doSetupBase(task, ctx, workspace, prj, &hook)
	case workspacebackend.SaveState:
		return h.doSaveState(task, ctx, workspace, prj, &hook)
	case workspacebackend.RestoreState:
		return h.doRestoreState(task, ctx, workspace, prj, &hook)
	default:
		return fmt.Errorf("unknown hook type %q", hook.HookType.String())
	}
}

func (h *HookManager) doSetupBase(task *state.Task, ctx context.Context, workspace string, prj *workspacebackend.Project, hook *HookSetup) error {
	/* create a memory out/err to log the hook output into the task's log */
	memFs := afero.NewMemMapFs()
	outerr, err := memFs.Create(workspacebackend.InstanceName(workspace, prj.ProjectId))
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
			filepath.Join(workspacebackend.SdkHooksPath(hook.Sdk.Name), hook.Type()),
		},
		WorkDir: workspacebackend.SdkHooksPath(hook.Sdk.Name),
		Stdin:   nil,
		Stdout:  outerr,
		Stderr:  outerr}

	done, err := h.backend.Exec(ctx, workspace, &args)
	hookLog, _ := afero.ReadFile(memFs, outerr.Name())
	if err != nil {
		st := task.State()
		st.Lock()
		task.Logf(string(hookLog))
		st.Unlock()
		return err
	}

	<-done
	return nil
}

func (h *HookManager) doSaveState(task *state.Task, ctx context.Context, workspace string, prj *workspacebackend.Project, hook *HookSetup) error {
	return nil
}

func (h *HookManager) doRestoreState(task *state.Task, ctx context.Context, workspace string, prj *workspacebackend.Project, hook *HookSetup) error {
	return nil
}
