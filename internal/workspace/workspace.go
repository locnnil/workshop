package workspace

import (
	srv "github.com/canonical/workspace/internal/server"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

type Workspace interface {
	Launch() error
}

type LxdWorkspace struct {
	Name string `yaml:"name"`
	Base string `yaml:"base"`

	server srv.Server
}

func NewWorkspace(server srv.Server, fs afero.Fs, filepath string) (Workspace, error) {
	var ws = LxdWorkspace{server: server}
	buf, err := afero.ReadFile(fs, filepath)

	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(buf, &ws); err != nil {
		return nil, err
	}

	return &ws, nil
}

func (w *LxdWorkspace) Launch() error {
	if err := w.server.LaunchWorkspaceInstance(w.Name, w.Base); err != nil {
		return err
	}

	return nil
}
