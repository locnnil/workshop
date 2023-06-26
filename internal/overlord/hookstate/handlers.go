package hookstate

import (
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

	blob, err := SdkSetup(task)
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
			filepath.Join(workspacebackend.SdkHooksPath(blob.Name), hook.Type()),
		},
		WorkDir: workspacebackend.SdkHooksPath(blob.Name),
		Stdin:   nil,
		Stdout:  outerr,
		Stderr:  outerr}

	done, err := h.backend.Exec(ctx, workspace, &args)
	hookLog, _ := afero.ReadFile(memFs, outerr.Name())
	if err != nil {
		st.Lock()
		task.Logf(string(hookLog))
		st.Unlock()
		return err
	}

	<-done

	return nil
}
