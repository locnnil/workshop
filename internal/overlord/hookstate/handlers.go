package hookstate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/canonical/x-go/strutil"
	"gopkg.in/tomb.v2"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/overlord/handlersetup"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
)

const (
	hookRawOutputBufferSize = 1024 * 1024 * 10
	hookTaskLogSize         = 1024 * 64
)

func (h *HookManager) doRunHook(task *state.Task, tomb *tomb.Tomb) error {
	user, prj, w, err := handlersetup.UserProjectWorkshop(task)
	if err != nil {
		return err
	}

	ctx, cancel := handlersetup.BackendContext(tomb, user, prj.ProjectId)
	defer cancel()

	var hook HookSetup
	st := task.State()
	st.Lock()
	err = task.Get("hook-setup", &hook)
	st.Unlock()
	if err != nil {
		return err
	}

	hookPath := sdk.SdkHookPath(hook.Sdk, hook.Type())

	execArgs := &workshop.ExecArgs{
		UserId:  0,
		GroupId: 0,
		Command: []string{
			"bash",
			"-eo",
			"pipefail",
			hookPath,
		},
		Environment: map[string]string{
			"SDK": sdk.SdkDir(hook.Sdk),
		},
		WorkDir: sdk.SdkHooksDir(hook.Sdk),
		Timeout: hook.Timeout,
	}

	if hook.HookType == SaveState || hook.HookType == RestoreState {
		what := workshop.StateStorageDir(prj.ProjectId, w)
		where := dirs.WorkshopStateDir

		usr, err := osutil.UserLookup(user)
		if err != nil {
			return err
		}
		uid, gid, err := osutil.UidGid(usr)
		if err != nil {
			return err
		}
		sdkStateDir := filepath.Join(what, "sdk", hook.Sdk)
		if err := osutil.MkdirAllChown(sdkStateDir, 0755, uid, gid); err != nil {
			return err
		}

		mount := workshop.Mount{
			Name:  workshop.ConfigStateStorageDevice,
			Type:  workshop.HostWorkshop,
			What:  what,
			Where: dirs.WorkshopStateDir,
		}
		if err := h.backend.AddWorkshopMount(ctx, w, mount); err != nil {
			return fmt.Errorf("cannot run hook %q for SDK %q: %w", hook.Type(), hook.Sdk, err)
		}
		defer func() {
			if err1 := h.backend.RemoveWorkshopMount(ctx, w, mount.Name); err1 != nil {
				logger.Noticef("RunHook on Do: Cannot unmount state storage %q: %v", mount.What, err1)
			}
		}()

		execArgs.Environment["SDK_STATE_DIR"] = filepath.Join(where, "sdk", hook.Sdk)
	}

	switch hook.HookType {
	case SetupProject:
		execArgs.Command = []string{
			"sudo",
			"-u",
			"#" + workshop.User.Uid,
			"-g",
			"#" + workshop.User.Gid,
			"--preserve-env",
			"--",
			"bash",
			"-elo",
			"pipefail",
			hookPath,
		}

		uid, gid, err := osutil.UidGid(&workshop.User)
		if err != nil {
			return err
		}
		execArgs.UserId = int(uid)
		execArgs.GroupId = int(gid)

		execArgs.WorkDir = workshop.WorkshopProjectPath

		execArgs.Environment["HOME"] = workshop.User.HomeDir
		xdgRuntimeDir := filepath.Join(dirs.XdgRuntimeDirBase, workshop.User.Uid)
		execArgs.Environment["XDG_RUNTIME_DIR"] = xdgRuntimeDir
		execArgs.Environment["DBUS_SESSION_BUS_ADDRESS"] = "unix:path=" + filepath.Join(xdgRuntimeDir, "bus")
		return h.executeHook(ctx, task, w, &hook, execArgs)
	default:
		return h.executeHook(ctx, task, w, &hook, execArgs)
	}
}

type HookLogKey string

type HookLog struct {
	l   sync.Mutex
	buf *strutil.LimitedBuffer
}

func NewHookLog() *HookLog {
	return &HookLog{
		buf: strutil.NewLimitedBuffer(-1, hookRawOutputBufferSize),
	}
}

func (h *HookLog) Output() *bytes.Buffer {
	h.l.Lock()
	defer h.l.Unlock()

	return bytes.NewBuffer(bytes.Clone(h.buf.Bytes()))
}

func (h *HookLog) taskLog() []byte {
	h.l.Lock()
	defer h.l.Unlock()

	// 10 lines per task log to be stored in the state as a sensible default
	// (see task.Logf).
	return bytes.Clone(strutil.TruncateOutput(h.buf.Bytes(), 10, hookTaskLogSize))
}

func (h *HookLog) Write(buf []byte) (n int, err error) {
	h.l.Lock()
	defer h.l.Unlock()

	return h.buf.Write(buf)
}

func (h *HookLog) flush() {
	h.l.Lock()
	defer h.l.Unlock()

	buf := h.buf.Bytes()
	if len(buf) > 0 && buf[len(buf)-1] != '\n' {
		_, _ = h.buf.Write([]byte{'\n'})
	}
}

func (h *HookManager) executeHook(ctx context.Context, task *state.Task, w string, hook *HookSetup, execArgs *workshop.ExecArgs) error {
	hookPath := sdk.SdkHookPath(hook.Sdk, hook.Type())

	wsFs, err := h.backend.WorkshopFs(ctx, w)
	if err != nil {
		return err
	}

	info, err := wsFs.Stat(hookPath)
	wsFs.Close()
	if errors.Is(err, os.ErrNotExist) || !info.Mode().IsRegular() {
		logger.Debugf("%q SDK does not provide %q hook", hook.Sdk, hook.Type())
		return nil
	}

	hookCtx, err := createHookContext(task, h.repository, hook)
	if err != nil {
		return err
	}

	contextID := hookCtx.ID()
	h.contextsMutex.Lock()
	h.contexts[contextID] = hookCtx
	h.contextsMutex.Unlock()

	defer func() {
		h.contextsMutex.Lock()
		delete(h.contexts, contextID)
		h.contextsMutex.Unlock()
	}()

	if err := hookCtx.Handler().Before(); err != nil {
		return err
	}

	hookOutErr := NewHookLog()

	st := task.State()
	st.Lock()
	st.Cache(HookLogKey(task.ID()), hookOutErr)
	st.Unlock()

	execArgs.Environment["WORKSHOP_COOKIE"] = hookCtx.ID()
	args := workshop.Execution{
		ExecArgs: *execArgs,
		ExecControls: workshop.ExecControls{
			Stdin:  nil,
			Stdout: hookOutErr,
			Stderr: hookOutErr,
		},
	}

	exectx, err := h.backend.Exec(ctx, w, &args)
	// Handle errors that are unrelated to the command, for example, LXD-related
	// issues. An error here means the execution has not started at all.
	if err != nil {
		return err
	}

	err = exectx.WaitExecution(ctx)

	hookOutErr.flush()
	taskLog := hookOutErr.taskLog()

	if len(taskLog) > 0 {
		st.Lock()
		task.Logf("%s", string(taskLog))
		st.Unlock()
	}

	// Handle the command execution errors; all the errors that are related to
	// the backend that was executing the command should have been handled above
	// already.
	if err != nil {
		ignore, handlerError := hookCtx.Handler().Error(err)
		if handlerError != nil {
			return handlerError
		}
		if ignore {
			return nil
		}
		return err
	}

	if err = hookCtx.Handler().Done(); err != nil {
		return err
	}

	hookCtx.Lock()
	defer hookCtx.Unlock()
	if err = hookCtx.Done(); err != nil {
		return err
	}

	return err
}

func (h *HookManager) doHookCleanup(task *state.Task, tomb *tomb.Tomb) error {
	h.state.Lock()
	h.state.Cache(HookLogKey(task.ID()), nil)
	logger.Debugf("On HookManager.doHookCleanup: cleaned up logs for task %s", task.ID())
	h.state.Unlock()
	return nil
}

func createHookContext(task *state.Task, repo *repository, hook *HookSetup) (*Context, error) {
	hookCtx, err := NewContext(task, task.State(), hook, nil, "")
	if err != nil {
		return nil, err
	}

	handlers := repo.generateHandlers(hookCtx)
	handlersCount := len(handlers)
	if handlersCount == 0 {
		return nil, fmt.Errorf("internal error: no registered handlers for hook %q", hook.HookType)
	}
	if handlersCount > 1 {
		return nil, fmt.Errorf("internal error: %d handlers registered for hook %q, expected 1", handlersCount, hook.HookType)
	}
	hookCtx.handler = handlers[0]
	return hookCtx, nil
}
