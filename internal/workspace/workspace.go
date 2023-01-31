package workspace

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"

	util "github.com/canonical/workspace/internal"
	store "github.com/canonical/workspace/internal/fakestore"
	srv "github.com/canonical/workspace/internal/server"
	"github.com/spf13/afero"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

type Workspace interface {
	Launch(client store.StoreClient) error
}

type SDK struct {
	Channel string `yaml:"channel"`
}

type WorkspaceInstance struct {
	Name string          `yaml:"name"`
	Base string          `yaml:"base"`
	SDKs map[string]*SDK `yaml:"sdks"`

	server srv.WorkspaceServer
	fs     afero.Fs
}

type WorkspaceFile struct {
	Name string
	File fs.FileInfo
}

var SupportedBases = []string{"ubuntu@20.04", "ubuntu@22.04"}
var validName = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
var validChannel = regexp.MustCompile(`^(?P<track>[a-zA-Z0-9\.-]+)/(?P<risk>(stable|candidate|beta|edge))$`)

func NewWorkspace(server srv.WorkspaceServer, fs afero.Fs, wsFile WorkspaceFile) (Workspace, error) {
	var ws = WorkspaceInstance{server: server, fs: fs}
	buf, err := afero.ReadFile(fs, wsFile.File.Name())

	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(buf, &ws); err != nil {
		return nil, err
	}

	if !validName.MatchString(ws.Name) {
		return nil, fmt.Errorf("a workspace's name must: (1) start with a letter, (2) include only lower case alpha-numeric or an underscore symbol(s)")
	}

	if !slices.Contains(SupportedBases, ws.Base) {
		return nil, fmt.Errorf("unsupported base: %s", ws.Base)
	}

	if ws.Name != wsFile.Name {
		return nil, fmt.Errorf("the %s's file must be named as .workspace.%s.yaml (now: %s)", ws.Name, ws.Name, wsFile.File.Name())
	}

	for i, k := range ws.SDKs {
		if matches := validChannel.FindStringSubmatch(k.Channel); matches != nil {
			track := matches[validChannel.SubexpIndex("track")]
			risk := matches[validChannel.SubexpIndex("risk")]
			if risk != "stable" {
				ws.SDKs[i].Channel = fmt.Sprintf("%s/stable", track)
				fmt.Printf("Only stable risk levels are supported. Switching to %s for \"%s\"\n", ws.SDKs[i].Channel, i)
			}
		} else {
			return nil, fmt.Errorf("unsupported channel %s for \"%s\"", k.Channel, i)
		}
	}

	return &ws, nil
}

func (w *WorkspaceInstance) Launch(client store.StoreClient) error {
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
		fmt.Printf("Setting up \"%s\" SDK revision %d from %s...\n", sdkName, blob.Revision, sdk.Channel)

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
		/* Note: the following command requires ~ tar >= 1.29 due to --one-top-level */
		done, err := w.server.Exec(w.Name, "root", []string{
			"tar",
			"--extract",
			"--file",
			sdkMount["path"],
			"--one-top-level=" + filepath.Join(util.WorkspaceSdksDir, sdkName),
			"--no-same-owner",
		})

		/* LXD will close this channel */
		<-done

		/* Make sure the SDK file will be unmounted once installed into the workspace */
		delete(devices, sdkName)
		w.server.UpdateWorkspaceDevices(w.Name, devices)

		if err != nil {
			fmt.Printf("could not install \"%s\": %v", sdkName, err)
		}

		/* Run lifecycle hooks */
	}

	fmt.Printf("Workspace \"%s\" started.\n", w.Name)

	return nil
}

func (w *WorkspaceInstance) Start() error {
	return w.server.SetWorkspaceState(w.Name, "start")
}
