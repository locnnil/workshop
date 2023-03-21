package projectstate

import (
	"errors"
	"fmt"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/spf13/afero"
	"gopkg.in/tomb.v2"
)

func (m *ProjectManager) doLoadProject(task *state.Task, tomb *tomb.Tomb) error {
	st := task.State()
	st.Lock()
	defer st.Unlock()

	var projectDir string

	err := task.Get("project-directory", &projectDir)
	if err != nil {
		return err
	}

	project, err := LoadProject(m.server, m.fs, projectDir)
	if err != nil {
		return err
	}

	change := task.Change()
	projectKey := ProjectKey{
		Path:      project.ProjectDirectory(),
		ProjectId: project.ProjectId(),
	}

	change.Set("project-key", projectKey)

	return nil
}

func (m *ProjectManager) doLoadOrCreateProject(task *state.Task, tomb *tomb.Tomb) error {
	st := task.State()
	st.Lock()
	defer st.Unlock()

	var projectDir string

	err := task.Get("project-directory", &projectDir)
	if err != nil {
		return err
	}

	project, err := LoadProject(m.server, m.fs, projectDir)

	if errors.Is(err, afero.ErrFileNotFound) {
		project, err = NewProject(m.server, m.fs, projectDir)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	change := task.Change()
	projectKey := ProjectKey{
		Path:      project.ProjectDirectory(),
		ProjectId: project.ProjectId(),
	}

	change.Set("project-key", projectKey)

	return nil
}

func ProjectAndWorkspace(task *state.Task) (*ProjectKey, string, error) {
	st := task.State()
	var project ProjectKey
	var name string

	st.Lock()
	err := task.Change().Get("project-key", &project)
	st.Unlock()

	if err != nil {
		return nil, "", fmt.Errorf("cannot get project for task %q: %v", task.ID(), err)
	}

	st.Lock()
	err = task.Change().Get("workspace", &name)
	st.Unlock()

	if err != nil {
		return nil, "", fmt.Errorf("cannot get workspace for task %q: %v", task.ID(), err)
	}

	return &project, name, nil
}
