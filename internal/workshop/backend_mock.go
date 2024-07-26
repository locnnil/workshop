package workshop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/canonical/lxd/shared/api"
	"golang.org/x/exp/slices"

	"github.com/canonical/workshop/internal/progress"
	"github.com/canonical/workshop/internal/sdk"
)

/* Fake backend implementation for tests */

type ExecFunc func(ctx context.Context, name string, args *Execution) (ExecContext, error)

type FakeWorkshop struct {
	*Workshop
	Config             map[string]string
	Devices            map[string]map[string]string
	WorkshopFilesystem WorkshopFs
	Profiles           []SdkProfile
}

type ExecCall struct {
	Name string
	Args *Execution
}

type FsCall struct {
	Name string
}

type AssignProfileCall struct {
	Name    string
	Profile SdkProfile
}

type RemoveProfileCall struct {
	Name    string
	Profile string
}

type DownloadCall struct {
	Base string
}

type FakeWorkshopBackend struct {
	// the key is a project-id - workshop name
	Workshops map[string]map[string]*FakeWorkshop
	// workshops put to stash (e.g. during refresh)
	StashedWorkshops map[string]map[string]*FakeWorkshop
	// the key is a username
	projects map[string][]*Project

	ExecCallback ExecFunc
	ExecCalls    []*ExecCall

	WorkshopFsCallback func(ctx context.Context, name string) (WorkshopFs, error)
	WorkshopFsCalls    []*FsCall

	AssignProfileCallback func(ctx context.Context, workshop string, profile SdkProfile) error
	AssignProfileCalls    []*AssignProfileCall

	RemoveProfileCallback func(ctx context.Context, workshop string, profile string) error
	RemoveProfileCalls    []*RemoveProfileCall

	DownloadBaseCallback func(ctx context.Context, base string, report *progress.Reporter) error
	DownloadBaseCalls    []*DownloadCall
}

func NewFakeWorkshopBackend() *FakeWorkshopBackend {
	var be FakeWorkshopBackend
	be.Workshops = make(map[string]map[string]*FakeWorkshop)
	be.StashedWorkshops = make(map[string]map[string]*FakeWorkshop)
	be.projects = make(map[string][]*Project)

	be.ExecCallback = DoExecDefault

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

func (f *FakeWorkshopBackend) LaunchWorkshop(ctx context.Context, file *File) error {
	user, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}

	prj := f.project(user, projectId)

	if f.Workshops[projectId] == nil {
		f.Workshops[projectId] = make(map[string]*FakeWorkshop)
	}
	if _, ok := f.Workshops[projectId][file.Name]; ok {
		return errors.New("workshop exists")
	}

	ws := &FakeWorkshop{}
	ws.Workshop = &Workshop{Backend: f,
		Name:    file.Name,
		Running: false,
		Project: prj,
		Base:    file.Base,
		File:    file,
	}
	ws.Config = make(map[string]string)
	ws.Devices = make(map[string]map[string]string)
	ws.WorkshopFilesystem = NewFakeWorkshopFs()
	ws.Content = make(map[string]sdk.Setup)

	content := make(map[string]sdk.Setup)
	f.Workshops[projectId][file.Name] = ws

	for _, s := range file.Sdks {
		setup := sdk.Setup{
			Name:    s.Name,
			Channel: s.Channel,
		}
		ws.LinkSdk(ctx, setup)
		content[setup.Name] = setup
	}

	buf, err := json.Marshal(content)
	if err != nil {
		return err
	}

	ws.Config[ConfigWorkshopContent] = string(buf)

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
	if w.Running {
		return api.StatusErrorf(http.StatusConflict, "workshop already running")
	}
	w.Running = true
	return nil
}

func (s *FakeWorkshopBackend) StopWorkshop(ctx context.Context, name string, force bool) error {
	w, err := s.Workshop(ctx, name)
	if err != nil {
		return err
	}
	w.Running = false
	return nil
}

func (f *FakeWorkshopBackend) AddWorkshopDevice(ctx context.Context, name string, props Device) error {
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

func (f *FakeWorkshopBackend) AssignProfile(ctx context.Context, workshop string, profile SdkProfile) error {
	f.AssignProfileCalls = append(f.AssignProfileCalls, &AssignProfileCall{Name: workshop, Profile: profile})

	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}
	f.Workshops[projectId][workshop].Profiles = append(f.Workshops[projectId][workshop].Profiles, profile)

	if f.AssignProfileCallback != nil {
		return f.AssignProfileCallback(ctx, workshop, profile)
	}
	return nil
}

func (s *FakeWorkshopBackend) Profile(ctx context.Context, w string, pr string) (SdkProfile, error) {
	_, projectId, err := s.userProject(ctx)
	if err != nil {
		return SdkProfile{}, err
	}
	wp, ok := s.Workshops[projectId][w]
	if !ok {
		return SdkProfile{}, ErrWorkshopNotFound
	}

	profiles := wp.Profiles
	idx := slices.IndexFunc(profiles, func(p SdkProfile) bool { return p.Name() == pr })
	if idx != -1 {
		return s.Workshops[projectId][w].Profiles[idx], nil
	}
	return SdkProfile{}, ErrSdkProfileNotFound
}

func (s *FakeWorkshopBackend) RemoveProfile(ctx context.Context, workshop string, profile string) error {
	s.RemoveProfileCalls = append(s.RemoveProfileCalls, &RemoveProfileCall{Name: workshop, Profile: profile})

	if s.RemoveProfileCallback != nil {
		return s.RemoveProfileCallback(ctx, workshop, profile)
	}
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
	return ErrSdkProfileNotFound
}

func (f *FakeWorkshopBackend) Profiles(ctx context.Context, workshop string) ([]SdkProfile, error) {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return nil, err
	}
	return f.Workshops[projectId][workshop].Profiles, nil
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

	var c map[string]sdk.Setup
	if err := json.Unmarshal([]byte(f.Workshops[projectId][name].Config[ConfigWorkshopContent]), &c); err != nil {
		return nil, err
	}
	workshop.Content = c
	return workshop.Workshop, nil
}

func (f *FakeWorkshopBackend) ProjectWorkshops(ctx context.Context) ([]*File, []*Workshop, error) {
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
	s.WorkshopFsCalls = append(s.WorkshopFsCalls, &FsCall{Name: name})
	if s.WorkshopFsCallback != nil {
		return s.WorkshopFsCallback(ctx, name)
	}

	_, projectId, err := s.userProject(ctx)
	if err != nil {
		return nil, err
	}
	return s.Workshops[projectId][name].WorkshopFilesystem, nil
}

func (f *FakeWorkshopBackend) Exec(ctx context.Context, name string, args *Execution) (ExecContext, error) {
	f.ExecCalls = append(f.ExecCalls, &ExecCall{name, args})
	return f.ExecCallback(ctx, name, args)
}

func DoExecDefault(ctx context.Context, name string, args *Execution) (ExecContext, error) {
	return ExecContext{
		WaitExecution: func(ctx context.Context) error {
			return nil
		},
	}, nil
}

func (s *FakeWorkshopBackend) RemoveWorkshopStash(ctx context.Context, name string) error {
	return nil
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
	return nil
}

func (s *FakeWorkshopBackend) DeleteStateStorage(ctx context.Context, name string) error {
	return nil
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
func (b *FakeWorkshopBackend) Download(ctx context.Context, base string, report *progress.Reporter) error {
	b.DownloadBaseCalls = append(b.DownloadBaseCalls, &DownloadCall{Base: base})
	if b.DownloadBaseCallback != nil {
		return b.DownloadBaseCallback(ctx, base, report)
	}
	return nil
}
