package workspace

import (
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

type Workspace interface {
	Launch() error
}

type LXDWorkspace struct {
	Name string `yaml:"name"`
	Base string `yaml:"base"`
}

func NewWorkspace(fs afero.Fs, filepath string) (Workspace, error) {
	var ws LXDWorkspace
	buf, err := afero.ReadFile(fs, filepath)

	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(buf, &ws); err != nil {
		return nil, err
	}

	return &ws, nil
}

func (w *LXDWorkspace) Launch() error {
	return nil
}
