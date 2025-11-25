package fakebackend

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/canonical/lxd/shared/api"

	"github.com/canonical/workshop/internal/fsutil"
	"github.com/canonical/workshop/internal/progress"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
)

/* Fake backend implementation for tests */

type ExecFunc func(ctx context.Context, name string, args *workshop.Execution) (workshop.ExecContext, error)

type FakeWorkshop struct {
	*workshop.Workshop
	Devices            map[string]map[string]string
	WorkshopFilesystem fsutil.Fs
}

type FakeVolume struct {
	Kind   string
	What   string
	Mounts []WorkshopVolumeMount
	Size   uint64
}

type WorkshopVolumeMount struct {
	ProjectId string
	Workshop  string
	Where     string
}

type ExecCall struct {
	Name string
	Args workshop.Execution
}

type FsCall struct {
	Name string
}

type DownloadCall struct {
	Image workshop.BaseImage
}

type AttachVolumeCall struct {
	Workshop string
	Name     string
}

type SnapshotCall struct {
	Workshop string
	Sdk      string
}

type RestoreCall struct {
	Workshop string
	Sdk      string
	File     *workshop.File
}

type WorkshopFsCallback func(ctx context.Context, name string) (fsutil.Fs, error)

type FakeWorkshopBackend struct {
	workshopLock sync.Mutex
	// the key is a project-id - workshop name
	Workshops map[string]map[string]*FakeWorkshop
	// workshops put to stash (e.g. during refresh)
	StashedWorkshops map[string]map[string]*FakeWorkshop
	// storage volumes, the key is a volume name
	volumeLock sync.Mutex
	Volumes    map[string]FakeVolume
	SdkVolumes map[string]sdk.Meta
	// the key is a username
	projects map[string][]workshop.Project

	execLock     sync.Mutex
	ExecCallback ExecFunc
	ExecCalls    []*ExecCall

	workshopFsLock     sync.Mutex
	WorkshopFsCallback WorkshopFsCallback
	WorkshopFsCalls    []*FsCall

	baseLock             sync.Mutex
	GetBaseCallback      func(ctx context.Context, base string) (workshop.BaseImage, error)
	DownloadBaseCallback func(ctx context.Context, image workshop.BaseImage, report *progress.Reporter) error
	DownloadBaseCalls    []*DownloadCall

	AttachVolumeCalls []AttachVolumeCall

	snapshotLock     sync.Mutex
	SnapshotCalls    []SnapshotCall
	SnapshotCallback func(ctx context.Context, workshop string, snapid string) error
	RestoreCalls     []RestoreCall
	RestoreCallback  func(ctx context.Context, workshop string, snapid string, File *workshop.File) error

	BaseDir string
}

var _ workshop.Backend = (*FakeWorkshopBackend)(nil)

func New(baseDir string) (*FakeWorkshopBackend, error) {
	var be FakeWorkshopBackend
	be.Workshops = make(map[string]map[string]*FakeWorkshop)
	be.StashedWorkshops = make(map[string]map[string]*FakeWorkshop)
	be.Volumes = make(map[string]FakeVolume)
	be.SdkVolumes = make(map[string]sdk.Meta)
	be.projects = make(map[string][]workshop.Project)

	be.ExecCallback = DoExecDefault
	be.BaseDir = baseDir

	return &be, nil
}

func (s *FakeWorkshopBackend) CreateOrLoadProject(ctx context.Context, path string) (*workshop.Project, bool, error) {
	username, ok := ctx.Value(workshop.ContextUser).(string)
	if !ok {
		return nil, false, errors.New("user not found")
	}
	if val, ok := s.projects[username]; ok {
		idx := slices.IndexFunc(val, func(p workshop.Project) bool { return p.Path == path })
		if idx != -1 {
			return &val[idx], false, nil
		}
	} else {
		s.projects[username] = make([]workshop.Project, 0)
	}

	prjId, _ := workshop.NewProjectId()
	newPrj := workshop.Project{ProjectId: prjId, Path: path}
	s.projects[username] = append(s.projects[username], newPrj)
	return &newPrj, true, nil
}

func (f *FakeWorkshopBackend) Projects(ctx context.Context) (map[string][]workshop.Project, error) {
	userName, ok := ctx.Value(workshop.ContextUser).(string)
	if ok {
		return map[string][]workshop.Project{userName: f.projects[userName]}, nil
	}
	return maps.Clone(f.projects), nil
}

func (f *FakeWorkshopBackend) project(user, id string) *workshop.Project {
	prjs := f.projects[user]
	idx := slices.IndexFunc(prjs, func(p workshop.Project) bool { return p.ProjectId == id })
	if idx != -1 {
		return &prjs[idx]
	}
	return nil
}

func (f *FakeWorkshopBackend) LaunchOrRebuildWorkshop(ctx context.Context, file *workshop.File, image workshop.BaseImage) error {
	user, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}

	prj := f.project(user, projectId)

	f.workshopLock.Lock()
	if f.Workshops[projectId] == nil {
		f.Workshops[projectId] = make(map[string]*FakeWorkshop)
	}
	ws, ok := f.Workshops[projectId][file.Name]
	if !ok {
		ws = &FakeWorkshop{}
		f.Workshops[projectId][file.Name] = ws
	}
	f.workshopLock.Unlock()

	if ok {
		// Remove SDKs.
		sdks := ws.SdksByInstallOrder()
		slices.Reverse(sdks)
		for _, setup := range sdks {
			if err := f.UninstallSdk(ctx, file.Name, setup.Setup); err != nil {
				return err
			}
		}

		// rebuild the workshop
		ws.File = file
		ws.Image = image
	} else {
		ws.Workshop = &workshop.Workshop{Backend: f,
			Name:    file.Name,
			Running: false,
			Project: *prj,
			Image:   image,
			File:    file,
		}
	}

	wfspath, err := os.MkdirTemp(f.BaseDir, "*")
	if err != nil {
		return err
	}
	ws.WorkshopFilesystem = fsutil.NewBasePathFs(wfspath)

	ws.Devices = make(map[string]map[string]string)

	ws.Sdks = make(map[string]workshop.SdkInstallation)
	ws.Profiles = make(map[string]workshop.SdkProfile, 0)

	return nil
}

func (f *FakeWorkshopBackend) RemoveWorkshop(ctx context.Context, name string) error {
	user, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}

	prj := f.project(user, projectId)

	f.workshopLock.Lock()
	wp, ok := f.Workshops[prj.ProjectId][name]
	f.workshopLock.Unlock()
	if !ok {
		return workshop.ErrWorkshopNotLaunched
	}

	for _, sk := range wp.Sdks {
		if err := f.UninstallSdk(ctx, name, sk.Setup); err != nil {
			return err
		}
	}

	f.workshopLock.Lock()
	delete(f.Workshops[prj.ProjectId], name)
	f.workshopLock.Unlock()
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

func (f *FakeWorkshopBackend) AddWorkshopMount(ctx context.Context, name string, mount workshop.Mount) error {
	if mount.Type != workshop.HostWorkshop {
		return errors.New("fake backend only supports HostWorkshop mounts")
	}

	wfs, err := f.WorkshopFs(ctx, name)
	if err != nil {
		return err
	}
	defer wfs.Close()

	mnt, err := wfs.FsBackend.(*fsutil.BasePathFs).RealPath(mount.Where)
	if err != nil {
		return err
	}

	if mount.MakeWhere {
		if err := os.MkdirAll(filepath.Dir(mnt), mount.Mode); err != nil {
			return err
		}
	}

	if err := os.Symlink(mount.What, mnt); err != nil {
		return err
	}

	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}

	f.workshopLock.Lock()
	defer f.workshopLock.Unlock()

	f.Workshops[projectId][name].Devices[mount.Name] = map[string]string{"type": "disk", "source": mount.What,
		"path": mount.Where}
	return nil
}

func (f *FakeWorkshopBackend) RemoveWorkshopMount(ctx context.Context, name, mount string) error {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return err
	}

	f.workshopLock.Lock()
	device, ok := f.Workshops[projectId][name].Devices[mount]
	delete(f.Workshops[projectId][name].Devices, mount)
	f.workshopLock.Unlock()
	if !ok {
		return fmt.Errorf("mount %q not found", mount)
	}

	wfs, err := f.WorkshopFs(ctx, name)
	if err != nil {
		return err
	}
	defer wfs.Close()

	where, err := wfs.FsBackend.(*fsutil.BasePathFs).RealPath(device["path"])
	if err != nil {
		return err
	}

	return os.Remove(where)
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

	f.workshopLock.Lock()
	defer f.workshopLock.Unlock()

	wp := f.Workshops[projectId][name]
	if wp == nil {
		return nil, workshop.ErrWorkshopNotLaunched
	}
	return wp.Workshop, nil
}

func (f *FakeWorkshopBackend) ProjectWorkshops(ctx context.Context) ([]*workshop.Workshop, error) {
	_, projectId, err := f.userProject(ctx)
	if err != nil {
		return nil, err
	}

	f.workshopLock.Lock()
	var names []string
	for _, i := range f.Workshops[projectId] {
		names = append(names, i.Name)
	}
	f.workshopLock.Unlock()

	var workshops = make([]*workshop.Workshop, 0)
	for _, name := range names {
		ws, _ := f.Workshop(ctx, name)
		workshops = append(workshops, ws)
	}
	return workshops, nil
}

func (s *FakeWorkshopBackend) SetWorkshopFsCallback(c WorkshopFsCallback) func() {
	s.workshopFsLock.Lock()
	defer s.workshopFsLock.Unlock()

	old := s.WorkshopFsCallback
	s.WorkshopFsCallback = c

	return func() {
		s.workshopFsLock.Lock()
		defer s.workshopFsLock.Unlock()

		s.WorkshopFsCallback = old
	}
}

func (s *FakeWorkshopBackend) WorkshopFs(ctx context.Context, name string) (fsutil.Fs, error) {
	s.workshopFsLock.Lock()
	s.WorkshopFsCalls = append(s.WorkshopFsCalls, &FsCall{Name: name})
	if s.WorkshopFsCallback != nil {
		defer s.workshopFsLock.Unlock()
		return s.WorkshopFsCallback(ctx, name)
	}
	s.workshopFsLock.Unlock()

	_, projectId, err := s.userProject(ctx)
	if err != nil {
		return fsutil.Fs{}, err
	}

	s.workshopLock.Lock()
	defer s.workshopLock.Unlock()

	fs, exists := s.Workshops[projectId][name]
	if !exists {
		return fsutil.Fs{}, fmt.Errorf(`%q filesystem is not available`, name)
	}
	return fs.WorkshopFilesystem, nil
}

func (f *FakeWorkshopBackend) Exec(ctx context.Context, name string, args *workshop.Execution) (workshop.ExecContext, error) {
	f.execLock.Lock()
	f.ExecCalls = append(f.ExecCalls, &ExecCall{name, *args})
	f.execLock.Unlock()
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
	_, projectId, err := s.userProject(ctx)
	if err != nil {
		return err
	}

	s.workshopLock.Lock()
	defer s.workshopLock.Unlock()

	delete(s.StashedWorkshops[projectId], "stash-"+name)
	return nil
}

func (s *FakeWorkshopBackend) UnstashWorkshop(ctx context.Context, name string) error {
	_, projectId, err := s.userProject(ctx)
	if err != nil {
		return err
	}

	s.workshopLock.Lock()
	defer s.workshopLock.Unlock()

	wp := s.StashedWorkshops[projectId]["stash-"+name]
	if wp == nil {
		return fmt.Errorf("stashed workshop %q not found", name)
	}
	wp.Name = name
	s.Workshops[projectId][name] = wp
	wp.Running = true

	return nil
}

func (s *FakeWorkshopBackend) StashWorkshop(ctx context.Context, name string) error {
	_, projectId, err := s.userProject(ctx)
	if err != nil {
		return err
	}

	s.workshopLock.Lock()
	defer s.workshopLock.Unlock()

	wp := s.Workshops[projectId][name]
	if wp == nil {
		return workshop.ErrWorkshopNotLaunched
	}

	if s.StashedWorkshops[projectId] == nil {
		s.StashedWorkshops[projectId] = make(map[string]*FakeWorkshop)
	}
	wp.Running = false

	wcpy := *wp.Workshop
	stashed := *wp
	stashed.Workshop = &wcpy
	stashed.Name = "stash-" + name

	s.StashedWorkshops[projectId][stashed.Name] = &stashed
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

func (b *FakeWorkshopBackend) GetBase(ctx context.Context, base string) (workshop.BaseImage, error) {
	b.baseLock.Lock()
	defer b.baseLock.Unlock()

	if b.GetBaseCallback != nil {
		return b.GetBaseCallback(ctx, base)
	}
	return workshop.BaseImage{Name: base, Fingerprint: "fakeimage123"}, nil
}

func (b *FakeWorkshopBackend) DownloadBase(ctx context.Context, image workshop.BaseImage, report *progress.Reporter) error {
	b.baseLock.Lock()
	defer b.baseLock.Unlock()

	b.DownloadBaseCalls = append(b.DownloadBaseCalls, &DownloadCall{Image: image})
	if b.DownloadBaseCallback != nil {
		return b.DownloadBaseCallback(ctx, image, report)
	}
	return nil
}

// ImportSdk imports an SDK tarball into the volume. The tarball must be a valid
// directory (unpacked so that the tests could provide a temp directory instead
// of a packed tarball).
func (s *FakeWorkshopBackend) ImportSdk(ctx context.Context, meta sdk.Meta, tarball *os.File) error {
	s.volumeLock.Lock()
	defer s.volumeLock.Unlock()

	name := sdk.VolumeName(meta.Name, meta.Revision)
	if _, ok := s.Volumes[name]; ok {
		return workshop.ErrVolumeAlreadyExists
	}

	s.Volumes[name] = FakeVolume{Kind: "sdk", What: tarball.Name()}
	s.SdkVolumes[name] = meta
	return nil
}

func (b *FakeWorkshopBackend) DeleteSdk(ctx context.Context, setup sdk.Setup) error {
	b.volumeLock.Lock()
	defer b.volumeLock.Unlock()

	what := sdk.VolumeName(setup.Name, setup.Revision)
	volume, ok := b.Volumes[what]
	if !ok {
		return nil
	}

	if len(volume.Mounts) > 0 {
		return workshop.ErrVolumeInUse
	}

	delete(b.Volumes, what)
	delete(b.SdkVolumes, what)
	return nil
}

func (s *FakeWorkshopBackend) Sdks(ctx context.Context) ([]workshop.SdkVolume, error) {
	s.volumeLock.Lock()
	defer s.volumeLock.Unlock()

	var sdks []workshop.SdkVolume
	for name, volume := range s.Volumes {
		if volume.Kind != "sdk" {
			continue
		}
		sdks = append(sdks, s.sdkVolume(name, volume))
	}
	return sdks, nil
}

func (s *FakeWorkshopBackend) Sdk(ctx context.Context, setup sdk.Setup) (workshop.SdkVolume, error) {
	s.volumeLock.Lock()
	defer s.volumeLock.Unlock()

	name := sdk.VolumeName(setup.Name, setup.Revision)
	volume, ok := s.Volumes[name]
	if !ok {
		return workshop.SdkVolume{}, workshop.ErrVolumeNotFound
	}
	return s.sdkVolume(name, volume), nil
}

func (s *FakeWorkshopBackend) sdkVolume(name string, volume FakeVolume) workshop.SdkVolume {
	workshops := map[string][]string{}
	for _, mount := range volume.Mounts {
		workshops[mount.ProjectId] = append(workshops[mount.ProjectId], mount.Workshop)
	}

	return workshop.SdkVolume{
		Meta:      s.SdkVolumes[name],
		Workshops: workshops,
		Size:      volume.Size,
	}
}

func (b *FakeWorkshopBackend) InstallSdk(ctx context.Context, name string, setup sdk.Setup) error {
	_, projectId, err := b.userProject(ctx)
	if err != nil {
		return err
	}

	b.workshopLock.Lock()
	wp := b.Workshops[projectId][name]
	b.workshopLock.Unlock()
	if _, exist := wp.Sdks[setup.Name]; exist {
		return fmt.Errorf("%q SDK is already installed", setup.Name)
	}

	wp.Sdks[setup.Name] = workshop.SdkInstallation{Setup: setup, InstallTime: workshop.InstallTimeNow()}

	userDataDir := filepath.Join(b.BaseDir, "share")
	mount := workshop.SdkMount(userDataDir, projectId, name, setup)
	if mount.Type == workshop.Volume {
		return b.attachVolume(ctx, name, mount)
	}
	return b.AddWorkshopMount(ctx, name, mount)
}

func (s *FakeWorkshopBackend) attachVolume(ctx context.Context, name string, mount workshop.Mount) error {
	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	s.volumeLock.Lock()
	defer s.volumeLock.Unlock()

	s.AttachVolumeCalls = append(s.AttachVolumeCalls, AttachVolumeCall{Workshop: name, Name: mount.What})

	wfs, err := s.WorkshopFs(ctx, name)
	if err != nil {
		return err
	}
	defer wfs.Close()

	mnt, err := wfs.FsBackend.(*fsutil.BasePathFs).RealPath(mount.Where)
	if err != nil {
		return err
	}

	volume, ok := s.Volumes[mount.What]
	if !ok {
		return fmt.Errorf("volume %q not found", mount.What)
	}
	entry := WorkshopVolumeMount{ProjectId: projectId, Workshop: name, Where: mount.Where}
	if slices.Contains(volume.Mounts, entry) {
		return fmt.Errorf("volume %q already mounted in %q", mount.What, name)
	}

	if err := os.MkdirAll(filepath.Dir(mnt), 0755); err != nil {
		return err
	}

	if err := os.Symlink(volume.What, mnt); err != nil {
		return err
	}

	volume.Mounts = append(volume.Mounts, entry)
	s.Volumes[mount.What] = volume
	return nil
}

func (b *FakeWorkshopBackend) UninstallSdk(ctx context.Context, name string, setup sdk.Setup) error {
	_, projectId, err := b.userProject(ctx)
	if err != nil {
		return err
	}

	b.workshopLock.Lock()
	wp := b.Workshops[projectId][name]
	b.workshopLock.Unlock()
	delete(wp.Sdks, setup.Name)

	if setup.IsVolume() {
		what := sdk.VolumeName(setup.Name, setup.Revision)
		where := sdk.SdkDir(setup.Name)
		return b.detachVolume(ctx, name, what, where)
	}
	what := workshop.SdkDeviceName(setup.Name)
	return b.RemoveWorkshopMount(ctx, name, what)
}

func (s *FakeWorkshopBackend) detachVolume(ctx context.Context, name, what, where string) error {
	projectId, ok := ctx.Value(workshop.ContextProjectId).(string)
	if !ok {
		return fmt.Errorf("context key project-id not found")
	}

	s.volumeLock.Lock()
	defer s.volumeLock.Unlock()

	volume, ok := s.Volumes[what]
	if !ok {
		return fmt.Errorf("volume %q not found", what)
	}
	idx := slices.IndexFunc(volume.Mounts, func(m WorkshopVolumeMount) bool {
		return m.ProjectId == projectId && m.Workshop == name && m.Where == where
	})
	if idx < 0 {
		return fmt.Errorf("volume %q not mounted in %q", what, name)
	}

	wfs, err := s.WorkshopFs(ctx, name)
	if err != nil {
		return err
	}
	defer wfs.Close()

	err = wfs.Remove(volume.Mounts[idx].Where)
	if err != nil {
		return err
	}

	volume.Mounts = slices.Delete(volume.Mounts, idx, idx+1)
	s.Volumes[what] = volume
	return err
}

func (s *FakeWorkshopBackend) Snapshot(ctx context.Context, name, sk string) error {
	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	s.SnapshotCalls = append(s.SnapshotCalls, SnapshotCall{Workshop: name, Sdk: sk})
	if s.SnapshotCallback != nil {
		return s.SnapshotCallback(ctx, name, sk)
	}
	return nil
}

func (s *FakeWorkshopBackend) Restore(ctx context.Context, name, sk string, file *workshop.File) error {
	wp, err := s.Workshop(ctx, name)
	if err != nil {
		return err
	}

	sdks := wp.SdksByInstallOrder()
	lastIntact := slices.IndexFunc(sdks, func(s workshop.SdkInstallation) bool { return s.Name == sk })
	if lastIntact < 0 {
		return fmt.Errorf("invalid snapshot %q", sk)
	}
	unwantedSdks := sdks[lastIntact+1:]
	slices.Reverse(unwantedSdks)

	fs, err := s.WorkshopFs(ctx, name)
	if err != nil {
		return err
	}
	defer fs.Close()

	// Remove project mount
	if err := fs.RemoveIfExists(workshop.WorkshopProjectPath); err != nil {
		return err
	}

	// Remove SDKs from after the snapshot.
	for _, u := range unwantedSdks {
		if err := s.UninstallSdk(ctx, name, u.Setup); err != nil {
			return err
		}
	}

	wp.File = file

	s.snapshotLock.Lock()
	defer s.snapshotLock.Unlock()

	s.RestoreCalls = append(s.RestoreCalls, RestoreCall{Workshop: name, Sdk: sk, File: file})
	if s.RestoreCallback != nil {
		return s.RestoreCallback(ctx, name, sk, file)
	}
	return nil
}
