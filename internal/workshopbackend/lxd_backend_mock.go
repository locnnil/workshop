package workshopbackend

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/lxd/shared/api"
	"golang.org/x/exp/slices"
)

/* Fake backend implementation for tests */

type ExecFunc func(ctx context.Context, name string, args *Execution) (ExecContext, error)

type FakeWorkshop struct {
	*Workshop
	Config             map[string]string
	WorkshopFilesystem WorkshopFs
	Profiles           []SdkProfile
}

type ExecCall struct {
	Name string
	Args *Execution
}

type FakeWorkshopBackend struct {
	// the key is a project-id - workshop name
	Workshops map[string]map[string]*FakeWorkshop
	// workshops put to stash (e.g. during refresh)
	StashedWorkshops map[string]map[string]*FakeWorkshop
	// the key is a username
	projects map[string][]*Project

	DoExec    ExecFunc
	ExecCalls []*ExecCall
}

func NewFakeWorkshopBackend() *FakeWorkshopBackend {
	var be FakeWorkshopBackend
	be.Workshops = make(map[string]map[string]*FakeWorkshop)
	be.StashedWorkshops = make(map[string]map[string]*FakeWorkshop)
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
	ws.Config = make(map[string]string)
	ws.WorkshopFilesystem = NewFakeWorkshopFs()

	ws.Workshop = &Workshop{backend: f,
		Name:    name,
		devices: defaultDevices(),
		running: true,
		project: prj,
		content: make(map[string]sdk.Setup),
		base:    base,
	}

	f.Workshops[projectId][name] = ws

	file, err := prj.WorkshopFile(name)
	if err != nil {
		return err
	}
	for _, s := range file.Sdks {
		ws.LinkSdk(ctx, sdk.Setup{
			Name:    s.Name,
			Channel: s.Channel,
		})
	}

	ws.Profiles = make([]SdkProfile, 0)
	return nil
}

func (f *FakeWorkshopBackend) RemoveWorkshop(ctx context.Context, name string) error {
	user, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}

	prj := f.project(user, projectId)

	if _, ok := f.Workshops[prj.ProjectId][name]; !ok {
		return ErrWorkshopNotFound
	}

	delete(f.Workshops[prj.ProjectId], name)
	return nil
}

func (s *FakeWorkshopBackend) StartWorkshop(ctx context.Context, name string) error {
	w, err := s.Workshop(ctx, name)
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
	w, err := s.Workshop(ctx, name)
	if err != nil {
		return err
	}
	w.running = false
	return nil
}

func (f *FakeWorkshopBackend) AddWorkshopDevice(ctx context.Context, name string, props Device) error {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}
	f.Workshops[projectId][name].devices[props.Name()] = props.properties
	return nil
}

func (f *FakeWorkshopBackend) RemoveWorkshopDevice(ctx context.Context, name string, device string) error {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}
	delete(f.Workshops[projectId][name].devices, device)
	return nil
}

func (f *FakeWorkshopBackend) AssignProfile(ctx context.Context, workshop string, profile SdkProfile) error {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}
	f.Workshops[projectId][workshop].Profiles = append(f.Workshops[projectId][workshop].Profiles, profile)
	return nil
}

func (s *FakeWorkshopBackend) RemoveProfile(ctx context.Context, workshop string, profile string) error {
	_, projectId, err := s.userProject(ctx)
	if err != nil {
		return err
	}
	profiles := s.Workshops[projectId][workshop].Profiles
	idx := slices.IndexFunc(profiles, func(p SdkProfile) bool { return p.Name() == profile })
	if idx != -1 {
		s.Workshops[projectId][workshop].Profiles = slices.Delete(profiles, idx, idx+1)
		return nil
	}
	return errors.New("profile not found")
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

func (f *FakeWorkshopBackend) Workshop(ctx context.Context, name string) (*Workshop, error) {
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

	workshop.content, err = InstalledContent(f.Workshops[projectId][name].Config)
	if err != nil {
		return nil, err
	}
	return workshop.Workshop, nil
}

func (f *FakeWorkshopBackend) ProjectWorkshops(ctx context.Context) ([]*WorkshopFile, []*Workshop, error) {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return nil, nil, err
	}

	var workshops = make([]*Workshop, 0)
	for _, i := range f.Workshops[projectId] {
		ws, _ := f.Workshop(ctx, i.Name)
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

func (s *FakeWorkshopBackend) WorkshopFs(ctx context.Context, name string) (WorkshopFs, error) {
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
		WaitExecution: func(ctx context.Context) error {
			return nil
		},
	}, nil
}

func (s *FakeWorkshopBackend) RemoveWorkshopStash(ctx context.Context, name string) error {
	panic("not implemented") // TODO: Implement
}

func (s *FakeWorkshopBackend) UnstashWorkshop(ctx context.Context, name string) error {
	_, projectId, err := s.userProject(ctx)
	if err != nil {
		return err
	}

	workshop := s.StashedWorkshops[projectId][StashNamePrefix+name]
	if workshop == nil {
		return ErrWorkshopNotFound
	}
	delete(s.StashedWorkshops[projectId], StashNamePrefix+name)
	workshop.Name = name
	s.Workshops[projectId][name] = workshop
	return nil
}

func (s *FakeWorkshopBackend) StashWorkshop(ctx context.Context, name string) error {
	_, projectId, err := s.userProject(ctx)
	if err != nil {
		return err
	}

	workshop := s.Workshops[projectId][name]
	if workshop == nil {
		return ErrWorkshopNotFound
	}

	if s.StashedWorkshops[projectId] == nil {
		s.StashedWorkshops[projectId] = make(map[string]*FakeWorkshop)
	}
	s.StashedWorkshops[projectId][StashNamePrefix+name] = workshop
	workshop.Name = StashNamePrefix + name
	delete(s.Workshops[projectId], name)
	return nil
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

func FakeDefaultDevices(f func() map[string]map[string]string) func() {
	oldDefault := defaultDevices
	defaultDevices = f
	return func() { defaultDevices = oldDefault }
}

func FakeImageServer(server string) func() {
	oldImageServer := imageServer
	imageServer = server
	return func() { imageServer = oldImageServer }
}
