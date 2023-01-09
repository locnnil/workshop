package server

import (
	"fmt"
	"strings"

	lxd "github.com/lxc/lxd/client"
	"github.com/lxc/lxd/shared/api"
	"github.com/spf13/afero"

	util "github.com/canonical/workspace/internal"
)

type Server interface {
	LaunchWorkspaceInstance(name, base string) error
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
			return srv.UseProject(SDK_PROJECT_NAME), nil
		}
	} else {
		if srv, err := lxd.ConnectLXDUnix("", nil); err != nil {
			return nil, err
		} else {
			return srv.UseProject(SDK_PROJECT_NAME), nil
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
	imageName := strings.Replace(base, "@", ":", 1)
	var err error
	var image *api.Image
	var op lxd.RemoteOperation

	if _, _, err = s.GetImage(imageName); err == nil {
		return err
	} else {
		imageServer, err := ConnectSimpleStreams("https://cloud-images.ubuntu.com/releases/", nil)
		if err != nil {
			return err
		}

		names := strings.Split(imageName, ":")
		if len(names) <= 1 {
			return fmt.Errorf("cannot find a base image for the workspace")
		}

		alias, _, err := imageServer.GetImageAlias(names[1])
		if err != nil {
			return err
		}

		image, _, err = imageServer.GetImage(alias.Target)
		if err != nil {
			return err
		}

		op, err = s.CopyImage(imageServer, *image, nil)

		if err != nil {
			return err
		}

		_, err = op.AddHandler(func(o api.Operation) {
			//util.IfPrintf(util.IsTTY(syscall.Stdout), "\033[A\r%40s\r", "")

			if o.Metadata["download_progress"] != nil {
				fmt.Printf("Download image: %v", o.Metadata["download_progress"])
			} else if o.Metadata["create_instance_from_image_unpack_progress"] != nil {
				fmt.Printf("%v", o.Metadata["create_instance_from_image_unpack_progress"])
			} else if o.Metadata["fingerprint"] != nil {
				receivedFingerprint := o.Metadata["fingerprint"].(string)
				fmt.Printf("Imported image fingerprint: %v", receivedFingerprint)
			} else if o.Err == "" {
				fmt.Printf("UNEXPECTED: %v", o.Err)
			}

			fmt.Printf("\n")
		})

		if err != nil {
			return err
		}

		if err = util.CancellableWait(op); err != nil {
			return err
		}

	}

	return nil
}
