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
type ContextKeyUser string

const (
	ContextProjectId = ContextKeyProjectId("project-id")
	ContextUser      = ContextKeyUser("user")
)

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
	DeleteWorkspace(ctx context.Context, name string) error
	SetWorkspaceState(ctx context.Context, name, action string) error

	AddWorkspaceDevice(ctx context.Context, name string, props WorkspaceDevice) error
	RemoveWorkspaceDevice(ctx context.Context, name string, props string) error

	AddWorkspaceConfig(ctx context.Context, name string, item *WorkspaceConfigValue) error
	RemoveWorkspaceConfig(ctx context.Context, name string, key string) error

	GetWorkspace(ctx context.Context, name string) (*WorkspaceProps, error)
	GetWorkspaceFs(ctx context.Context, name string) (WorkspaceFs, error)
	GetWorkspacesByConfig(ctx context.Context, filter WorkspaceConfigFilter) ([]*WorkspaceProps, error)
	GetWorkspacesByDevices(ctx context.Context, filter WorkspaceDeviceFilter) (map[string]*WorkspaceProps, error)

	Exec(ctx context.Context, name string, args *ExecArgs) (chan bool, error)
}

func New() WorkspaceBackend {
	server := LxdBackend{}
	return &server
}

func lxdProjectName(user string) string {
	return "workspace." + user
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

	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return fmt.Errorf("context key user not found")
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
			Project:     lxdProjectName(user),
		},
	}
	op, err := conn.CreateInstanceFromImage(imageSrv, *image, req)
	if err != nil {
		return err
	}

	return op.Wait()
}

func (s *LxdBackend) SetWorkspaceState(ctx context.Context, name, action string) error {
	conn, err := s.getLxdClient(ctx)
	if err != nil {
		return err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
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

func (s *LxdBackend) AddWorkspaceConfig(ctx context.Context, name string, item *WorkspaceConfigValue) error {
	conn, err := s.getLxdClient(ctx)
	if err != nil {
		return err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(util.ToInstanceName(name, projectId))
	if err != nil {
		return err
	}
	inst.Config[item.Name] = item.Value
	op, _ := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)

	return op.Wait()
}

func (s *LxdBackend) RemoveWorkspaceConfig(ctx context.Context, name string, key string) error {
	conn, err := s.getLxdClient(ctx)
	if err != nil {
		return err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(util.ToInstanceName(name, projectId))
	if err != nil {
		return err
	}

	delete(inst.Config, key)
	op, _ := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)

	return op.Wait()
}

func (s *LxdBackend) AddWorkspaceDevice(ctx context.Context, name string, device WorkspaceDevice) error {
	conn, err := s.getLxdClient(ctx)
	if err != nil {
		return err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	inst, etag, err := conn.GetInstance(util.ToInstanceName(name, projectId))
	if err != nil {
		return err
	}
	inst.Devices[device.Name] = device.Properties
	op, _ := conn.UpdateInstance(inst.Name, inst.InstancePut, etag)

	return op.Wait()
}

func (s *LxdBackend) RemoveWorkspaceDevice(ctx context.Context, name string, device string) error {
	conn, err := s.getLxdClient(ctx)
	if err != nil {
		return err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
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

func (s *LxdBackend) GetWorkspace(ctx context.Context, name string) (*WorkspaceProps, error) {
	conn, err := s.getLxdClient(ctx)
	if err != nil {
		return nil, err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return nil, fmt.Errorf("context key project-id not found")
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

func (s *LxdBackend) GetWorkspacesByConfig(ctx context.Context, filter WorkspaceConfigFilter) ([]*WorkspaceProps, error) {
	conn, err := s.getLxdClient(ctx)
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

func (s *LxdBackend) GetWorkspacesByDevices(ctx context.Context, filter WorkspaceDeviceFilter) (map[string]*WorkspaceProps, error) {
	conn, err := s.getLxdClient(ctx)
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

func (s *LxdBackend) DeleteWorkspace(ctx context.Context, name string) error {
	conn, err := s.getLxdClient(ctx)
	if err != nil {
		return err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
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

func (s *LxdBackend) GetWorkspaceFs(ctx context.Context, name string) (WorkspaceFs, error) {
	conn, err := s.getLxdClient(ctx)
	if err != nil {
		return nil, err
	}

	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return nil, fmt.Errorf("context key project-id not found")
	}

	sftp, err := conn.GetInstanceFileSFTP(util.ToInstanceName(name, projectId))
	if err != nil {
		return nil, err
	}

	return NewWorkspaceFs(sftp), nil
}

func (s *LxdBackend) getLxdClient(ctx context.Context) (lxd.InstanceServer, error) {
	user, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return nil, fmt.Errorf("context key %s not found", ContextUser)
	}

	if srv, err := lxd.ConnectLXDUnixWithContext(ctx, LxdSock, nil); err != nil {
		return nil, err
	} else {
		if err = InitProject(srv, lxdProjectName(user)); err != nil {
			return nil, err
		}
		return srv.UseProject(lxdProjectName(user)), nil
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

type ExecFunc func(ctx context.Context, name string, args *ExecArgs) (chan bool, error)

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

func (f *FakeWorkspaceBackend) DeleteWorkspace(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}

func (f *FakeWorkspaceBackend) SetWorkspaceState(ctx context.Context, name, action string) error {
	panic("not implemented") // TODO: Implement
}

func (f *FakeWorkspaceBackend) AddWorkspaceDevice(ctx context.Context, name string, props WorkspaceDevice) error {
	projectId := ctx.Value(ContextProjectId).(string)
	f.workspaces[projectId][name].Devices[props.Name] = props.Properties
	return nil
}

func (f *FakeWorkspaceBackend) RemoveWorkspaceDevice(ctx context.Context, name string, device string) error {
	projectId := ctx.Value(ContextProjectId).(string)
	delete(f.workspaces[projectId][name].Devices, device)
	return nil
}

func (f *FakeWorkspaceBackend) AddWorkspaceConfig(ctx context.Context, name string, item *WorkspaceConfigValue) error {
	projectId := ctx.Value(ContextProjectId).(string)
	f.workspaces[projectId][name].Config[item.Name] = item.Value
	return nil
}

func (f *FakeWorkspaceBackend) RemoveWorkspaceConfig(ctx context.Context, name string, key string) error {
	projectId := ctx.Value(ContextProjectId).(string)
	delete(f.workspaces[projectId][name].Config, key)
	return nil
}

func (f *FakeWorkspaceBackend) GetWorkspace(ctx context.Context, name string) (*WorkspaceProps, error) {
	projectId := ctx.Value(ContextProjectId).(string)
	return f.workspaces[projectId][name], nil
}

func (f *FakeWorkspaceBackend) GetWorkspacesByConfig(ctx context.Context, filter WorkspaceConfigFilter) ([]*WorkspaceProps, error) {
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

func (f *FakeWorkspaceBackend) GetWorkspacesByDevices(ctx context.Context, filter WorkspaceDeviceFilter) (map[string]*WorkspaceProps, error) {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkspaceBackend) GetWorkspaceFs(ctx context.Context, name string) (WorkspaceFs, error) {
	return s.Fs, nil
}

func (f *FakeWorkspaceBackend) Exec(ctx context.Context, name string, args *ExecArgs) (chan bool, error) {
	return f.DoExec(ctx, name, args)
}

func DoExecDefault(ctx context.Context, name string, args *ExecArgs) (chan bool, error) {
	done := make(chan bool)
	close(done)
	return done, nil
}
