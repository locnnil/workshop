package workspacebackend

import (
	"context"
	"fmt"
	"net/http"

	"github.com/canonical/workspace/internal/sdk"
	"github.com/lxc/lxd/shared/api"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

/* Fake backend implementation for tests */

type ExecFunc func(ctx context.Context, name string, args *Execution) (ExecContext, error)

type FakeWorkspace struct {
	*Workspace
	Config              map[string]string
	WorkspaceFilesystem WorkspaceFs
}

type ExecCall struct {
	Name string
	Args *Execution
}

type FakeWorkspaceBackend struct {
	Workspaces map[string]map[string]*FakeWorkspace
	projects   map[string]map[string]*Project

	DoExec    ExecFunc
	ExecCalls []*ExecCall
}

func NewFakeWorkspaceBackend() *FakeWorkspaceBackend {
	var be FakeWorkspaceBackend
	be.Workspaces = make(map[string]map[string]*FakeWorkspace)
	be.projects = make(map[string]map[string]*Project)

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

func (f *FakeWorkspaceBackend) Projects(ctx context.Context) (map[string]*Project, error) {
	username, _, err := f.userProject(ctx)
	if err != nil {
		return nil, err
	}
	return f.projects[username], nil
}

func (f *FakeWorkspaceBackend) LaunchWorkspace(ctx context.Context, name, base string) error {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}
	projects, err := f.Projects(ctx)
	if err != nil {
		return err
	}
	prj := projects[projectId]

	if f.Workspaces[projectId] == nil {
		f.Workspaces[projectId] = make(map[string]*FakeWorkspace)
	}
	if _, ok := f.Workspaces[projectId][name]; ok {
		return api.StatusErrorf(http.StatusNotFound, "workspace exists already")
	}

	ws := &FakeWorkspace{}
	file, err := prj.WorkspaceFile(name)
	if err != nil {
		return err
	}

	ws.Config = make(map[string]string)
	ws.WorkspaceFilesystem = NewFakeWorkspaceFs()

	ws.Workspace = &Workspace{backend: f,
		Name:      name,
		Devices:   defaultDevices(),
		running:   true,
		projectId: projectId,
		content:   make(map[string]*sdk.SdkInfo),
		file:      file,
	}

	f.Workspaces[projectId][name] = ws

	for _, s := range ws.File().Sdks {
		ws.LinkSdk(ctx, &sdk.SdkInfo{
			Name:    s.Name,
			Channel: s.Channel,
		})
	}
	return nil
}

func (f *FakeWorkspaceBackend) RemoveWorkspace(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkspaceBackend) StartWorkspace(ctx context.Context, name string) error {
	w, err := s.GetWorkspace(ctx, name)
	if err != nil {
		return err
	}
	if w.running {
		return api.StatusErrorf(http.StatusConflict, "workspace already running")
	}
	w.running = true
	return nil
}

func (s *FakeWorkspaceBackend) StopWorkspace(ctx context.Context, name string, force bool) error {
	w, err := s.GetWorkspace(ctx, name)
	if err != nil {
		return err
	}
	w.running = false
	return nil
}

func (f *FakeWorkspaceBackend) AddWorkspaceDevice(ctx context.Context, name string, props WorkspaceDevice) error {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}
	f.Workspaces[projectId][name].Devices[props.Name] = props.Properties
	return nil
}

func (f *FakeWorkspaceBackend) RemoveWorkspaceDevice(ctx context.Context, name string, device string) error {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}
	delete(f.Workspaces[projectId][name].Devices, device)
	return nil
}

func (f *FakeWorkspaceBackend) AddWorkspaceConfig(ctx context.Context, name string, item *WorkspaceConfigValue) error {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}
	f.Workspaces[projectId][name].Config[item.Name] = item.Value
	return nil
}

func (f *FakeWorkspaceBackend) RemoveWorkspaceConfig(ctx context.Context, name string, key string) error {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}
	delete(f.Workspaces[projectId][name].Config, key)
	return nil
}

func (f *FakeWorkspaceBackend) GetWorkspace(ctx context.Context, name string) (*Workspace, error) {
	user, projectId, err := f.userProject(ctx)
	if err != nil {
		return nil, err
	}

	project := f.projects[user][projectId]
	if project == nil {
		return nil, api.StatusErrorf(404, "project not found")
	}
	workspace := f.Workspaces[projectId][name]
	if workspace == nil {
		return nil, ErrWorkspaceNotFound
	}
	workspace.file, err = project.WorkspaceFile(workspace.Name)
	if err != nil {
		return nil, err
	}

	workspace.content, err = InstalledContent(f.Workspaces[projectId][name].Config)
	if err != nil {
		return nil, err
	}
	return workspace.Workspace, nil
}

func (f *FakeWorkspaceBackend) GetProjectWorkspaces(ctx context.Context) ([]*WorkspaceFile, []*Workspace, error) {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return nil, nil, err
	}

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
	_, projectId, err := s.userProject(ctx)
	if err != nil {
		return nil, err
	}
	return s.Workspaces[projectId][name].WorkspaceFilesystem, nil
}

func (f *FakeWorkspaceBackend) Exec(ctx context.Context, name string, args *Execution) (ExecContext, error) {
	f.ExecCalls = append(f.ExecCalls, &ExecCall{name, args})
	return f.DoExec(ctx, name, args)
}

func DoExecDefault(ctx context.Context, name string, args *Execution) (ExecContext, error) {
	return ExecContext{
		WaitExecution: func(ctx context.Context) error { return nil },
	}, nil
}

func (s *FakeWorkspaceBackend) RemoveWorkspaceStash(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkspaceBackend) UnstashWorkspace(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkspaceBackend) StashWorkspace(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkspaceBackend) CreateStateStorage(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkspaceBackend) DeleteStateStorage(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkspaceBackend) userProject(ctx context.Context) (string, string, error) {
	projectId, ok := ctx.Value(ContextProjectId).(string)
	if !ok {
		return "", "", fmt.Errorf("context key project-id not found")
	}

	userName, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return "", "", fmt.Errorf("context key user not found")
	}
	return userName, projectId, nil
}
