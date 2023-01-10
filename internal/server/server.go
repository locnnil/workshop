package server

import (
	"fmt"
	"strings"
	"syscall"

	util "github.com/canonical/workspace/internal"
	lxd "github.com/lxc/lxd/client"

	"github.com/lxc/lxd/shared/api"
	"github.com/lxc/lxd/shared/termios"
	"github.com/spf13/afero"
)

type Server interface {
	LaunchWorkspaceInstance(name, base string) error
	SetInstanceState(name, action string) error
}

type LxdServer struct {
	lxd.InstanceServer
	filesystem afero.Fs
}

const LXD_SOCK = "/var/snap/lxd/common/lxd/unix.socket"

var ConnectSimpleStreams = lxd.ConnectSimpleStreams

func (s *LxdServer) connect() (srv lxd.InstanceServer, err error) {
	if ok, err := afero.Exists(s.filesystem, LXD_SOCK); err != nil {
		return nil, err
	} else if ok {
		if srv, err := lxd.ConnectLXDUnix(LXD_SOCK, nil); err != nil {
			return nil, err
		} else {
			return srv.UseProject(WORKSPACE_PROJECT_NAME), nil
		}
	} else {
		if srv, err := lxd.ConnectLXDUnix("", nil); err != nil {
			return nil, err
		} else {
			return srv.UseProject(WORKSPACE_PROJECT_NAME), nil
		}
	}
}

func NewServer(fs afero.Fs) (Server, error) {
	server := LxdServer{filesystem: fs}

	if lxdInst, err := server.connect(); err != nil {
		return nil, err
	} else {
		if err = initProject(lxdInst); err != nil {
			return nil, err
		}
		server.InstanceServer = lxdInst
	}

	return &server, nil
}

func (s *LxdServer) LaunchWorkspaceInstance(name, base string) error {
	var err error
	var imageSrv lxd.ImageServer
	var image *api.Image

	if alias, _, err := s.GetImageAlias(base); err == nil {
		if image, _, err = s.GetImage(alias.Target); err != nil {
			return err
		}
		imageSrv = s
	} else {
		imageSrv, image, err = s.fetchRemoteImage(base)
		if err != nil {
			return err
		}

		defer s.CreateImageAlias(api.ImageAliasesPost{ImageAliasesEntry: api.ImageAliasesEntry{
			Name: base,
			ImageAliasesEntryPut: api.ImageAliasesEntryPut{
				Target: image.Fingerprint,
			},
		}})
	}

	err = s.launchInstance(name, &imageSrv, image)
	if err != nil {
		return err
	}

	return nil
}

func (s *LxdServer) launchInstance(name string, imageServer *lxd.ImageServer, image *api.Image) error {
	req := api.InstancesPost{
		InstancePut: api.InstancePut{
			Devices: map[string]map[string]string{"root": {"type": "disk", "pool": "default", "path": "/"}},
		},
		Name: name,
		Type: api.InstanceType("container"),
		Source: api.InstanceSource{
			Type:        "image",
			Fingerprint: image.Fingerprint,
		},
	}
	fmt.Printf("Setting up \"%s\" workspace...\n", name)
	op, err := s.CreateInstanceFromImage(*imageServer, *image, req)
	if err != nil {
		return nil
	}

	op.AddHandler(ProgressHandler)
	err = util.CancellableWait(op)
	if err != nil {
		return err
	}
	return nil
}

func (s *LxdServer) fetchRemoteImage(base string) (lxd.ImageServer, *api.Image, error) {
	var image *api.Image

	imageServer, err := ConnectSimpleStreams("https://cloud-images.ubuntu.com/releases/", nil)
	if err != nil {
		return nil, nil, err
	}

	names := strings.Split(base, "@")
	if len(names) <= 1 {
		return nil, nil, fmt.Errorf("cannot find a base image for the workspace")
	}

	alias, _, err := imageServer.GetImageAlias(fmt.Sprintf("%s/amd64", names[1]))
	if err != nil {
		return nil, nil, err
	}

	image, _, err = imageServer.GetImage(alias.Target)
	if err != nil {
		return nil, nil, err
	}

	return imageServer, image, nil
}

func ProgressHandler(o api.Operation) {

	if o.Metadata["download_progress"] != nil {
		fmt.Printf("Download image: %v\n", o.Metadata["download_progress"])
	} else if o.Metadata["create_instance_from_image_unpack_progress"] != nil {
		fmt.Printf("%v\n", o.Metadata["create_instance_from_image_unpack_progress"])
	} else if o.Metadata["fingerprint"] != nil {
		receivedFingerprint := o.Metadata["fingerprint"].(string)
		fmt.Printf("Imported image fingerprint: %v\n", receivedFingerprint)
	} else if o.Err == "" {
		fmt.Printf("UNEXPECTED: %v\n", o.Err)
	}

	if termios.IsTerminal(syscall.Stdout) {
		fmt.Print("\033[A\033[2K\r")
	}
}

func (s *LxdServer) SetInstanceState(name string, action string) error {
	inst, etag, err := s.GetInstance(name)
	if err != nil {
		return err
	}

	/* Do nothing if the instance is already in the desired state */
	if (inst.StatusCode == api.Running && action == "start") ||
		(inst.StatusCode == api.Stopped && action == "stop") {
		return nil
	}

	req := api.InstanceStatePut{
		Action:  action,
		Timeout: -1,
		Force:   false,
	}

	op, err := s.UpdateInstanceState(name, req, etag)
	if err != nil {
		return err
	}

	return op.Wait()
}
