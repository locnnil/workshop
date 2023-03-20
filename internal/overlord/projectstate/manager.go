package projectstate

import (
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

	runner.AddHandler("load-or-create-project", manager.doLoadOrCreateProject, nil)

	return manager
}

func (w *ProjectManager) Ensure() error {
	return nil
}
