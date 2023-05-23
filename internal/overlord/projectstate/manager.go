package projectstate

import (
	"errors"

	"github.com/canonical/workspace/internal/overlord/state"
	backend "github.com/canonical/workspace/internal/workspacebackend"
	"github.com/spf13/afero"
)

type ProjectManager struct {
	backend backend.WorkspaceBackend
	fs      afero.Fs
}

func NewProjectManager(runner *state.TaskRunner, be backend.WorkspaceBackend) *ProjectManager {
	manager := &ProjectManager{
		backend: be,
		fs:      afero.NewOsFs(),
	}

	return manager
}

func (m *ProjectManager) LoadOrCreateProject(projectDir string) (*ProjectKey, error) {
	project, err := LoadProject(m.backend, m.fs, projectDir)

	if errors.Is(err, afero.ErrFileNotFound) {
		project, err = NewProject(m.backend, m.fs, projectDir)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	projectKey := ProjectKey{
		Path:      project.ProjectDirectory(),
		ProjectId: project.ProjectId(),
	}

	return &projectKey, nil
}

func (m *ProjectManager) LoadProject(projectDir string) (*ProjectKey, error) {
	project, err := LoadProject(m.backend, m.fs, projectDir)

	if err != nil {
		return nil, err
	}

	projectKey := ProjectKey{
		Path:      project.ProjectDirectory(),
		ProjectId: project.ProjectId(),
	}

	return &projectKey, nil
}

func (w *ProjectManager) Ensure() error {
	return nil
}
