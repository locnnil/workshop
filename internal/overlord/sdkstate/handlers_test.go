package sdkstate_test

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/afero"
	"gopkg.in/check.v1"
	"gopkg.in/tomb.v2"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/ifacetest"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/overlord"
	"github.com/canonical/workshop/internal/overlord/sdkstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/sdk/system"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshop"
	"github.com/canonical/workshop/internal/workshop/fakebackend"
)

type sdkStateSuite struct {
	fs          afero.Fs
	backend     *fakebackend.FakeWorkshopBackend
	state       *state.State
	runner      *state.TaskRunner
	se          *overlord.StateEngine
	sdkmgr      *sdkstate.SdkManager
	repo        *interfaces.Repository
	ctx         context.Context
	project     workshop.Project
	installTime time.Time

	restoreProjectId   func()
	restoreInstallTime func()
}

var _ = check.Suite(&sdkStateSuite{})

func TestMain(m *testing.M) {
	// Ensure consistent file permissions for sdkStateSuite.
	syscall.Umask(0002)
	m.Run()
}

func Test(t *testing.T) { check.TestingT(t) }

func fakeHandler(task *state.Task, _ *tomb.Tomb) error {
	return nil
}

func setWorkshopProject(w string, p workshop.Project, tasks ...*state.Task) {
	for _, i := range tasks {
		i.Set("workshop", w)
		i.Set("project", p)
	}
}

var ErrTrigger = errors.New("error out")

var sdkYaml = `
name: test
base: ubuntu@22.04
plugs:
  plug:
    interface: test-interface
    attr: value
  plug2:
    interface: test-interface
    attr2: value2
`

var sdkYamlViolatesPolicy = `
name: test-broken
base: ubuntu@22.04
plugs:
  plug:
    interface: test-interface
    attr: value
  plug2:
    interface: test-interface
    attr2: value2
slots:
  slot:
    interface: ssh-agent
`

func (s *sdkStateSuite) SetUpTest(c *check.C) {
	var err error
	dirs.SetRootDir(c.MkDir())
	dirs.SetCacheDir(c.MkDir())
	c.Assert(dirs.CreateDirs(), check.IsNil)

	s.fs = afero.NewMemMapFs()
	ctx := context.WithValue(context.TODO(), workshop.ContextProjectId, "projectId")
	s.ctx = context.WithValue(ctx, workshop.ContextUser, "testuser")

	s.backend, err = fakebackend.New(c.MkDir())
	c.Check(err, check.IsNil)

	s.project = workshop.Project{
		Path:      c.MkDir(),
		ProjectId: "projectId",
	}
	s.restoreProjectId = testutil.FakeFunc(func() (string, error) { return s.project.ProjectId, nil }, &workshop.NewProjectId)
	s.backend.CreateOrLoadProject(s.ctx, s.project.Path)

	s.state = state.New(nil)
	s.runner = state.NewTaskRunner(s.state)

	workshop.ReplaceBackend(s.state, s.backend)

	/* empty task handler */
	s.runner.AddHandler("fake-task", fakeHandler, nil)

	s.repo = interfaces.NewRepository()
	mockIface(c, s.repo, &ifacetest.TestInterface{InterfaceName: "test-interface"})
	s.sdkmgr = sdkstate.New(s.state, s.runner, s.repo)

	/* error-provoking task handler */
	erroringHandler := func(task *state.Task, _ *tomb.Tomb) error {
		return ErrTrigger
	}
	retryHandler := func(task *state.Task, _ *tomb.Tomb) error {
		// to keep the change not ready
		return &state.Retry{After: 1 * time.Hour}
	}
	s.runner.AddHandler("error-trigger", erroringHandler, nil)
	s.runner.AddHandler("retry-task", retryHandler, nil)

	s.se = overlord.NewStateEngine(s.state)
	s.se.StartUp()
	s.se.AddManager(s.sdkmgr)
	s.se.AddManager(s.runner)

	s.installTime = time.Date(2023, 04, 25, 1, 2, 3, 0, time.UTC)
	s.restoreInstallTime = testutil.FakeFunc(func() time.Time { return s.installTime }, &workshop.InstallTimeNow)

	wf := &workshop.File{Name: "ws", Base: "ubuntu@20.04", Sdks: []workshop.SdkRecord{
		{Name: "test", Channel: "latest/stable"},
		{Name: "test-broken", Channel: "latest/stable"},
	}}
	err = s.backend.LaunchOrRebuildWorkshop(s.ctx, wf)
	c.Assert(err, check.IsNil)
}

func (s *sdkStateSuite) mockSdk(c *check.C, name, sdkYaml string, rev sdk.Revision) {
	vfs := c.MkDir()

	meta := filepath.Join(vfs, "meta")
	err := os.MkdirAll(meta, 0755)
	c.Assert(err, check.IsNil)
	err = os.WriteFile(filepath.Join(meta, "sdk.yaml"), []byte(sdkYaml), 0644)
	c.Assert(err, check.IsNil)
	volume := workshop.VolumeInfo{
		Name:     sdk.VolumeName(name, rev),
		Kind:     "sdk",
		Sdk:      name,
		Revision: rev,
		Metadata: sdkYaml,
	}
	file, err := os.Open(vfs)
	c.Assert(err, check.IsNil)
	defer file.Close()
	err = s.backend.ImportVolume(s.ctx, volume, file)
	c.Assert(err, check.IsNil)
}

func mockIface(c *check.C, repo *interfaces.Repository, iface interfaces.Interface) {
	err := repo.AddInterface(iface)
	c.Assert(err, check.IsNil)
}

func (s *sdkStateSuite) TearDownTest(c *check.C) {
	s.restoreProjectId()
	s.restoreInstallTime()
}

func (s *sdkStateSuite) TestDoInstallSdkSuccess(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	defer sdk.MockSanitizePlugsSlots(func(sdkInfo *sdk.Info) {})()

	newSdk := sdk.Setup{Name: "test", Channel: "latest/stable", Revision: sdk.Revision{N: 2}, InstallTime: &s.installTime}
	s.mockSdk(c, "test", sdkYaml, sdk.R(2))

	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", newSdk)
	t1 := s.state.NewTask("install-sdk", "test")
	t1.Set("sdk-retrieve-task", t.ID())

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t, t1)
	chg.Set("user", "testuser")
	chg.AddTask(t1)
	chg.AddTask(t)

	s.state.Unlock()
	s.se.Ensure()
	s.se.Wait()
	s.state.Lock()

	c.Check(chg.Err(), check.IsNil)
	c.Check(chg.Status(), check.Equals, state.DoneStatus)

	c.Assert(s.backend.WorkshopVolumeMountPoints, check.HasLen, 1)
	c.Assert(s.backend.WorkshopVolumeMountPoints["test-2"], check.Equals, "/var/lib/workshop/sdk/test")

	props, err := s.backend.Workshop(s.ctx, "ws")
	c.Assert(err, check.IsNil)
	info := props.Sdks
	c.Check(info["test"], check.DeepEquals, newSdk)

	sdkInfo, err := props.SdkInfo(s.ctx, "test")
	c.Assert(err, check.IsNil)
	c.Assert(sdkInfo.Plugs, check.HasLen, 2)
	c.Assert(sdkInfo.Slots, check.HasLen, 0)
}

func (s *sdkStateSuite) TestUndoInstallSdkSuccess(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	newSdk := sdk.Setup{Name: "test-2", Channel: "latest/stable", Revision: sdk.Revision{N: 2}, InstallTime: &s.installTime}
	s.mockSdk(c, "test-2", sdkYaml, sdk.R(2))

	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", newSdk)

	t1 := s.state.NewTask("install-sdk", "test")
	t1.Set("sdk-retrieve-task", t.ID())
	t1.WaitFor(t)

	terr := s.state.NewTask("error-trigger", "provoking total undo")
	terr.WaitFor(t1)

	chg := s.state.NewChange("sample", "...")
	chg.Set("workshop", "ws")
	chg.Set("project-id", s.project.ProjectId)
	chg.Set("user", "testuser")
	chg.AddTask(t)
	chg.AddTask(t1)
	chg.AddTask(terr)

	setWorkshopProject("ws", s.project, t, t1, terr)

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		_ = s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	c.Check(t1.Status(), check.Equals, state.UndoneStatus)

	c.Assert(s.backend.WorkshopVolumeMountPoints, check.HasLen, 0)
}

func (s *sdkStateSuite) TestRetrieveSystemSdkSuccess(c *check.C) {
	sdk.ReplaceStore(s.state, sdk.NewFakeStore())

	s.state.Lock()
	defer s.state.Unlock()

	newSdk := sdk.Setup{Name: sdk.System.String(), Source: sdk.SystemSource, Revision: system.SystemSdkRevision}
	t := s.state.NewTask("retrieve-sdk", "retrieve")
	t.Set("sdk-setup", newSdk)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t)
	chg.Set("user", "testuser")
	chg.AddTask(t)

	s.state.Unlock()
	c.Assert(s.se.Ensure(), check.IsNil)
	s.se.Wait()
	s.state.Lock()

	c.Check(chg.Err(), check.IsNil)

	f, err := os.Open(newSdk.Filepath())
	c.Assert(err, check.IsNil)
	defer f.Close()

	tr := tar.NewReader(f)
	var entries []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		c.Assert(err, check.IsNil)

		info := hdr.FileInfo()
		if info.IsDir() {
			continue
		}

		entry := fmt.Sprintf("%s %s", info.Mode().Perm(), hdr.Name)
		entries = append(entries, entry)

		r, err := system.SystemSdkFs.Open(hdr.Name)
		c.Assert(err, check.IsNil)
		c.Check(osutil.StreamsEqual(r, tr), check.Equals, true)
		r.Close()
	}

	c.Check(entries, check.DeepEquals, []string{"-r--r--r-- meta/sdk.yaml"})
}

func (s *sdkStateSuite) TestDoRegisterSdkSuccess(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	defer sdk.MockSanitizePlugsSlots(func(sdkInfo *sdk.Info) {})()

	testSdk := sdk.Setup{Name: "test", Channel: "latest/stable", Revision: sdk.Revision{N: 1}, InstallTime: &s.installTime}
	s.mockSdk(c, "test", sdkYaml, sdk.R(1))

	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", testSdk)
	t1 := s.state.NewTask("install-sdk", "test")
	t1.Set("sdk-retrieve-task", t.ID())
	t2 := s.state.NewTask("register-sdk", "test")
	t2.Set("sdk-retrieve-task", t.ID())
	t2.WaitFor(t1)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t, t1, t2)
	chg.Set("user", "testuser")
	chg.AddTask(t2)
	chg.AddTask(t1)
	chg.AddTask(t)

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		_ = s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	c.Check(chg.Err(), check.Equals, nil)

	c.Assert(s.repo.Plugs(s.project.ProjectId, "ws", "test"), check.HasLen, 2)
	c.Assert(s.repo.Plug(s.project.ProjectId, "ws", "test", "plug"), check.NotNil)
	c.Assert(s.repo.Plug(s.project.ProjectId, "ws", "test", "plug2"), check.NotNil)
}

func (s *sdkStateSuite) TestDoRegisterSdkFailedPolicyCheck(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	defer sdk.MockSanitizePlugsSlots(func(sdkInfo *sdk.Info) {})()
	s.mockSdk(c, "test-broken", sdkYamlViolatesPolicy, sdk.R(2))

	testSdk := sdk.Setup{Name: "test-broken", Channel: "latest/stable", Revision: sdk.Revision{N: 2}, InstallTime: &s.installTime}

	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", testSdk)
	t1 := s.state.NewTask("install-sdk", "...")
	t1.Set("sdk-retrieve-task", t.ID())
	t2 := s.state.NewTask("register-sdk", "test-broken")
	t2.Set("sdk-retrieve-task", t.ID())
	t2.WaitFor(t1)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, t, t1, t2)
	chg.Set("user", "testuser")
	chg.AddTask(t2)
	chg.AddTask(t1)
	chg.AddTask(t)

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	c.Assert(chg.Err(), check.ErrorMatches, `(?s).*installation not allowed by "slot" slot rule of interface "ssh-agent".*`)

	// not in the fs (removed)
	wfs, err := s.backend.WorkshopFs(s.ctx, "ws")
	c.Assert(err, check.IsNil)
	_, err = wfs.Stat(sdk.SdkDir("test-broken"))
	c.Check(osutil.IsDirNotExist(err), check.Equals, true)

	// not in the SDK list (unlinked)
	wp, err := s.backend.Workshop(s.ctx, "ws")
	c.Assert(err, check.IsNil)
	_, ok := wp.Sdks["test-broken"]
	c.Check(ok, check.Equals, false)

	// not in the repo (removed)
	c.Check(s.repo.Plugs(s.project.ProjectId, "ws", "test"), check.HasLen, 0)
	c.Check(s.repo.Slots(s.project.ProjectId, "ws", "test"), check.HasLen, 0)
}

func (s *sdkStateSuite) TestDoUnregisterSdk(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	defer sdk.MockSanitizePlugsSlots(func(sdkInfo *sdk.Info) {})()
	s.mockSdk(c, "test", sdkYaml, sdk.R(1))

	newSdk := sdk.Setup{Name: "test", Revision: sdk.Revision{N: 1}}
	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", newSdk)
	t1 := s.state.NewTask("install-sdk", "...")
	t1.Set("sdk-retrieve-task", t.ID())
	register := s.state.NewTask("register-sdk", "test")
	register.Set("sdk-retrieve-task", t.ID())
	register.WaitFor(t1)

	terr := s.state.NewTask("error-trigger", "provoking total undo")
	terr.WaitFor(register)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, register, t, t1)

	chg.Set("user", "testuser")
	chg.AddTask(register)
	chg.AddTask(t)
	chg.AddTask(terr)
	chg.AddTask(t1)

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()
	c.Assert(chg.Err(), check.ErrorMatches, `(?s).*provoking total undo \(error out\)`)

	props, err := s.backend.Workshop(s.ctx, "ws")
	c.Assert(err, check.IsNil)
	_, ok := props.Sdks["test"]
	c.Check(ok, check.Equals, false)
	c.Check(register.Status(), check.Equals, state.UndoneStatus)

	c.Assert(s.repo.Plugs(s.project.ProjectId, "ws", "test"), check.HasLen, 0)
	c.Assert(s.repo.Plug(s.project.ProjectId, "ws", "test", "plug"), check.IsNil)
	c.Assert(s.repo.Plug(s.project.ProjectId, "ws", "test", "plug2"), check.IsNil)
}

func (s *sdkStateSuite) TestDoRegisterSdkBadInterfacesFound(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	s.mockSdk(c, "test", sdkYaml, sdk.R(1))

	newSdk := sdk.Setup{Name: "test", Revision: sdk.Revision{N: 1}}
	t := s.state.NewTask("fake-task", "retrieve")
	t.Set("sdk-setup", newSdk)
	t1 := s.state.NewTask("install-sdk", "...")
	t1.Set("sdk-retrieve-task", t.ID())
	register := s.state.NewTask("register-sdk", "test")
	register.Set("sdk-retrieve-task", t.ID())
	register.WaitFor(t1)

	chg := s.state.NewChange("sample", "...")
	setWorkshopProject("ws", s.project, register, t, t1)

	chg.Set("user", "testuser")
	chg.AddTask(register)
	chg.AddTask(t)
	chg.AddTask(t1)

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()

	c.Assert(chg.Err(), check.ErrorMatches, `(?s).*"test" SDK has bad plugs or slots: plug, plug2 \(unknown interface "test-interface"\).*`)

	c.Assert(s.repo.Plugs(s.project.ProjectId, "ws", "test"), check.HasLen, 0)
	c.Assert(s.repo.Plug(s.project.ProjectId, "ws", "test", "plug"), check.IsNil)
	c.Assert(s.repo.Plug(s.project.ProjectId, "ws", "test", "plug2"), check.IsNil)
}

func (s *sdkStateSuite) TestSDKVolumeRemovedAfterCooldownOK(c *check.C) {
	s.state.Lock()
	s.mockSdk(c, "test", sdkYaml, sdk.R(1))
	newSdk := sdk.Setup{Name: "test", Revision: sdk.Revision{N: 1}, Source: sdk.StoreSource}
	t := s.state.NewTask("unregister-sdk", "...")
	t.Set("sdk-retrieve-task", t.ID())
	t.Set("sdk-setup", newSdk)

	chg := s.state.NewChange("sample", "...")
	chg.Set("user", "testuser")
	setWorkshopProject("ws", s.project, t)
	chg.AddTask(t)
	s.state.Unlock()

	defer sdkstate.FakeSdkVolumeCooldownTime(0)()

	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}

	s.state.Lock()
	defer s.state.Unlock()
	c.Check(chg.Err(), check.IsNil)
	_, err := s.backend.Volume(s.ctx, sdk.VolumeName("test", sdk.R(1)))
	c.Assert(err, check.Equals, workshop.ErrVolumeNotFound)
	c.Assert(t.IsClean(), check.Equals, true)
}

func (s *sdkStateSuite) TestSDKVolumeNotRemovedBeforeCooldown(c *check.C) {
	s.state.Lock()
	s.mockSdk(c, "test", sdkYaml, sdk.R(1))
	newSdk := sdk.Setup{Name: "test", Revision: sdk.Revision{N: 1}, Source: sdk.StoreSource}
	t := s.state.NewTask("unregister-sdk", "...")
	t.Set("sdk-retrieve-task", t.ID())
	t.Set("sdk-setup", newSdk)

	chg := s.state.NewChange("sample", "...")
	chg.Set("user", "testuser")
	setWorkshopProject("ws", s.project, t)
	chg.AddTask(t)
	s.state.Unlock()

	// Set cooldown to a large value so it never passes
	defer sdkstate.FakeSdkVolumeCooldownTime(24 * time.Hour)()

	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}

	s.state.Lock()
	defer s.state.Unlock()
	// The volume should still exist
	_, err := s.backend.Volume(s.ctx, sdk.VolumeName("test", sdk.R(1)))
	c.Assert(err, check.IsNil)
	// The task should not be clean (cleanup not performed)
	c.Assert(t.IsClean(), check.Equals, false)
}

func (s *sdkStateSuite) TestTaskQuitsCleanupIfOtherVolumeUsersExist(c *check.C) {
	s.state.Lock()
	s.mockSdk(c, "test", sdkYaml, sdk.R(1))
	newSdk := sdk.Setup{Name: "test", Revision: sdk.Revision{N: 1}, Source: sdk.StoreSource}
	t := s.state.NewTask("unregister-sdk", "...")
	t.Set("sdk-retrieve-task", t.ID())
	t.Set("sdk-setup", newSdk)

	chg := s.state.NewChange("sample", "...")
	chg.Set("user", "testuser")
	setWorkshopProject("ws", s.project, t)
	chg.AddTask(t)

	other := s.state.NewChange("refresh", "...")
	other.Set("user", "testuser")
	t2 := s.state.NewTask("unregister-sdk", "t2")
	t2.Set("sdk-retrieve-task", t2.ID())
	t2.Set("sdk-setup", newSdk)
	retry := s.state.NewTask("retry-task", "...")
	retry.WaitFor(t2)
	setWorkshopProject("ws", s.project, t2, retry)
	other.AddTask(t2)
	other.AddTask(retry)

	s.state.Unlock()

	defer sdkstate.FakeSdkVolumeCooldownTime(0)()

	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}

	s.state.Lock()
	defer s.state.Unlock()
	_, err := s.backend.Volume(s.ctx, sdk.VolumeName("test", sdk.R(1)))
	c.Assert(err, check.IsNil)
	c.Assert(t.IsClean(), check.Equals, true)
	c.Assert(t2.IsClean(), check.Equals, false)
}

func (s *sdkStateSuite) TestTaskRetriesCleanupIfOtherVolumeUsersExist(c *check.C) {
	s.state.Lock()
	s.mockSdk(c, "test", sdkYaml, sdk.R(1))
	newSdk := sdk.Setup{Name: "test", Revision: sdk.Revision{N: 1}, Source: sdk.StoreSource}
	t := s.state.NewTask("unregister-sdk", "...")
	t.Set("sdk-retrieve-task", t.ID())
	t.Set("sdk-setup", newSdk)

	chg := s.state.NewChange("sample", "...")
	chg.Set("user", "testuser")
	setWorkshopProject("ws", s.project, t)
	chg.AddTask(t)

	other := s.state.NewChange("launch", "...")
	other.Set("user", "testuser")
	t2 := s.state.NewTask("install-sdk", "t2")
	t2.Set("sdk-retrieve-task", t2.ID())
	t2.Set("sdk-setup", newSdk)
	t2.SetToWait(state.DoStatus)
	setWorkshopProject("ws", s.project, t2)
	other.AddTask(t2)

	s.state.Unlock()

	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}

	s.state.Lock()

	_, err := s.backend.Volume(s.ctx, sdk.VolumeName("test", sdk.R(1)))
	c.Check(err, check.IsNil)
	c.Check(t.IsClean(), check.Equals, false)

	// Finish the launch change that would enable the t cleanup to finish.
	waited := t2.WaitedStatus()
	t2.SetStatus(waited)
	defer sdkstate.FakeSdkVolumeCooldownTime(0)()

	s.state.Unlock()

	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()
	defer s.state.Unlock()
	c.Check(t.IsClean(), check.Equals, true)
	_, err = s.backend.Volume(s.ctx, sdk.VolumeName("test", sdk.R(1)))
	c.Assert(err, check.Equals, workshop.ErrVolumeNotFound)
}

func (s *sdkStateSuite) TestSDKVolumeCleanupWithLaterUnregisterTask(c *check.C) {
	s.state.Lock()
	s.mockSdk(c, "test", sdkYaml, sdk.R(1))
	newSdk := sdk.Setup{Name: "test", Revision: sdk.Revision{N: 1}, Source: sdk.StoreSource}

	t1 := s.state.NewTask("unregister-sdk", "t1")
	t1.Set("sdk-retrieve-task", t1.ID())
	t1.Set("sdk-setup", newSdk)

	t2 := s.state.NewTask("unregister-sdk", "t2")
	t2.Set("sdk-retrieve-task", t2.ID())
	t2.Set("sdk-setup", newSdk)
	t2.WaitFor(t1)

	// Add both tasks to their own changes
	chg1 := s.state.NewChange("launch", "chg1")
	chg1.Set("user", "testuser")
	setWorkshopProject("ws", s.project, t1)
	chg1.AddTask(t1)

	chg2 := s.state.NewChange("refresh", "chg2")
	chg2.Set("user", "testuser")
	setWorkshopProject("ws", s.project, t2)
	chg2.AddTask(t2)

	s.state.Unlock()

	// Use default cooldown (1h), so t2 will not be clean (cooldown not passed)
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}

	s.state.Lock()
	defer s.state.Unlock()

	// The first task should be clean (cleanup skipped due to newer task)
	c.Assert(t1.IsClean(), check.Equals, true)
	// The second task should not be clean (cleanup not performed, cooldown not passed)
	c.Assert(t2.IsClean(), check.Equals, false)
	_, err := s.backend.Volume(s.ctx, sdk.VolumeName("test", sdk.R(1)))
	c.Assert(err, check.IsNil)
}

func (s *sdkStateSuite) TestSDKVolumeCleanupWithDifferentRevisions(c *check.C) {
	s.state.Lock()
	s.mockSdk(c, "test", sdkYaml, sdk.R(1))
	s.mockSdk(c, "test", sdkYaml, sdk.R(2))
	sdk1 := sdk.Setup{Name: "test", Revision: sdk.Revision{N: 1}, Source: sdk.StoreSource}
	sdk2 := sdk.Setup{Name: "test", Revision: sdk.Revision{N: 2}, Source: sdk.StoreSource}

	t1 := s.state.NewTask("unregister-sdk", "t1")
	t1.Set("sdk-retrieve-task", t1.ID())
	t1.Set("sdk-setup", sdk1)
	chg1 := s.state.NewChange("refresh", "chg1")
	chg1.Set("user", "testuser")
	setWorkshopProject("ws", s.project, t1)
	chg1.AddTask(t1)

	t2 := s.state.NewTask("unregister-sdk", "t2")
	t2.Set("sdk-retrieve-task", t2.ID())
	t2.Set("sdk-setup", sdk2)
	t3 := s.state.NewTask("retry-task", "t3")
	t3.WaitFor(t2)
	chg2 := s.state.NewChange("refresh", "chg2")
	chg2.Set("user", "testuser")
	setWorkshopProject("ws", s.project, t2, t3)
	chg2.AddTask(t2)
	chg2.AddTask(t3)

	defer sdkstate.FakeSdkVolumeCooldownTime(0)()

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()
	defer s.state.Unlock()
	c.Assert(t1.IsClean(), check.Equals, true)
	c.Assert(t2.IsClean(), check.Equals, false)
	// The volume should still exist
	_, err := s.backend.Volume(s.ctx, sdk.VolumeName("test", sdk.R(2)))
	c.Assert(err, check.IsNil)
}

func (s *sdkStateSuite) TestSDKVolumeCleanupWithSameRevisionDifferentSource(c *check.C) {
	s.state.Lock()
	s.mockSdk(c, "test", sdkYaml, sdk.R(1))
	sdkStore := sdk.Setup{Name: "test", Revision: sdk.Revision{N: 1}, Source: sdk.TrySource}
	sdkProject := sdk.Setup{Name: "test", Revision: sdk.Revision{N: 1}, Source: sdk.ProjectSource}

	t1 := s.state.NewTask("unregister-sdk", "t1")
	t1.Set("sdk-retrieve-task", t1.ID())
	t1.Set("sdk-setup", sdkStore)
	chg1 := s.state.NewChange("launch", "chg1")
	chg1.Set("user", "testuser")
	setWorkshopProject("ws", s.project, t1)
	chg1.AddTask(t1)

	t2 := s.state.NewTask("unregister-sdk", "t2")
	t2.Set("sdk-retrieve-task", t2.ID())
	t2.Set("sdk-setup", sdkProject)
	t2.WaitFor(t1)
	t3 := s.state.NewTask("retry-task", "t3")
	t3.WaitFor(t2)
	chg2 := s.state.NewChange("refresh", "chg2")
	chg2.Set("user", "testuser")
	setWorkshopProject("ws", s.project, t2, t3)
	chg2.AddTask(t2)
	chg2.AddTask(t3)

	defer sdkstate.FakeSdkVolumeCooldownTime(0)()

	s.state.Unlock()
	for i := 0; i < 6; i = i + 1 {
		s.se.Ensure()
		s.se.Wait()
	}
	s.state.Lock()
	defer s.state.Unlock()
	c.Assert(t1.IsClean(), check.Equals, true)
	// t2's change is not ready, so not clean.
	c.Assert(t2.IsClean(), check.Equals, false)
	_, err := s.backend.Volume(s.ctx, sdk.VolumeName("test", sdk.R(1)))
	c.Assert(err, check.Equals, workshop.ErrVolumeNotFound)
}
