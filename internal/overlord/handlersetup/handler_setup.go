package handlersetup

import (
	"context"
	"errors"
	"fmt"

	"gopkg.in/tomb.v2"

	"github.com/canonical/workshop/internal/overlord/conflict"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshop"
)

// OnDo helps to decide whether:
// 1. The task needs to be put on Wait (wait-on-error for refresh).
//
// 2. The error needs to be reported but safely ingored (ContextCancelled can
// happen if a user cancells or something gets interrupted during the execution
// due to abortion, e.g. a running hook is called off because their change was
// aborted.
//
// 3. The error needs to be reported as is which will abort the change (or the
// affected lanes).
func OnDo(handler state.HandlerFunc) state.HandlerFunc {
	return func(task *state.Task, tomb *tomb.Tomb) error {
		err := handler(task, tomb)
		if _, ok := err.(*state.Retry); ok {
			return err
		}

		st := task.State()
		st.Lock()
		defer st.Unlock()

		switch {
		case errors.Is(err, context.Canceled):
			task.Logf("The task execution was cancelled")
			// the context cancellation here means the change was aborted and
			// the undo logic chain started. we don't report the context
			// cancellation as error here as it is an expected interruption
			return nil
		case err != nil:
			change := task.Change()
			if change.Kind() == "refresh" {
				var setup conflict.RefreshSetup
				if errKey := change.Get("refresh-setup", &setup); errKey != nil {
					return errKey
				}

				mode, moderr := conflict.ParseRefreshMode(setup.Mode)
				if moderr != nil {
					return fmt.Errorf("internal error: unkown refresh mode: %s", setup.Mode)
				}

				if mode == conflict.RefreshWaitOnError {
					task.Logf("Setting the task to wait until the refresh is either aborted or continued...")
					task.Errorf(err.Error())
					return &state.Wait{
						WaitedStatus: state.DoingStatus,
						Reason:       fmt.Sprintf("wait on error: %v", err),
					}
				}

			}

			return err
		}

		return nil
	}
}

func Workshop(task *state.Task) (string, error) {
	var w string
	err := task.Get("workshop", &w)
	return w, err
}

func Sdk(task *state.Task) (string, error) {
	var s string
	err := task.Get("sdk", &s)
	return s, err
}

func UserProjectWorkshop(task *state.Task) (string, *workshop.Project, string, error) {
	st := task.State()
	var prj workshop.Project
	var name string
	var user string

	st.Lock()
	id := task.ID()
	err := task.Get("project", &prj)
	st.Unlock()

	if err != nil {
		return "", nil, "", fmt.Errorf("cannot get project, task %q: %v", id, err)
	}

	st.Lock()
	err = task.Get("workshop", &name)
	st.Unlock()

	if err != nil {
		return "", nil, "", fmt.Errorf("cannot get workshop, task %q: %v", id, err)
	}

	st.Lock()
	changeId := task.Change().ID()
	err = task.Change().Get("user", &user)
	st.Unlock()

	if err != nil {
		return "", nil, "", fmt.Errorf("cannot get user name, change %q: %v", changeId, err)
	}

	return user, &prj, name, nil
}

func BackendContext(tomb *tomb.Tomb, user string, projectId string) (context.Context, context.CancelFunc) {
	ctx := tomb.Context(context.Background())
	ctxProject := context.WithValue(ctx, workshop.ContextProjectId, projectId)
	ctxUser := context.WithValue(ctxProject, workshop.ContextUser, user)
	ctxCancel, cancel := context.WithCancel(ctxUser)
	return ctxCancel, cancel
}

// InjectTasks makes all the halt tasks of the mainTask wait for extraTasks;
// extraTasks join the same lane and change as the mainTask.
func InjectTasks(mainTask *state.Task, extraTasks *state.TaskSet) {
	lanes := mainTask.Lanes()
	if len(lanes) == 1 && lanes[0] == 0 {
		lanes = nil
	}
	for _, l := range lanes {
		extraTasks.JoinLane(l)
	}

	chg := mainTask.Change()
	// Change shouldn't normally be nil, except for cases where
	// this helper is used before tasks are added to a change.
	if chg != nil {
		chg.AddAll(extraTasks)
	}

	// make all halt tasks of the mainTask wait on extraTasks
	ht := mainTask.HaltTasks()
	for _, t := range ht {
		t.WaitAll(extraTasks)
	}

	// make the extra tasks wait for main task
	extraTasks.WaitFor(mainTask)
}
