package workshopbackend

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/canonical/workshop/internal/sdk"
	"github.com/lxc/lxd/shared/api"
	"golang.org/x/exp/slices"
)

/* Fake backend implementation for tests */

type ExecFunc func(ctx context.Context, name string, args *Execution) (ExecContext, error)

type FakeWorkshop struct {
	*Workshop
	Config             map[string]string
	WorkshopFilesystem WorkshopFs
}

type ExecCall struct {
	Name string
	Args *Execution
}

type FakeWorkshopBackend struct {
	Workshops map[string]map[string]*FakeWorkshop
	// the key is a username
	projects map[string][]*Project

	DoExec    ExecFunc
	ExecCalls []*ExecCall
}

func NewFakeWorkshopBackend() *FakeWorkshopBackend {
	var be FakeWorkshopBackend
	be.Workshops = make(map[string]map[string]*FakeWorkshop)
	be.projects = make(map[string][]*Project)

	be.DoExec = DoExecDefault

	return &be
}

func (s *FakeWorkshopBackend) CreateOrLoadProject(ctx context.Context, path string) (*Project, bool, error) {
	username, ok := ctx.Value(ContextUser).(string)
	if !ok {
		return nil, false, errors.New("user not found")
	}
	if val, ok := s.projects[username]; ok {
		idx := slices.IndexFunc(val, func(p *Project) bool { return p.Path == path })
		if idx != -1 {
			return val[idx], false, nil
		}
	} else {
		s.projects[username] = make([]*Project, 0)
	}

	prjId, _ := NewProjectId()
	newPrj := &Project{ProjectId: prjId, Path: path}
	s.projects[username] = append(s.projects[username], newPrj)
	return newPrj, true, nil
}

func (f *FakeWorkshopBackend) Projects(ctx context.Context) (map[string][]*Project, error) {
	userName, ok := ctx.Value(ContextUser).(string)
	if ok {
		return map[string][]*Project{userName: f.projects[userName]}, nil
	}
	all := map[string][]*Project{}
	for name, prjs := range f.projects {
		all[name] = prjs
	}
	return all, nil
}

func (f *FakeWorkshopBackend) project(user, id string) *Project {
	prjs := f.projects[user]
	idx := slices.IndexFunc(prjs, func(p *Project) bool { return p.ProjectId == id })
	if idx != -1 {
		return prjs[idx]
	}
	return nil
}

func (f *FakeWorkshopBackend) LaunchWorkshop(ctx context.Context, name, base string) error {
	user, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}

	prj := f.project(user, projectId)

	if f.Workshops[projectId] == nil {
		f.Workshops[projectId] = make(map[string]*FakeWorkshop)
	}
	if _, ok := f.Workshops[projectId][name]; ok {
		return api.StatusErrorf(http.StatusNotFound, "workshop exists already")
	}

	ws := &FakeWorkshop{}
	file, err := prj.WorkshopFile(name)
	if err != nil {
		return err
	}

	ws.Config = make(map[string]string)
	ws.WorkshopFilesystem = NewFakeWorkshopFs()

	ws.Workshop = &Workshop{backend: f,
		Name:      name,
		Devices:   defaultDevices(),
		running:   true,
		projectId: projectId,
		content:   make(map[string]sdk.Setup),
		file:      file,
	}

	f.Workshops[projectId][name] = ws

	for _, s := range ws.File().Sdks {
		ws.LinkSdk(ctx, sdk.Setup{
			Workshop: name,
			Name:     s.Name,
			Channel:  s.Channel,
		})
	}
	return nil
}

func (f *FakeWorkshopBackend) RemoveWorkshop(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkshopBackend) StartWorkshop(ctx context.Context, name string) error {
	w, err := s.GetWorkshop(ctx, name)
	if err != nil {
		return err
	}
	if w.running {
		return api.StatusErrorf(http.StatusConflict, "workshop already running")
	}
	w.running = true
	return nil
}

func (s *FakeWorkshopBackend) StopWorkshop(ctx context.Context, name string, force bool) error {
	w, err := s.GetWorkshop(ctx, name)
	if err != nil {
		return err
	}
	w.running = false
	return nil
}

func (f *FakeWorkshopBackend) AddWorkshopDevice(ctx context.Context, name string, props WorkshopDevice) error {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}
	f.Workshops[projectId][name].Devices[props.Name] = props.Properties
	return nil
}

func (f *FakeWorkshopBackend) RemoveWorkshopDevice(ctx context.Context, name string, device string) error {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}
	delete(f.Workshops[projectId][name].Devices, device)
	return nil
}

func (f *FakeWorkshopBackend) AddWorkshopConfig(ctx context.Context, name string, item *WorkshopConfigValue) error {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}
	f.Workshops[projectId][name].Config[item.Name] = item.Value
	return nil
}

func (f *FakeWorkshopBackend) RemoveWorkshopConfig(ctx context.Context, name string, key string) error {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}
	delete(f.Workshops[projectId][name].Config, key)
	return nil
}

func (f *FakeWorkshopBackend) GetWorkshop(ctx context.Context, name string) (*Workshop, error) {
	user, projectId, err := f.userProject(ctx)
	if err != nil {
		return nil, err
	}

	project := f.project(user, projectId)
	if project == nil {
		return nil, api.StatusErrorf(404, "project not found")
	}
	workshop := f.Workshops[projectId][name]
	if workshop == nil {
		return nil, ErrWorkshopNotFound
	}
	workshop.file, err = project.WorkshopFile(workshop.Name)
	if err != nil {
		return nil, err
	}

	workshop.content, err = InstalledContent(f.Workshops[projectId][name].Config)
	if err != nil {
		return nil, err
	}
	return workshop.Workshop, nil
}

func (f *FakeWorkshopBackend) GetProjectWorkshops(ctx context.Context) ([]*WorkshopFile, []*Workshop, error) {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return nil, nil, err
	}

	var workshops = make([]*Workshop, 0)
	for _, i := range f.Workshops[projectId] {
		ws, _ := f.GetWorkshop(ctx, i.Name)
		workshops = append(workshops, ws)
	}
	return nil, workshops, nil
}

func (f *FakeWorkshopBackend) GetWorkshopsByConfig(ctx context.Context, filter WorkshopConfigFilter) ([]*Workshop, error) {
	res := make([]*Workshop, 0)
	for _, i := range f.Workshops {
		for _, j := range i {
			if filter(j.Config) {
				res = append(res, j.Workshop)
			}
		}
	}
	return res, nil
}

func (f *FakeWorkshopBackend) GetWorkshopsByDevices(ctx context.Context, filter WorkshopDeviceFilter) (map[string]*Workshop, error) {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkshopBackend) GetWorkshopFs(ctx context.Context, name string) (WorkshopFs, error) {
	_, projectId, err := s.userProject(ctx)
	if err != nil {
		return nil, err
	}
	return s.Workshops[projectId][name].WorkshopFilesystem, nil
}

func (f *FakeWorkshopBackend) Exec(ctx context.Context, name string, args *Execution) (ExecContext, error) {
	f.ExecCalls = append(f.ExecCalls, &ExecCall{name, args})
	return f.DoExec(ctx, name, args)
}

func DoExecDefault(ctx context.Context, name string, args *Execution) (ExecContext, error) {
	return ExecContext{
		WaitExecution: func(ctx context.Context) error { return nil },
	}, nil
}

func (s *FakeWorkshopBackend) RemoveWorkshopStash(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkshopBackend) UnstashWorkshop(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkshopBackend) StashWorkshop(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkshopBackend) CreateStateStorage(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkshopBackend) DeleteStateStorage(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkshopBackend) userProject(ctx context.Context) (string, string, error) {
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
