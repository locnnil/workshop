package server

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	util "github.com/canonical/workspace/internal"
	"github.com/gorilla/websocket"
	lxd "github.com/lxc/lxd/client"

	"github.com/lxc/lxd/shared/api"
	"github.com/lxc/lxd/shared/termios"
	"github.com/spf13/afero"
)

type WorkspaceDevices map[string]map[string]string

type Server interface {
	LaunchWorkspaceInstance(name, base string) error
	SetWorkspaceState(name, action string) error
	UpdateWorkspaceDevices(name string, devices WorkspaceDevices) error
	GetWorkspaceDevices(name string) (WorkspaceDevices, error)

	Exec(name, user string, command []string) error
}

type LxdServer struct {
	lxd.InstanceServer
	Fs afero.Fs
}

const LXD_SOCK = "/var/snap/lxd/common/lxd/unix.socket"

var ConnectSimpleStreams = lxd.ConnectSimpleStreams

func (s *LxdServer) connect() (lxd.InstanceServer, error) {
	project, err := GetLXDProjectName()
	if err != nil {
		return nil, err
	}
	if ok, err := afero.Exists(s.Fs, LXD_SOCK); err != nil {
		return nil, err
	} else if ok {
		if srv, err := lxd.ConnectLXDUnix(LXD_SOCK, nil); err != nil {
			return nil, err
		} else {
			return srv.UseProject(project), nil
		}
	} else {
		if srv, err := lxd.ConnectLXDUnix("", nil); err != nil {
			return nil, err
		} else {
			return srv.UseProject(project), nil
		}
	}
}

func NewServer(fs afero.Fs) (Server, error) {
	server := LxdServer{Fs: fs}

	if lxdInst, err := server.connect(); err != nil {
		return nil, err
	} else {
		if err = InitProject(lxdInst); err != nil {
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

	/* Skip if the instance exists already */
	if _, _, err := s.GetInstance(name); err == nil {
		return fmt.Errorf("%s already exists", name)
	}

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
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	// canonicalise the project's pathname to make sure it is unambiguous for
	// all workspaces
	projectPath, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return nil
	}
	req := api.InstancesPost{
		InstancePut: api.InstancePut{
			Devices: map[string]map[string]string{
				"root":              {"type": "disk", "pool": "default", "path": "/"},
				"workspace.project": {"type": "disk", "source": projectPath, "path": "/project"},
				"workspace.network": {"type": "nic", "network": "lxdbr0", "name": "eth0"},
			},
			Config: map[string]string{
				"raw.idmap":        fmt.Sprint("uid ", os.Getuid(), " 1000\ngid ", os.Getgid(), " 1000"),
				"security.nesting": "true",
			},
		},
		Name: name,
		Type: api.InstanceType("container"),
		Source: api.InstanceSource{
			Type:        "image",
			Fingerprint: image.Fingerprint,
		},
	}
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

func (s *LxdServer) SetWorkspaceState(name string, action string) error {
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

func (s *LxdServer) UpdateWorkspaceDevices(name string, devices WorkspaceDevices) error {
	inst, _, err := s.GetInstance(name)
	if err != nil {
		return err
	}

	inst.Devices = devices
	op, _ := s.UpdateInstance(name, inst.InstancePut, "")

	return op.Wait()
}

func (s *LxdServer) GetWorkspaceDevices(name string) (WorkspaceDevices, error) {
	inst, _, err := s.GetInstance(name)
	if err != nil {
		return nil, err
	}

	return inst.Devices, nil
}

func (s *LxdServer) Exec(name, user string, command []string) error {
	req := api.InstanceExecPost{
		Command: command, WaitForWS: true,
		User: 0, Group: 0, Cwd: "/",
		Interactive: false,
	}

	arg := lxd.InstanceExecArgs{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		Control: SignalHandler, DataDone: make(chan bool),
	}

	if op, err := s.ExecInstance(name, req, &arg); err != nil {
		return err
	} else if err := op.Wait(); err != nil {
		return fmt.Errorf("LXD error: (%s)", op.Get().Err)
	} else if int(op.Get().Metadata["return"].(float64)) != 0 {
		return fmt.Errorf("command failed with an error code (%d)", int(op.Get().Metadata["return"].(float64)))
	}

	/* Flush any remaining I/O */
	<-arg.DataDone

	return nil
}

func SignalHandler(control *websocket.Conn) {
	signals := make(chan os.Signal, 10)
	signal.Notify(signals, syscall.SIGINT)

	closeMessage := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	defer control.WriteMessage(websocket.CloseMessage, closeMessage)

	for {
		signal := <-signals

		switch signal {
		case syscall.SIGINT:
			err := control.WriteJSON(api.InstanceExecControl{
				Command: "signal",
				Signal:  int(syscall.SIGINT),
			})
			if err != nil {
				fmt.Printf("Failed to interrupt command execution: %v\n", err)
				return
			}
		}
	}
}
