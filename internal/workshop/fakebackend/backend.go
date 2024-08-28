package fakebackend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"github.com/canonical/lxd/shared/api"
	"github.com/spf13/afero"
	"golang.org/x/exp/slices"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/progress"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
)

/* Fake backend implementation for tests */

type ExecFunc func(ctx context.Context, name string, args *workshop.Execution) (workshop.ExecContext, error)

type FakeWorkshop struct {
	*workshop.Workshop
	Config             map[string]string
	Devices            map[string]map[string]string
	WorkshopFilesystem workshop.WorkshopFs
	Profiles           []workshop.SdkProfile
}

type ExecCall struct {
	Name string
	Args *workshop.Execution
}

type FsCall struct {
	Name string
}

type AssignProfileCall struct {
	Name    string
	Profile workshop.SdkProfile
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
	// state storages, the key is a volume name
	WorkshopStateStorages map[string]map[string]bool
	// the key is a username
	projects map[string][]*workshop.Project

	ExecCallback ExecFunc
	ExecCalls    []*ExecCall

	WorkshopFsCallback func(ctx context.Context, name string) (workshop.WorkshopFs, error)
	WorkshopFsCalls    []*FsCall

	AssignProfileCallback func(ctx context.Context, workshop string, profile workshop.SdkProfile) error
	AssignProfileCalls    []*AssignProfileCall

	RemoveProfileCallback func(ctx context.Context, workshop string, profile string) error
	RemoveProfileCalls    []*RemoveProfileCall

	DownloadBaseCallback func(ctx context.Context, base string, report *progress.Reporter) error
	DownloadBaseCalls    []*DownloadCall
}

func New() (workshop.Backend, error) {
	var be FakeWorkshopBackend
	be.Workshops = make(map[string]map[string]*FakeWorkshop)
	be.StashedWorkshops = make(map[string]map[string]*FakeWorkshop)
	be.WorkshopStateStorages = make(map[string]map[string]bool)
	be.projects = make(map[string][]*workshop.Project)

	be.ExecCallback = DoExecDefault

	return &be, nil
}

func (s *FakeWorkshopBackend) CreateOrLoadProject(ctx context.Context, path string) (*workshop.Project, bool, error) {
	username, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return nil, false, errors.New("user not found")
	}
	if val, ok := s.projects[username]; ok {
		idx := slices.IndexFunc(val, func(p *workshop.Project) bool { return p.Path == path })
		if idx != -1 {
			return val[idx], false, nil
		}
	} else {
		s.projects[username] = make([]*workshop.Project, 0)
	}

	prjId, _ := workshop.NewProjectId()
	newPrj := &workshop.Project{ProjectId: prjId, Path: path}
	s.projects[username] = append(s.projects[username], newPrj)
	return newPrj, true, nil
}

func (f *FakeWorkshopBackend) Projects(ctx context.Context) (map[string][]*workshop.Project, error) {
	userName, ok := ctx.Value(workshop.ContextUser).(string)
	if ok {
		return map[string][]*workshop.Project{userName: f.projects[userName]}, nil
	}
	all := map[string][]*workshop.Project{}
	for name, prjs := range f.projects {
		all[name] = prjs
	}
	return all, nil
}

func (f *FakeWorkshopBackend) project(user, id string) *workshop.Project {
	prjs := f.projects[user]
	idx := slices.IndexFunc(prjs, func(p *workshop.Project) bool { return p.ProjectId == id })
	if idx != -1 {
		return prjs[idx]
	}
	return nil
}

func (f *FakeWorkshopBackend) LaunchWorkshop(ctx context.Context, file *workshop.File) error {
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
	ws.WorkshopFilesystem = NewWorkshopFs()
	ws.Workshop = &workshop.Workshop{Backend: f,
		Name:    file.Name,
		Running: false,
		Project: prj,
		Base:    file.Base,
		File:    file,
	}
	ws.Config = make(map[string]string)
	ws.Config[workshop.ConfigWorkshopContent] = `{}`
	ws.Devices = make(map[string]map[string]string)
	ws.Content = make(map[string]sdk.Setup)
	ws.Profiles = make([]workshop.SdkProfile, 0)

	f.Workshops[projectId][file.Name] = ws
	return nil
}

func (f *FakeWorkshopBackend) RemoveWorkshop(ctx context.Context, name string) error {
	user, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}

	prj := f.project(user, projectId)

	if _, ok := f.Workshops[prj.ProjectId][name]; !ok {
		return workshop.ErrWorkshopNotFound
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

func (f *FakeWorkshopBackend) AddWorkshopDevice(ctx context.Context, name string, props workshop.Device) error {
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

func (f *FakeWorkshopBackend) AssignProfile(ctx context.Context, workshop string, profile workshop.SdkProfile) error {
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

func (s *FakeWorkshopBackend) Profile(ctx context.Context, w string, pr string) (workshop.SdkProfile, error) {
	_, projectId, err := s.userProject(ctx)
	if err != nil {
		return workshop.SdkProfile{}, err
	}
	wp, ok := s.Workshops[projectId][w]
	if !ok {
		return workshop.SdkProfile{}, workshop.ErrWorkshopNotFound
	}

	profiles := wp.Profiles
	idx := slices.IndexFunc(profiles, func(p workshop.SdkProfile) bool { return p.Name() == pr })
	if idx != -1 {
		return s.Workshops[projectId][w].Profiles[idx], nil
	}
	return workshop.SdkProfile{}, workshop.ErrSdkProfileNotFound
}

func (s *FakeWorkshopBackend) RemoveProfile(ctx context.Context, wp string, profile string) error {
	s.RemoveProfileCalls = append(s.RemoveProfileCalls, &RemoveProfileCall{Name: wp, Profile: profile})

	if s.RemoveProfileCallback != nil {
		return s.RemoveProfileCallback(ctx, wp, profile)
	}
	_, projectId, err := s.userProject(ctx)
	if err != nil {
		return err
	}
	profiles := s.Workshops[projectId][wp].Profiles
	idx := slices.IndexFunc(profiles, func(p workshop.SdkProfile) bool { return p.Name() == profile })
	if idx != -1 {
		s.Workshops[projectId][wp].Profiles = slices.Delete(profiles, idx, idx+1)
		return nil
	}
	return workshop.ErrSdkProfileNotFound
}

func (f *FakeWorkshopBackend) Profiles(ctx context.Context, workshop string) ([]workshop.SdkProfile, error) {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return nil, err
	}
	return f.Workshops[projectId][workshop].Profiles, nil
}

func (f *FakeWorkshopBackend) AddWorkshopConfig(ctx context.Context, name string, item *workshop.WorkshopConfigValue) error {
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

func (f *FakeWorkshopBackend) Workshop(ctx context.Context, name string) (*workshop.Workshop, error) {
	user, projectId, err := f.userProject(ctx)
	if err != nil {
		return nil, err
	}

	project := f.project(user, projectId)
	if project == nil {
		return nil, api.StatusErrorf(404, "project not found")
	}
	wp := f.Workshops[projectId][name]
	if wp == nil {
		return nil, workshop.ErrWorkshopNotFound
	}

	var c map[string]sdk.Setup
	if err := json.Unmarshal([]byte(f.Workshops[projectId][name].Config[workshop.ConfigWorkshopContent]), &c); err != nil {
		return nil, err
	}
	wp.Content = c
	return wp.Workshop, nil
}

func (f *FakeWorkshopBackend) ProjectWorkshops(ctx context.Context) ([]string, []*workshop.Workshop, error) {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return nil, nil, err
	}

	var workshops = make([]*workshop.Workshop, 0)
	for _, i := range f.Workshops[projectId] {
		ws, _ := f.Workshop(ctx, i.Name)
		workshops = append(workshops, ws)
	}
	return nil, workshops, nil
}

func (f *FakeWorkshopBackend) GetWorkshopsByConfig(ctx context.Context, filter workshop.WorkshopConfigFilter) ([]*workshop.Workshop, error) {
	res := make([]*workshop.Workshop, 0)
	for _, i := range f.Workshops {
		for _, j := range i {
			if filter(j.Config) {
				res = append(res, j.Workshop)
			}
		}
	}
	return res, nil
}

func (s *FakeWorkshopBackend) WorkshopFs(ctx context.Context, name string) (workshop.WorkshopFs, error) {
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

func (f *FakeWorkshopBackend) Exec(ctx context.Context, name string, args *workshop.Execution) (workshop.ExecContext, error) {
	f.ExecCalls = append(f.ExecCalls, &ExecCall{name, args})
	return f.ExecCallback(ctx, name, args)
}

func DoExecDefault(ctx context.Context, name string, args *workshop.Execution) (workshop.ExecContext, error) {
	return workshop.ExecContext{
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

	wp := s.StashedWorkshops[projectId][workshop.StashNamePrefix+name]
	if wp == nil {
		return workshop.ErrWorkshopNotFound
	}
	delete(s.StashedWorkshops[projectId], workshop.StashNamePrefix+name)
	wp.Name = name
	s.Workshops[projectId][name] = wp
	return nil
}

func (s *FakeWorkshopBackend) StashWorkshop(ctx context.Context, name string) error {
	_, projectId, err := s.userProject(ctx)
	if err != nil {
		return err
	}

	wp := s.Workshops[projectId][name]
	if wp == nil {
		return workshop.ErrWorkshopNotFound
	}

	if s.StashedWorkshops[projectId] == nil {
		s.StashedWorkshops[projectId] = make(map[string]*FakeWorkshop)
	}
	s.StashedWorkshops[projectId][workshop.StashNamePrefix+name] = wp
	wp.Name = workshop.StashNamePrefix + name
	delete(s.Workshops[projectId], name)
	return nil
}

func (s *FakeWorkshopBackend) AttachStateStorage(ctx context.Context, wp, name string) error {
	paths := s.WorkshopStateStorages[name]
	if paths == nil {
		s.WorkshopStateStorages[name] = map[string]bool{}
		return nil
	}
	wfs, err := s.WorkshopFs(ctx, wp)
	if err != nil {
		return err
	}
	defer wfs.Close()
	for path := range paths {
		if err = wfs.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	return nil
}

func (s *FakeWorkshopBackend) DetachStateStorage(ctx context.Context, wp, name string) error {
	wfs, err := s.WorkshopFs(ctx, wp)
	if err != nil {
		return err
	}
	defer wfs.Close()

	afero.Walk(wfs, dirs.WorkshopStateDir, func(path string, info fs.FileInfo, err error) error {
		s.WorkshopStateStorages[name][path] = true
		return nil
	})

	err = wfs.RemoveAll(dirs.WorkshopStateDir)
	return err
}

func (s *FakeWorkshopBackend) CreateStateStorage(ctx context.Context, name string) error {
	return nil
}

func (s *FakeWorkshopBackend) DeleteStateStorage(ctx context.Context, name string) error {
	return nil
}

func (s *FakeWorkshopBackend) userProject(ctx context.Context) (string, string, error) {
	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return "", "", fmt.Errorf("context key project-id not found")
	}

	userName, ok := ctx.Value(workshop.ContextUser).(string)
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

/* Fake workshop fs implementation for tests */

type FakeInstanceFs struct {
	afero.Fs
}

func NewWorkshopFs() workshop.WorkshopFs {
	var fs FakeInstanceFs
	fs.Fs = afero.NewMemMapFs()
	return &fs
}

func (w *FakeInstanceFs) Symlink(source, target string) error {
	return w.Fs.Mkdir(target, os.ModeSymlink)
}

func (w *FakeInstanceFs) Close() {
}
