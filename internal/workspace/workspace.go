package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	util "github.com/canonical/workspace/internal"
	srv "github.com/canonical/workspace/internal/server"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

type Workspace interface {
	Launch() error
}

type SDK struct {
	Channel string `yaml:"channel"`
}

type LxdWorkspace struct {
	Name string `yaml:"name"`
	Base string `yaml:"base"`

	SDKs map[string]SDK `yaml:"sdks"`

	server srv.Server
	fs     afero.Fs
}

func NewWorkspace(server srv.Server, fs afero.Fs, filepath string) (Workspace, error) {
	var ws = LxdWorkspace{server: server, fs: fs}
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
	var err error

	fmt.Printf("Setting up \"%s\" workspace...\n", w.Name)

	if err := w.server.LaunchWorkspaceInstance(w.Name, w.Base); err != nil {
		return err
	}

	if err = w.Start(); err != nil {
		return err
	}

	for name, sdk := range w.SDKs {
		fmt.Printf("Setting up \"%s\" SDK revision from %s.\n", name, sdk.Channel)

		/* Download an SDK */
		w.downloadSDK(name, sdk.Channel)

		/* Bind-mount the SDK to the workspace */
	}

	fmt.Printf("Workspace \"%s\" started.\n", w.Name)

	return nil
}

func (w *LxdWorkspace) Start() error {
	return w.server.SetInstanceState(w.Name, "start")
}

func (w *LxdWorkspace) downloadSDK(name, channel string) error {
	var track, risk string
	if sa := strings.Split(channel, "/"); len(sa) != 2 {
		return fmt.Errorf("%s has an invalid channel %s, must take the form <track>/<risk>", name, channel)
	} else {
		track, risk = sa[0], sa[1]
	}

	file, err := w.fs.OpenFile(filepath.Join(util.SdksDir, fmt.Sprintf("%s_%s_%s.sdk", name, track, risk)), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	defer file.Close()
	return nil
}
