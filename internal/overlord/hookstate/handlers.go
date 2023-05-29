package hookstate

import (
	"fmt"
	"path/filepath"

	util "github.com/canonical/workspace/internal"
	. "github.com/canonical/workspace/internal/overlord/sharedstate"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"

	"github.com/spf13/afero"
	"gopkg.in/tomb.v2"
)

func (h *HookManager) doRunHook(task *state.Task, tomb *tomb.Tomb) error {
	prj, workspace, err := ProjectAndWorkspace(task)
	if err != nil {
		return err
	}

	blob, err := SdkSetup(task)
	if err != nil {
		return err
	}

	ctx, cancel := BackendContext(tomb, prj)
	defer cancel()

	st := task.State()
	st.Lock()
	var hook util.WorkspaceHookType
	err = task.Get("hook-setup", &hook)
	st.Unlock()
	if err != nil {
		return err
	}

	/* create a memory out/err to log the hook output into the task's log */
	memFs := afero.NewMemMapFs()
	outerr, err := memFs.Create(util.ToInstanceName(workspace, prj.ProjectId))
	if err != nil {
		return err
	}

	fmt.Printf("Running hook \"%s\" for \"%s\"...\n", hook.String(), blob.Name)

	args := workspacebackend.ExecArgs{
		User: "root",
		Command: []string{
			"bash",
			"-xue",
			"-o",
			"pipefail",
			"-c",
			filepath.Join(util.ToHooksPath(blob.Name), hook.String()),
		},
		WorkDir: util.ToHooksPath(blob.Name),
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
