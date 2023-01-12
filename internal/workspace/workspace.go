package workspace

import (
	"fmt"
	"path/filepath"

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

	for sdkName, sdk := range w.SDKs {

		/* Download an SDK */
		blob, err := client.FetchSDK(sdkName, sdk.Channel, util.SdksDir)
		if err != nil {
			return err
		}
		fmt.Printf("Setting up \"%s\" SDK revision %d from %s.\n", sdkName, blob.Revision, sdk.Channel)

		/* Bind-mount the SDK to the workspace */
		devices, err := w.server.GetWorkspaceDevices(w.Name)
		if err != nil {
			return err
		}

		sdkMount := map[string]string{"type": "disk", "source": blob.Filename, "path": filepath.Join("/root", filepath.Base(blob.Filename))}

		devices[sdkName] = sdkMount
		err = w.server.UpdateWorkspaceDevices(w.Name, devices)
		if err != nil {
			return err
		}
		/* Unpack the SDK to the desired location in the workspace */
		err = w.server.Exec(w.Name, "root", []string{
			"tar",
			"--extract",
			"--file",
			sdkMount["path"],
			"--one-top-level=" + filepath.Join(util.WorkspaceSdksDir, sdkName),
		})

		/* Make sure the SDK file will be unmounted onces installed into the workspace */
		delete(devices, sdkName)
		w.server.UpdateWorkspaceDevices(w.Name, devices)

		if err != nil {
			return fmt.Errorf("could not install \"%s\": %w", sdkName, err)
		}

		/* Run lifecycle hooks */
	}

	fmt.Printf("Workspace \"%s\" started.\n", w.Name)

	return nil
}

func (w *LxdWorkspace) Start() error {
	return w.server.SetWorkspaceState(w.Name, "start")
}
