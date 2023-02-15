package workspace

import (
	"fmt"
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

type Project struct {
	Path string
}

type WorkspaceInstance struct {
	Name string          `yaml:"name"`
	Base string          `yaml:"base"`
	SDKs map[string]*SDK `yaml:"sdks"`
	Prj  Project

	server srv.WorkspaceServer
	fs     afero.Fs
}

var SupportedBases = []string{"ubuntu@20.04", "ubuntu@22.04"}
var validName = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)
var validChannel = regexp.MustCompile(`^(?P<track>[a-zA-Z0-9\.-]+)/(?P<risk>(stable|candidate|beta|edge))$`)

func NewWorkspace(server srv.WorkspaceServer, fs afero.Fs, ws srv.WorkspaceFile) (Workspace, error) {
	var inst = WorkspaceInstance{
		Prj:    Project{Path: ws.Project},
		server: server,
		fs:     fs,
	}
	buf, err := afero.ReadFile(fs, filepath.Join(ws.Project, ws.File.Name()))

	if err != nil {
		return nil, err
	}

	if err = yaml.Unmarshal(buf, &inst); err != nil {
		return nil, err
	}

	if !validName.MatchString(inst.Name) {
		return nil, fmt.Errorf("a workspace's name must: (1) start with a letter, (2) include only lower case alpha-numeric or an underscore symbol(s)")
	}

	if !slices.Contains(SupportedBases, inst.Base) {
		return nil, fmt.Errorf("unsupported base: %s", inst.Base)
	}

	if inst.Name != ws.Name {
		return nil, fmt.Errorf("the %s's file must be named as .workspace.%s.yaml (now: %s)", inst.Name, inst.Name, ws.File.Name())
	}

	for i, k := range inst.SDKs {
		if matches := validChannel.FindStringSubmatch(k.Channel); matches != nil {
			track := matches[validChannel.SubexpIndex("track")]
			risk := matches[validChannel.SubexpIndex("risk")]
			if risk != "stable" {
				inst.SDKs[i].Channel = fmt.Sprintf("%s/stable", track)
				fmt.Printf("Only stable risk levels are supported. Switching to %s for \"%s\"\n", inst.SDKs[i].Channel, i)
			}
		} else {
			return nil, fmt.Errorf("unsupported channel %s for \"%s\"", k.Channel, i)
		}
	}

	return &inst, nil
}

func (w *WorkspaceInstance) Launch(client store.StoreClient) error {
	var err error

	fmt.Printf("Setting up \"%s\" workspace...\n", w.Name)

	/* Launch a workspace with the required base */
	if err := w.server.LaunchWorkspaceInstance(w.Name, w.Base); err != nil {
		return err
	}

	/* Configure workspace core properties: (1) project directory */
	var prjMount = srv.WorkspaceDevice{
		Name:       srv.PROJECT_DEVICE_NAME,
		Properties: map[string]string{"type": "disk", "source": w.Prj.Path, "path": "/project"},
	}

	if err = w.server.AddWorkspaceDevice(w.Name, prjMount); err != nil {
		return err
	}

	/* Start the workspace. TODO: make sure that we have it completed before attempting to change the state */
	if err = w.Start(); err != nil {
		return err
	}

	for sdkName, sdk := range w.SDKs {

		/* Download SDK */
		blob, err := client.FetchSDK(sdkName, sdk.Channel, util.SdksDir)
		if err != nil {
			return err
		}
		fmt.Printf("Setting up \"%s\" SDK revision %d from %s...\n", sdkName, blob.Revision, sdk.Channel)

		/* Install SDK */
		err = w.installSDK(blob)
		if err != nil {
			return err
		}

		/* TODO: Run lifecycle hooks */
	}

	fmt.Printf("Workspace \"%s\" started.\n", w.Name)

	return nil
}

func (w *WorkspaceInstance) installSDK(blob store.SDKFile) error {
	/* Bind-mount the SDK to the workspace */
	var sdkMount = srv.WorkspaceDevice{
		Name: blob.Name,
		Properties: map[string]string{"type": "disk", "source": blob.Filename,
			"path": filepath.Join("/root", filepath.Base(blob.Filename))},
	}

	err := w.server.AddWorkspaceDevice(w.Name, sdkMount)
	if err != nil {
		return err
	}

	/* Unpack the SDK to the desired location in the workspace */
	/* Note: the following command requires ~ tar >= 1.29 due to --one-top-level */
	done, err := w.server.Exec(w.Name, "root", []string{
		"tar",
		"--extract",
		"--file",
		sdkMount.Properties["path"],
		"--one-top-level=" + filepath.Join(util.WorkspaceSdksDir, blob.Name),
		"--no-same-owner",
	})

	/* The server will close this channel when exec is finished and no i/o remains outstanding*/
	<-done

	/* Make sure the SDK file will be unmounted once installed into the workspace */
	w.server.RemoveWorkspaceDevice(w.Name, sdkMount.Name)

	if err != nil {
		fmt.Printf("could not install \"%s\": %v", blob.Name, err)
	}
	return nil
}

func (w *WorkspaceInstance) Start() error {
	return w.server.SetWorkspaceState(w.Name, "start")
}
