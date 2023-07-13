package workspacebackend

import (
	"context"

	"github.com/canonical/workspace/internal/sdk"
	"github.com/spf13/afero"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

/* Fake backend implementation for tests */

type ExecFunc func(ctx context.Context, name string, args *ExecArgs) (chan bool, error)

type FakeWorkspace struct {
	*Workspace
	Config map[string]string
}

type FakeWorkspaceBackend struct {
	Workspaces map[string]map[string]*FakeWorkspace
	projects   map[string]map[string]*Project
	WsFs       WorkspaceFs
	LocalFs    afero.Fs

	DoExec ExecFunc
}

func NewFakeWorkspaceBackend() *FakeWorkspaceBackend {
	var be FakeWorkspaceBackend
	be.Workspaces = make(map[string]map[string]*FakeWorkspace)
	be.projects = make(map[string]map[string]*Project)
	be.WsFs = NewFakeWorkspaceFs()
	be.LocalFs = afero.NewMemMapFs()

	be.DoExec = DoExecDefault

	return &be
}

func (s *FakeWorkspaceBackend) CreateOrLoadProject(ctx context.Context, path string) (*Project, bool, error) {
	username := ctx.Value(ContextUser).(string)
	if val, ok := s.projects[username]; ok {
		idx := slices.IndexFunc(maps.Values(val), func(p *Project) bool { return p.Path == path })
		if idx != -1 {
			return maps.Values(val)[idx], false, nil
		}
	} else {
		s.projects[username] = make(map[string]*Project)
	}

	prjId, _ := NewProjectId()
	newPrj := &Project{ProjectId: prjId, Path: path}
	s.projects[username][prjId] = newPrj
	return newPrj, true, nil
}

func (s *FakeWorkspaceBackend) Projects(ctx context.Context) (map[string]*Project, error) {
	username := ctx.Value(ContextUser).(string)
	return s.projects[username], nil
}

func (f *FakeWorkspaceBackend) LaunchWorkspace(ctx context.Context, name, base string) error {
	projectId := ctx.Value(ContextProjectId).(string)

	if f.Workspaces[projectId] == nil {
		f.Workspaces[projectId] = make(map[string]*FakeWorkspace)
	}
	ws := &FakeWorkspace{}
	ws.Config = make(map[string]string)
	ws.Workspace = &Workspace{backend: f,
		Name:      name,
		Devices:   defaultDevices(),
		isRunning: true,
		projectId: projectId,
		content:   make(map[string]*sdk.SdkInfo),
	}
	f.Workspaces[projectId][name] = ws
	return nil
}

func (f *FakeWorkspaceBackend) DeleteWorkspace(ctx context.Context, name string, forceful bool) error {
	panic("not implemented") // TODO: Implement
}

func (f *FakeWorkspaceBackend) SetWorkspaceState(ctx context.Context, name, action string) error {
	panic("not implemented") // TODO: Implement
}

func (f *FakeWorkspaceBackend) AddWorkspaceDevice(ctx context.Context, name string, props WorkspaceDevice) error {
	projectId := ctx.Value(ContextProjectId).(string)
	f.Workspaces[projectId][name].Devices[props.Name] = props.Properties
	return nil
}

func (f *FakeWorkspaceBackend) RemoveWorkspaceDevice(ctx context.Context, name string, device string) error {
	projectId := ctx.Value(ContextProjectId).(string)
	delete(f.Workspaces[projectId][name].Devices, device)
	return nil
}

func (f *FakeWorkspaceBackend) AddWorkspaceConfig(ctx context.Context, name string, item *WorkspaceConfigValue) error {
	projectId := ctx.Value(ContextProjectId).(string)
	f.Workspaces[projectId][name].Config[item.Name] = item.Value
	return nil
}

func (f *FakeWorkspaceBackend) RemoveWorkspaceConfig(ctx context.Context, name string, key string) error {
	projectId := ctx.Value(ContextProjectId).(string)
	delete(f.Workspaces[projectId][name].Config, key)
	return nil
}

func (f *FakeWorkspaceBackend) GetWorkspace(ctx context.Context, name string) (*Workspace, error) {
	projectId := ctx.Value(ContextProjectId).(string)
	user := ctx.Value(ContextUser).(string)
	project := f.projects[user][projectId]
	workspace := f.Workspaces[projectId][name].Workspace
	workspace.file, _ = project.WorkspaceFile(workspace.Name)
	return workspace, nil
}

func (f *FakeWorkspaceBackend) GetProjectWorkspaces(ctx context.Context) ([]*WorkspaceFile, []*Workspace, error) {
	projectId := ctx.Value(ContextProjectId).(string)
	var workspaces = make([]*Workspace, 0)
	for _, i := range f.Workspaces[projectId] {
		ws, _ := f.GetWorkspace(ctx, i.Name)
		workspaces = append(workspaces, ws)
	}
	return nil, workspaces, nil
}

func (f *FakeWorkspaceBackend) GetWorkspacesByConfig(ctx context.Context, filter WorkspaceConfigFilter) ([]*Workspace, error) {
	res := make([]*Workspace, 0)
	for _, i := range f.Workspaces {
		for _, j := range i {
			if filter(j.Config) {
				res = append(res, j.Workspace)
			}
		}
	}
	return res, nil
}

func (f *FakeWorkspaceBackend) GetWorkspacesByDevices(ctx context.Context, filter WorkspaceDeviceFilter) (map[string]*Workspace, error) {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkspaceBackend) GetWorkspaceFs(ctx context.Context, name string) (WorkspaceFs, error) {
	return s.WsFs, nil
}

func (f *FakeWorkspaceBackend) Exec(ctx context.Context, name string, args *ExecArgs) (chan bool, error) {
	return f.DoExec(ctx, name, args)
}

func DoExecDefault(ctx context.Context, name string, args *ExecArgs) (chan bool, error) {
	done := make(chan bool)
	close(done)

	return done, nil
}

func (s *FakeWorkspaceBackend) DeleteUnavailableWorkspace(ctx context.Context, name string) error {
	return nil
}

func (s *FakeWorkspaceBackend) MakeWorkspaceAvailable(ctx context.Context, name string) error {
	return nil
}

func (s *FakeWorkspaceBackend) MakeWorkspaceUnavailable(ctx context.Context, name string) error {
	return nil
}

func (s *FakeWorkspaceBackend) CreateStateStorage(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkspaceBackend) DeleteStateStorage(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}
