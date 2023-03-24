package projectstate

import (
	"errors"

	"github.com/canonical/workspace/internal/overlord/state"
	srv "github.com/canonical/workspace/internal/server"
	"github.com/spf13/afero"
)

type ProjectManager struct {
	server srv.WorkspaceServer
	fs     afero.Fs
}

func NewProjectManager(runner *state.TaskRunner, server srv.WorkspaceServer) *ProjectManager {
	manager := &ProjectManager{
		server: server,
		fs:     afero.NewOsFs(),
	}

	return manager
}

func (m *ProjectManager) LoadOrCreateProject(projectDir string) (*ProjectKey, error) {
	project, err := LoadProject(m.server, m.fs, projectDir)

	if errors.Is(err, afero.ErrFileNotFound) {
		project, err = NewProject(m.server, m.fs, projectDir)
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

func (w *ProjectManager) Ensure() error {
	return nil
}
