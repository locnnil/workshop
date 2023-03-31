package hookstate

import (
	"github.com/canonical/workspace/internal/overlord/state"
	"gopkg.in/tomb.v2"
)

func (h *HookManager) doRunHook(task *state.Task, tomb *tomb.Tomb) error {
	// project, workspace, err := ProjectAndWorkspace(task)
	// if err != nil {
	// 	return err
	// }

	// blob, err := SdkSetup(task)
	// if err != nil {
	// 	return err
	// }

	// st := task.State()
	// st.Lock()
	// defer st.Unlock()

	// /* create a memory out/err to log the hook output into the task's log */
	// memFs := afero.NewMemMapFs()
	// outerr, err := memFs.Create(util.ToInstanceName(workspace, project.ProjectId))
	// if err != nil {
	// 	return err
	// }

	// args := srv.ExecArgs{
	// 	User: "root",
	// 	Command: []string{
	// 		"bash",
	// 		"-c",
	// 		"setup-base",
	// 	},
	// 	WorkDir: util.ToHooksPath(blob.Name),
	// 	Stdin:   nil,
	// 	Stdout:  outerr,
	// 	Stderr:  outerr}

	// done, err := h.backend.Exec(workspace, project.ProjectId, &args)
	// if err != nil {
	// 	return err
	// }

	// <-done

	// task.Logf(outerr)

	return nil
}
