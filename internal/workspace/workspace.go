package workspace

import (
	"fmt"

	util "github.com/canonical/workspace/internal"
	store "github.com/canonical/workspace/internal/fakestore"
	srv "github.com/canonical/workspace/internal/server"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

type Workspace interface {
	Launch(client store.StoreClient) error
}

type SDK struct {
	Channel string `yaml:"channel"`
}

type LxdWorkspace struct {
	Name string         `yaml:"name"`
	Base string         `yaml:"base"`
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

func (w *LxdWorkspace) Launch(client store.StoreClient) error {
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
		filename, err := client.FetchSDK(name, sdk.Channel, util.SdksDir)
		if err != nil {
			return err
		}

		/* Bind-mount the SDK to the workspace */
		devices, err := w.server.GetWorkspaceDevices()
		if err != nil {
			return err
		}
		sdkMount := map[string]string{"type": "disk", "source": filename, "path": "/root"}

		devices[name] = sdkMount
		err = w.server.UpdateWorkspaceDevices(devices)
		if err != nil {
			return err
		}
		/* Unpack the SDK to the desired location in the workspace */

		/* Make sure the SDK file will be unmounted onces installed into the workspace */
		delete(devices, name)
		w.server.UpdateWorkspaceDevices(devices)
	}

	fmt.Printf("Workspace \"%s\" started.\n", w.Name)

	return nil
}

func (w *LxdWorkspace) Start() error {
	return w.server.SetWorkspaceState(w.Name, "start")
}
