package server

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	util "github.com/canonical/workspace/internal"

	"github.com/gorilla/websocket"
	lxd "github.com/lxc/lxd/client"

	"github.com/lxc/lxd/shared/api"
	"github.com/lxc/lxd/shared/termios"
	"github.com/spf13/afero"
)

type ErrExec struct {
	Status int
}

func (e *ErrExec) Error() string {
	return fmt.Sprintf("command failed with an error code (%d)", e.Status)
}

type WorkspaceDevice struct {
	Name       string
	Properties map[string]string
}

type WorkspaceConfigValue struct {
	Name  string
	Value string
}

type WorkspaceProps struct {
	Name    string
	Devices map[string]map[string]string
	Config  map[string]string

	state  util.WorkspaceState
	reason util.WorkspaceStateReason
}

type ExecArgs struct {
	User    string
	Command []string
	WorkDir string
	Stdin   io.ReadCloser
	Stdout  io.WriteCloser
	Stderr  io.WriteCloser
}

type WorkspaceConfigFilter func(config map[string]string) bool
type WorkspaceDeviceFilter func(devices map[string]map[string]string) bool

func NewWorkspaceConfigFilter(key string, value string) WorkspaceConfigFilter {
	return func(config map[string]string) bool {
		return config[key] == value
	}
}

func EveryWorkspace() WorkspaceConfigFilter {
	return func(config map[string]string) bool {
		return true
	}
}

type WorkspaceServer interface {
	LaunchWorkspaceInstance(name, base, project_id string) error
	DeleteWorkspaceInstance(name, project_id string) error
	SetWorkspaceState(name, action, project_id string) error

	AddWorkspaceDevice(name, project_id string, props WorkspaceDevice) error
	RemoveWorkspaceDevice(name, project_id, device string) error

	AddWorkspaceConfig(name, project_id string, item *WorkspaceConfigValue) error
	AddWorkspacesConfig(filter WorkspaceConfigFilter, item *WorkspaceConfigValue) error
	RemoveWorkspaceConfig(name, project_id string, key string) error

	GetWorkspace(name, project_id string) (*WorkspaceProps, error)
	GetWorkspacesByConfig(filter WorkspaceConfigFilter) ([]*WorkspaceProps, error)
	GetWorkspacesByDevices(filter WorkspaceDeviceFilter) (map[string]*WorkspaceProps, error)

	Exec(name, project_id string, args *ExecArgs) (chan bool, error)
}

type LxdServer struct {
	lxd.InstanceServer
	Fs afero.Fs
}

const LXDSock = "/var/snap/lxd/common/lxd/unix.socket"

var ConnectSimpleStreams = lxd.ConnectSimpleStreams

func (s *LxdServer) connect() (lxd.InstanceServer, error) {
	project, err := GetLXDProjectName()
	if err != nil {
		return nil, err
	}
	if ok, err := afero.Exists(s.Fs, LXDSock); err != nil {
		return nil, err
	} else if ok {
		if srv, err := lxd.ConnectLXDUnix(LXDSock, nil); err != nil {
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

func NewServer(fs afero.Fs) (WorkspaceServer, error) {
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

func (s *LxdServer) LaunchWorkspaceInstance(name, base, project_id string) error {
	var err error
	var imageSrv lxd.ImageServer
	var image *api.Image

	/* Skip if the instance exists already */
	if _, _, err := s.getLxdInstance(name, project_id); err == nil {
		return fmt.Errorf("workspace \"%s\" already exists", name)
	}

	/* Check if we have the base image stored locally */
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

	err = s.launchInstance(name, project_id, &imageSrv, image)
	if err != nil {
		return err
	}

	return nil
}

func (s *LxdServer) launchInstance(name, project_id string, imageServer *lxd.ImageServer, image *api.Image) error {
	projectName, err := GetLXDProjectName()
	if err != nil {
		return err
	}

	req := api.InstancesPost{
		InstancePut: api.InstancePut{
			Devices: map[string]map[string]string{
				"root":              {"type": "disk", "pool": "default", "path": "/"},
				"workspace.network": {"type": "nic", "network": "lxdbr0", "name": "eth0"},
			},
			Config: map[string]string{
				"raw.idmap":                 fmt.Sprint("uid ", os.Getuid(), " 1000\ngid ", os.Getgid(), " 1000"),
				"security.nesting":          "true",
				"user.workspace.project-id": project_id,
			},
		},
		Name: util.ToInstanceName(name, project_id),
		Type: api.InstanceType("container"),
		Source: api.InstanceSource{
			Type:        "image",
			Fingerprint: image.Fingerprint,
			Project:     projectName,
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

func (s *LxdServer) SetWorkspaceState(name, project_id, action string) error {
	inst, etag, err := s.getLxdInstance(name, project_id)
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

	op, err := s.UpdateInstanceState(inst.Name, req, etag)
	if err != nil {
		return err
	}

	return op.Wait()
}

func (s *LxdServer) AddWorkspaceConfig(name, project_id string, item *WorkspaceConfigValue) error {
	inst, etag, err := s.getLxdInstance(name, project_id)
	if err != nil {
		return err
	}
	inst.Config[item.Name] = item.Value
	op, _ := s.UpdateInstance(inst.Name, inst.InstancePut, etag)

	return op.Wait()
}

func (s *LxdServer) RemoveWorkspaceConfig(name, project_id string, key string) error {
	inst, etag, err := s.getLxdInstance(name, project_id)
	if err != nil {
		return err
	}

	delete(inst.Config, key)
	op, _ := s.UpdateInstance(inst.Name, inst.InstancePut, etag)

	return op.Wait()
}

func (s *LxdServer) getLxdInstance(name, project_id string) (instance *api.Instance, ETag string, err error) {
	instanceName := util.ToInstanceName(name, project_id)
	return s.GetInstance(instanceName)
}

func (s *LxdServer) AddWorkspaceDevice(name, project_id string, device WorkspaceDevice) error {
	inst, etag, err := s.getLxdInstance(name, project_id)
	if err != nil {
		return err
	}
	inst.Devices[device.Name] = device.Properties
	op, _ := s.UpdateInstance(inst.Name, inst.InstancePut, etag)

	return op.Wait()
}

func (s *LxdServer) RemoveWorkspaceDevice(name, project_id, device string) error {
	inst, etag, err := s.getLxdInstance(name, project_id)
	if err != nil {
		return err
	}

	delete(inst.Devices, device)
	op, _ := s.UpdateInstance(inst.Name, inst.InstancePut, etag)

	return op.Wait()
}

func (s *LxdServer) Exec(name, project_id string, args *ExecArgs) (chan bool, error) {
	req := api.InstanceExecPost{
		Command: args.Command, WaitForWS: true,
		User: 0, Group: 0, Cwd: args.WorkDir,
		Interactive: false,
	}

	done := make(chan bool)

	arg := lxd.InstanceExecArgs{
		Stdin: args.Stdin, Stdout: args.Stdout, Stderr: args.Stderr,
		Control: SignalHandler, DataDone: done,
	}

	if op, err := s.ExecInstance(util.ToInstanceName(name, project_id), req, &arg); err != nil {
		return done, err
	} else if err := op.Wait(); err != nil {
		return done, err
	} else if status := int(op.Get().Metadata["return"].(float64)); status != 0 {
		return done, &ErrExec{Status: status}
	}

	return done, nil
}

func (s *LxdServer) AddWorkspacesConfig(filter WorkspaceConfigFilter, item *WorkspaceConfigValue) error {
	inst, err := s.GetInstances(api.InstanceTypeContainer)
	if err != nil {
		return err
	}

	for _, i := range inst {
		if filter(i.Config) {
			i.Config[item.Name] = item.Value
			s.UpdateInstance(i.Name, i.InstancePut, "")
		}
	}

	return nil
}

func (s *LxdServer) GetWorkspace(name, project_id string) (*WorkspaceProps, error) {
	inst, _, err := s.GetInstance(util.ToInstanceName(name, project_id))
	if err != nil {
		return nil, err
	}
	return &WorkspaceProps{
		Name:    name,
		state:   fromLxdToWorkspaceState(inst.StatusCode),
		Devices: inst.Devices,
		Config:  inst.Config,
	}, nil
}

func (s *LxdServer) GetWorkspacesByConfig(filter WorkspaceConfigFilter) ([]*WorkspaceProps, error) {
	instances, err := s.GetInstances(api.InstanceTypeContainer)
	if err != nil {
		return nil, err
	}
	var ws []*WorkspaceProps = make([]*WorkspaceProps, 0, len(instances))
	for _, i := range instances {
		if filter(i.Config) {
			ws = append(ws, &WorkspaceProps{
				Name:    util.ToWorkspaceName(i.Name),
				state:   fromLxdToWorkspaceState(i.StatusCode),
				Devices: i.Devices,
				Config:  i.Config,
			})
		}
	}

	return ws, nil
}

func (s *LxdServer) GetWorkspacesByDevices(filter WorkspaceDeviceFilter) (map[string]*WorkspaceProps, error) {
	instances, err := s.GetInstances(api.InstanceTypeContainer)
	if err != nil {
		return nil, err
	}
	var ws map[string]*WorkspaceProps = make(map[string]*WorkspaceProps, len(instances))
	for _, i := range instances {
		if filter(i.Devices) {
			name := util.ToWorkspaceName(i.Name)
			ws[name] = &WorkspaceProps{
				Name:    name,
				state:   fromLxdToWorkspaceState(i.StatusCode),
				Devices: i.Devices,
				Config:  i.Config,
			}

		}
	}

	return ws, nil
}

func (s *LxdServer) DeleteWorkspaceInstance(name, project_id string) error {
	inst, _, err := s.getLxdInstance(name, project_id)
	if err != nil {
		return err
	}

	if inst.StatusCode != 0 && inst.StatusCode != api.Stopped {
		req := api.InstanceStatePut{
			Action:  "stop",
			Timeout: -1,
			Force:   true,
		}

		op, err := s.UpdateInstanceState(util.ToInstanceName(name, project_id), req, "")
		if err != nil {
			return err
		}

		err = op.Wait()
		if err != nil {
			return fmt.Errorf("stopping the instance failed: %v", err)
		}
	}

	op, err := s.DeleteInstance(util.ToInstanceName(name, project_id))
	if err != nil {
		return err
	}

	return op.Wait()
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

func fromLxdToWorkspaceState(lxdStatus api.StatusCode) util.WorkspaceState {
	var state util.WorkspaceState
	switch lxdStatus {
	case api.Running, api.Ready:
		state = util.Ready
	case api.Stopped:
		state = util.Stopped
	default:
		state = util.Pending
	}
	return state
}

func (w *WorkspaceProps) State() util.WorkspaceState {
	return w.state
}

func (w *WorkspaceProps) Reason() util.WorkspaceStateReason {
	return w.reason
}

func (w *WorkspaceProps) SetState(s util.WorkspaceState, r util.WorkspaceStateReason) {
	w.state, w.reason = s, r
}
