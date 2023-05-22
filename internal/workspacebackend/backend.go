package workspacebackend

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	util "github.com/canonical/workspace/internal"
	"github.com/canonical/workspace/internal/logger"

	lxd "github.com/lxc/lxd/client"

	"github.com/lxc/lxd/shared/api"
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

func (w *WorkspaceProps) State() util.WorkspaceState {
	return w.state
}

func (w *WorkspaceProps) Reason() util.WorkspaceStateReason {
	return w.reason
}

func (w *WorkspaceProps) SetState(s util.WorkspaceState, r util.WorkspaceStateReason) {
	w.state, w.reason = s, r
}

type ExecArgs struct {
	User    string
	Command []string
	WorkDir string
	Stdin   io.ReadCloser
	Stdout  io.WriteCloser
	Stderr  io.WriteCloser
}

type LxdBackend struct {
}

const LxdSock = "/var/snap/lxd/common/lxd/unix.socket"

var ConnectSimpleStreams = lxd.ConnectSimpleStreams

type ContextKeyProjectId string

const ContextProjectId = ContextKeyProjectId("project-id")

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

type WorkspaceBackend interface {
	LaunchWorkspace(ctx context.Context, name, base string) error
	DeleteWorkspace(name, project_id string) error
	SetWorkspaceState(name, action, project_id string) error

	AddWorkspaceDevice(name, project_id string, props WorkspaceDevice) error
	RemoveWorkspaceDevice(name, project_id, device string) error

	AddWorkspaceConfig(name, project_id string, item *WorkspaceConfigValue) error
	AddWorkspacesConfig(filter WorkspaceConfigFilter, item *WorkspaceConfigValue) error
	RemoveWorkspaceConfig(name, project_id string, key string) error

	GetWorkspace(name, project_id string) (*WorkspaceProps, error)
	GetWorkspaceFs(name, projec_id string) (WorkspaceFs, error)
	GetWorkspacesByConfig(filter WorkspaceConfigFilter) ([]*WorkspaceProps, error)
	GetWorkspacesByDevices(filter WorkspaceDeviceFilter) (map[string]*WorkspaceProps, error)

	Exec(ctx context.Context, name string, args *ExecArgs) (chan bool, error)
}

func New() (WorkspaceBackend, error) {
	server := LxdBackend{}

	if lxdInst, err := server.getLxdClient(context.Background()); err != nil {
		return nil, err
	} else {
		if err = InitProject(lxdInst); err != nil {
			return nil, err
		}
	}

	return &server, nil
}

func (s *LxdBackend) LaunchWorkspace(ctx context.Context, name, base string) error {
	var err error
	var imageSrv lxd.ImageServer
	var image *api.Image

	conn, err := s.getLxdClient(ctx)
	if err != nil {
		return err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	/* Skip if the instance exists already */
	if _, _, err := conn.GetInstance(util.ToInstanceName(name, projectId)); err == nil {
		return fmt.Errorf("workspace \"%s\" already exists", name)
	}

	/* Check if we have the base image stored locally */
	if alias, _, err := conn.GetImageAlias(base); err == nil {
		if image, _, err = conn.GetImage(alias.Target); err != nil {
			return err
		}
		imageSrv = conn
	} else {
		imageSrv, image, err = s.fetchRemoteImage(base)
		if err != nil {
			return err
		}

		defer conn.CreateImageAlias(api.ImageAliasesPost{ImageAliasesEntry: api.ImageAliasesEntry{
			Name: base,
			ImageAliasesEntryPut: api.ImageAliasesEntryPut{
				Target: image.Fingerprint,
			},
		}})
	}

	projectName, err := GetLXDProjectName()
	if err != nil {
		return err
	}

	req := api.InstancesPost{
		InstancePut: api.InstancePut{
			Devices: defaultDevices(),
			Config:  defaultConfig(projectId),
		},
		Name: util.ToInstanceName(name, projectId),
		Type: api.InstanceType("container"),
		Source: api.InstanceSource{
			Type:        "image",
			Fingerprint: image.Fingerprint,
			Project:     projectName,
		},
	}
	op, err := conn.CreateInstanceFromImage(imageSrv, *image, req)
	if err != nil {
		return err
	}

	return op.Wait()
}

func (s *LxdBackend) SetWorkspaceState(name, projectId, action string) error {
	conn, err := s.getLxdClient(context.Background())
	if err != nil {
		return err
	}

	inst, _, err := conn.GetInstance(util.ToInstanceName(name, projectId))
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
		Timeout: 5,
		Force:   false,
	}

	op, err := conn.UpdateInstanceState(inst.Name, req, "")
	if err != nil {
		return err
	}

	err = op.Wait()
	if err != nil {
		return err
	}
	return err
}

func (s *LxdBackend) AddWorkspaceConfig(name, projectId string, item *WorkspaceConfigValue) error {
	conn, err := s.getLxdClient(context.Background())
	if err != nil {
		return err
	}

	inst, etag, err := conn.GetInstance(util.ToInstanceName(name, projectId))
	if err != nil {
		return err
	}
	inst.Config[item.Name] = item.Value
	op, _ := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)

	return op.Wait()
}

func (s *LxdBackend) RemoveWorkspaceConfig(name, projectId string, key string) error {
	conn, err := s.getLxdClient(context.Background())
	if err != nil {
		return err
	}

	inst, etag, err := conn.GetInstance(util.ToInstanceName(name, projectId))
	if err != nil {
		return err
	}

	delete(inst.Config, key)
	op, _ := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)

	return op.Wait()
}

func (s *LxdBackend) AddWorkspaceDevice(name, projectId string, device WorkspaceDevice) error {
	conn, err := s.getLxdClient(context.Background())
	if err != nil {
		return err
	}

	inst, etag, err := conn.GetInstance(util.ToInstanceName(name, projectId))
	if err != nil {
		return err
	}
	inst.Devices[device.Name] = device.Properties
	op, _ := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)

	return op.Wait()
}

func (s *LxdBackend) RemoveWorkspaceDevice(name, projectId, device string) error {
	conn, err := s.getLxdClient(context.Background())
	if err != nil {
		return err
	}

	inst, etag, err := conn.GetInstance(util.ToInstanceName(name, projectId))
	if err != nil {
		return err
	}

	delete(inst.Devices, device)
	op, _ := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)

	return op.Wait()
}

func (s *LxdBackend) Exec(ctx context.Context, name string, args *ExecArgs) (chan bool, error) {
	conn, err := s.getLxdClient(ctx)
	if err != nil {
		return nil, err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return nil, fmt.Errorf("context key project-id not found")
	}

	req := api.InstanceExecPost{
		Command: args.Command, WaitForWS: true,
		User: 0, Group: 0, Cwd: args.WorkDir,
		Interactive: false,
	}

	done := make(chan bool)

	arg := lxd.InstanceExecArgs{
		Stdin: args.Stdin, Stdout: args.Stdout, Stderr: args.Stderr,
		Control: nil, DataDone: done,
	}

	op, err := conn.ExecInstance(util.ToInstanceName(name, projectId), req, &arg)

	if err != nil {
		return done, err
	}

	if err := op.WaitContext(ctx); err != nil {
		return done, err
	}

	if status, ok := op.Get().Metadata["return"].(float64); ok {
		if status != 0 {
			logger.Debugf("command execution failed with %v", int(status))
			return done, &ErrExec{Status: int(status)}
		}
	}

	return done, nil
}

func (s *LxdBackend) AddWorkspacesConfig(filter WorkspaceConfigFilter, item *WorkspaceConfigValue) error {
	conn, err := s.getLxdClient(context.Background())
	if err != nil {
		return err
	}

	inst, err := conn.GetInstances(api.InstanceTypeContainer)
	if err != nil {
		return err
	}

	for _, i := range inst {
		if filter(i.Config) {
			i.Config[item.Name] = item.Value
			conn.UpdateInstance(i.Name, i.InstancePut, "")
		}
	}

	return nil
}

func (s *LxdBackend) GetWorkspace(name, projectId string) (*WorkspaceProps, error) {
	conn, err := s.getLxdClient(context.Background())
	if err != nil {
		return nil, err
	}

	inst, _, err := conn.GetInstance(util.ToInstanceName(name, projectId))
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

func (s *LxdBackend) GetWorkspacesByConfig(filter WorkspaceConfigFilter) ([]*WorkspaceProps, error) {
	conn, err := s.getLxdClient(context.Background())
	if err != nil {
		return nil, err
	}

	instances, err := conn.GetInstances(api.InstanceTypeContainer)
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

func (s *LxdBackend) GetWorkspacesByDevices(filter WorkspaceDeviceFilter) (map[string]*WorkspaceProps, error) {
	conn, err := s.getLxdClient(context.Background())
	if err != nil {
		return nil, err
	}

	instances, err := conn.GetInstances(api.InstanceTypeContainer)
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

func (s *LxdBackend) DeleteWorkspace(name, projectId string) error {
	conn, err := s.getLxdClient(context.Background())
	if err != nil {
		return err
	}

	inst, _, err := conn.GetInstance(util.ToInstanceName(name, projectId))
	if err != nil {
		return err
	}

	if inst.StatusCode != 0 && inst.StatusCode != api.Stopped {
		return fmt.Errorf("cannot delete a non-stopped workspace: %q", name)
	}

	op, err := conn.DeleteInstance(util.ToInstanceName(name, projectId))
	if err != nil {
		return err
	}

	return op.Wait()
}

func (s *LxdBackend) GetWorkspaceFs(name, project_id string) (WorkspaceFs, error) {
	conn, err := s.getLxdClient(context.Background())
	if err != nil {
		return nil, err
	}

	sftp, err := conn.GetInstanceFileSFTP(util.ToInstanceName(name, project_id))
	if err != nil {
		return nil, err
	}

	return NewWorkspaceFs(sftp), nil
}

func (s *LxdBackend) getLxdClient(ctx context.Context) (lxd.InstanceServer, error) {
	project, err := GetLXDProjectName()
	if err != nil {
		return nil, err
	}
	if srv, err := lxd.ConnectLXDUnixWithContext(ctx, LxdSock, nil); err != nil {
		return nil, err
	} else {
		return srv.UseProject(project), nil
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

func defaultDevices() map[string]map[string]string {
	return map[string]map[string]string{
		"root":              {"type": "disk", "pool": "default", "path": "/"},
		"workspace.network": {"type": "nic", "network": "lxdbr0", "name": "eth0"},
	}
}

func defaultConfig(projectId string) map[string]string {
	return map[string]string{
		"raw.idmap":                 fmt.Sprint("uid ", os.Getuid(), " 1000\ngid ", os.Getgid(), " 1000"),
		"security.nesting":          "true",
		"user.workspace.project-id": projectId,
	}
}

func (s *LxdBackend) fetchRemoteImage(base string) (lxd.ImageServer, *api.Image, error) {
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

/* Fake backend implementation for tests */

type ExecFunc func(name string, project_id string, args *ExecArgs) (chan bool, error)

type FakeWorkspaceBackend struct {
	workspaces map[string]map[string]*WorkspaceProps
	Fs         WorkspaceFs

	DoExec ExecFunc
}

func NewFakeWorkspaceBackend() *FakeWorkspaceBackend {
	var be FakeWorkspaceBackend
	be.workspaces = make(map[string]map[string]*WorkspaceProps)
	be.Fs = NewFakeWorkspaceFs()

	be.DoExec = DoExecDefault

	return &be
}

func (f *FakeWorkspaceBackend) workspace(name, project string) *WorkspaceProps {
	return f.workspaces[project][name]
}

func (f *FakeWorkspaceBackend) LaunchWorkspace(ctx context.Context, name, base string) error {
	projectId := ctx.Value(ContextProjectId).(string)
	if f.workspaces[projectId] == nil {
		f.workspaces[projectId] = make(map[string]*WorkspaceProps)
	}
	f.workspaces[projectId][name] = &WorkspaceProps{
		Name:    name,
		Devices: defaultDevices(),
		Config:  defaultConfig(projectId),
		state:   util.Ready,
	}
	return nil
}

func (f *FakeWorkspaceBackend) DeleteWorkspace(name string, project_id string) error {
	panic("not implemented") // TODO: Implement
}

func (f *FakeWorkspaceBackend) SetWorkspaceState(name string, action string, project_id string) error {
	panic("not implemented") // TODO: Implement
}

func (f *FakeWorkspaceBackend) AddWorkspaceDevice(name string, project_id string, props WorkspaceDevice) error {
	f.workspaces[project_id][name].Devices[props.Name] = props.Properties
	return nil
}

func (f *FakeWorkspaceBackend) RemoveWorkspaceDevice(name string, project_id string, device string) error {
	delete(f.workspaces[project_id][name].Devices, device)
	return nil
}

func (f *FakeWorkspaceBackend) AddWorkspaceConfig(name string, project_id string, item *WorkspaceConfigValue) error {
	f.workspaces[project_id][name].Config[item.Name] = item.Value
	return nil
}

func (f *FakeWorkspaceBackend) AddWorkspacesConfig(filter WorkspaceConfigFilter, item *WorkspaceConfigValue) error {
	panic("not implemented") // TODO: Implement
}

func (f *FakeWorkspaceBackend) RemoveWorkspaceConfig(name string, project_id string, key string) error {
	delete(f.workspaces[project_id][name].Config, key)
	return nil
}

func (f *FakeWorkspaceBackend) GetWorkspace(name string, project_id string) (*WorkspaceProps, error) {
	return f.workspaces[project_id][name], nil
}

func (f *FakeWorkspaceBackend) GetWorkspacesByConfig(filter WorkspaceConfigFilter) ([]*WorkspaceProps, error) {
	res := make([]*WorkspaceProps, 0)
	for _, i := range f.workspaces {
		for _, j := range i {
			if filter(j.Config) {
				res = append(res, j)
			}
		}
	}
	return res, nil
}

func (f *FakeWorkspaceBackend) GetWorkspacesByDevices(filter WorkspaceDeviceFilter) (map[string]*WorkspaceProps, error) {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkspaceBackend) GetWorkspaceFs(name, project_id string) (WorkspaceFs, error) {
	return s.Fs, nil
}

func (f *FakeWorkspaceBackend) Exec(ctx context.Context, name string, args *ExecArgs) (chan bool, error) {
	projectId := ctx.Value(ContextProjectId).(string)
	return f.DoExec(name, projectId, args)
}

func DoExecDefault(name string, project_id string, args *ExecArgs) (chan bool, error) {
	done := make(chan bool)
	close(done)
	return done, nil
}
