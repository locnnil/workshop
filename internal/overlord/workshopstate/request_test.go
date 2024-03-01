package workshopstate_test

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/workshopstate"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"

	"gopkg.in/check.v1"
)

type requestSuite struct {
	state   *state.State
	project *workshopbackend.Project
	backend workshopbackend.WorkshopBackend
	mgr     *workshopstate.WorkshopManager
	ctx     context.Context
}

var _ = check.Suite(&requestSuite{})

func Test(t *testing.T) { check.TestingT(t) }

func (s *requestSuite) SetUpTest(c *check.C) {
	s.state = state.New(nil)
	s.ctx = context.WithValue(context.Background(), workshopbackend.ContextUser, "testuser")

	s.backend = workshopbackend.NewFakeWorkshopBackend()
	s.mgr = workshopstate.New(s.state, state.NewTaskRunner(s.state), s.backend)
	s.project, _, _ = s.backend.CreateOrLoadProject(s.ctx, c.MkDir())
	s.ctx = context.WithValue(s.ctx, workshopbackend.ContextProjectId, s.project.ProjectId)
}

var workshopTemplate = `name: %s
base: ubuntu@20.04
sdks:
  {{ range . }}
  {{- .Name}}:
      channel: {{.Channel}}
  {{ end }} 
`

func (s *requestSuite) launchWorkshopWithSDKs(c *check.C, ws string, sdks []sdk.Setup) {
	t, err := template.New("workshop").Parse(fmt.Sprintf(workshopTemplate, ws))
	c.Assert(err, check.IsNil)

	var workshopFile = bytes.NewBuffer([]byte{})
	t.Execute(workshopFile, sdks)

	err = os.WriteFile(filepath.Join(s.project.Path, fmt.Sprintf(".workshop.%s.yaml", ws)), workshopFile.Bytes(), 0644)
	c.Assert(err, check.IsNil)

	err = s.backend.LaunchWorkshop(s.ctx, ws, "ubuntu@20.04")
	c.Assert(err, check.IsNil)
}

func (s *requestSuite) ensureTaskHasWorkshopAndProjectKeys(c *check.C, w string, ts []*state.Task) {
	for _, i := range ts {
		var prj workshopbackend.Project
		err := i.Get("project", &prj)
		c.Assert(err, check.IsNil)
		c.Assert(&prj, check.DeepEquals, s.project)

		var workshop string
		err = i.Get("workshop", &workshop)
		c.Assert(err, check.IsNil)
		c.Assert(workshop, check.Equals, w)
	}
}

func verifyExpectedTasks(c *check.C, ts []*state.Task, expected []string) {
	actual := make([]string, 0, len(ts))
	for _, i := range ts {
		actual = append(actual, i.Kind())
	}

	c.Assert(actual, testutil.DeepUnsortedMatches, expected)
}

func verifyDisconnectDependencies(c *check.C, ts *state.TaskSet) {
	prev := (*state.Task)(nil)
	for i, t := range ts.Tasks() {
		if t.Kind() == "disconnect" {
			if prev != nil {
				c.Assert(t.WaitTasks(), testutil.DeepUnsortedMatches, ts.Tasks()[i-1])
			}
		}
	}
}

func validateStateHooksTasksSetup(c *check.C, ts []*state.TaskSet, expectedSave, expectedRestore []string) {
	obtainedSave := []string{}
	obtainedRestore := []string{}
	for _, t := range ts[0].Tasks() {
		if t.Kind() == "run-hook" {
			var setup hookstate.HookSetup
			err := t.Get("hook-setup", &setup)
			c.Assert(err, check.IsNil)
			switch setup.HookType {
			case hookstate.SaveState:
				obtainedSave = append(obtainedSave, setup.Sdk)
			case hookstate.RestoreState:
				obtainedRestore = append(obtainedRestore, setup.Sdk)
			}
		}
	}

	// the save state shall be called only for the previously installed SDK
	c.Assert(obtainedSave, testutil.DeepUnsortedMatches, expectedSave)
	// the restore state shall be called for the new previously installed SDK
	c.Assert(obtainedRestore, testutil.DeepUnsortedMatches, expectedRestore)
}

func (s *requestSuite) TestLaunchWorkshopNoSdk(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	file := &workshopbackend.WorkshopFile{Name: "test", Base: "ubuntu@22.04"}
	ts := workshopstate.Launch(s.state, file, s.project)

	expected := []string{"create-workshop",
		"mount-project",
		"start-workshop"}
	tasks := ts.Tasks()

	verifyExpectedTasks(c, tasks, expected)

	var base string
	err := tasks[0].Get("base", &base)
	c.Assert(err, check.Equals, nil)
	c.Assert(base, check.Equals, "ubuntu@22.04")
	s.ensureTaskHasWorkshopAndProjectKeys(c, "test", ts.Tasks())
}

func (s *requestSuite) TestLaunchWorkshopWithSdks(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	sdk := workshopbackend.SdkRecord{Name: "sdk", Channel: "latest/stable"}
	sdk_2 := workshopbackend.SdkRecord{Name: "sdk_2", Channel: "latest/stable"}

	file := &workshopbackend.WorkshopFile{
		Name: "test",
		Base: "ubuntu@22.04",
		Sdks: workshopbackend.SdkList{sdk, sdk_2}}

	ts := workshopstate.Launch(s.state, file, s.project)

	expected := []string{
		"create-workshop",
		"mount-project",
		"start-workshop",
		"retrieve-sdk",
		"retrieve-sdk",
		"install-sdk",
		"install-sdk",
		"link-sdk",
		"link-sdk",
		"auto-connect",
		"auto-connect",
		"run-hook",
		"run-hook",
		"run-hook",
		"run-hook"}

	tasks := ts.Tasks()

	verifyExpectedTasks(c, tasks, expected)
	var err error

	var s1, s2 workshopbackend.SdkRecord
	err = tasks[0].Get("sdk-record", &s1)
	c.Assert(err, check.Equals, nil)
	c.Assert(s1, check.Equals, sdk)

	err = tasks[1].Get("sdk-record", &s2)
	c.Assert(err, check.Equals, nil)
	c.Assert(s2, check.Equals, sdk_2)

	// install-sdk task for sdk
	var id1, id2 string
	err = tasks[5].Get("sdk-retrieve-task", &id1)
	c.Assert(err, check.Equals, nil)
	c.Assert(id1, check.Equals, tasks[0].ID())

	// link-sdk task for sdk
	err = tasks[6].Get("sdk-retrieve-task", &id2)
	c.Assert(err, check.Equals, nil)
	c.Assert(id2, check.Equals, tasks[0].ID())

	// install-sdk task for sdk_2
	err = tasks[8].Get("sdk-retrieve-task", &id1)
	c.Assert(err, check.Equals, nil)
	c.Assert(id1, check.Equals, tasks[1].ID())

	// link-sdk task for sdk_2
	err = tasks[8].Get("sdk-retrieve-task", &id2)
	c.Assert(err, check.Equals, nil)
	c.Assert(id2, check.Equals, tasks[1].ID())

	s.ensureTaskHasWorkshopAndProjectKeys(c, "test", tasks)
}

func (s *requestSuite) TestRefreshEmptyWorkshop(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	file := &workshopbackend.WorkshopFile{
		Name: "ws",
		Base: "ubuntu@22.04",
		Sdks: workshopbackend.SdkList{}}

	ts, err := workshopstate.Refresh(s.state, file, []sdk.Setup{}, s.project)
	c.Assert(err, check.IsNil)

	expected := []string{
		"create-state-storage",
		"remove-state-storage",
		"create-workshop",
		"remove-workshop-stash",
		"stash-workshop",
		"mount-project",
		"start-workshop",
	}

	tasks := ts.Tasks()

	c.Assert(err, check.Equals, nil)
	verifyExpectedTasks(c, tasks, expected)

	s.ensureTaskHasWorkshopAndProjectKeys(c, "ws", tasks)
}

func (s *requestSuite) TestRefreshWorkshopWithSdks(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	file := &workshopbackend.WorkshopFile{
		Name: "ws",
		Base: "ubuntu@22.04",
		Sdks: workshopbackend.SdkList{{Name: "sdk-1", Channel: "latest/stable"}, {Name: "sdk-2", Channel: "latest/stable"}}}

	ts, err := workshopstate.Refresh(s.state, file, []sdk.Setup{
		{Name: "sdk-1", Channel: "latest/stable"},
		{Name: "sdk-2", Channel: "latest/stable"},
	}, s.project)
	c.Assert(err, check.IsNil)

	expected := []string{
		"retrieve-sdk",
		"retrieve-sdk",
		"create-state-storage",
		"run-hook", // save-state sdk-1
		"run-hook", // save-state sdk-2
		"disconnect",
		"disconnect",
		"stash-workshop",
		"create-workshop",
		"mount-project",
		"start-workshop",
		"install-sdk",
		"link-sdk",
		"install-sdk",
		"link-sdk",
		"auto-connect",
		"auto-connect",
		"run-hook", // setup-base sdk-1
		"run-hook", // setup-base sdk-2
		"run-hook", // restore-state sdk-1
		"run-hook", // restore-state sdl-2
		"run-hook", // check-health sdk-1
		"run-hook", // check-health sdk-2
		"remove-workshop-stash",
		"remove-state-storage",
	}

	tasks := ts.Tasks()

	c.Assert(err, check.Equals, nil)
	verifyExpectedTasks(c, tasks, expected)
	verifyDisconnectDependencies(c, ts)

	s.ensureTaskHasWorkshopAndProjectKeys(c, "ws", tasks)
}

func (s *requestSuite) TestRefreshSdkRemoved(c *check.C) {
	// Setup
	s.state.Lock()
	defer s.state.Unlock()
	existingSdks := []sdk.Setup{
		{Name: "sdk-1", Channel: "latest/stable"}, {Name: "sdk-2", Channel: "latest/stable"},
	}

	file := &workshopbackend.WorkshopFile{
		Name: "ws",
		Base: "ubuntu@22.04",
		Sdks: workshopbackend.SdkList{{Name: "sdk-1", Channel: "latest/stable"}}}

	s.launchWorkshopWithSDKs(c, "ws", existingSdks)

	// Execute
	ts, err := workshopstate.Refresh(s.state, file, existingSdks, s.project)
	c.Assert(err, check.IsNil)

	// Validate
	// save-state and restore-state will only exist for sdk-1
	validateStateHooksTasksSetup(c, []*state.TaskSet{ts}, []string{"sdk-1"}, []string{"sdk-1"})
	verifyDisconnectDependencies(c, ts)
}

func (s *requestSuite) TestRefreshSdkRemovedMakingWorkshopEmpty(c *check.C) {
	// Setup
	s.state.Lock()
	defer s.state.Unlock()
	existingSdks := []sdk.Setup{
		{Name: "sdk-1", Channel: "latest/stable"}, {Name: "sdk-2", Channel: "latest/stable"},
	}

	file := &workshopbackend.WorkshopFile{
		Name: "ws",
		Base: "ubuntu@22.04"}

	s.launchWorkshopWithSDKs(c, "ws", existingSdks)

	// Execute
	ts, err := workshopstate.Refresh(s.state, file, existingSdks, s.project)
	c.Assert(err, check.IsNil)

	// Validate
	// save-state and restore-state will only exist for sdk-1
	validateStateHooksTasksSetup(c, []*state.TaskSet{ts}, []string{}, []string{})
	verifyDisconnectDependencies(c, ts)
}

func (s *requestSuite) TestRefreshSdkReplaced(c *check.C) {
	// Setup
	s.state.Lock()
	defer s.state.Unlock()
	existingSdks := []sdk.Setup{
		{Name: "sdk-1", Channel: "latest/stable"}, {Name: "sdk-2", Channel: "latest/stable"},
	}

	file := &workshopbackend.WorkshopFile{
		Name: "ws",
		Base: "ubuntu@22.04",
		Sdks: workshopbackend.SdkList{{Name: "test-1", Channel: "latest/stable"}, {Name: "test-2", Channel: "latest/stable"}}}

	s.launchWorkshopWithSDKs(c, "ws", existingSdks)

	// Execute
	ts, err := workshopstate.Refresh(s.state, file, existingSdks, s.project)
	c.Assert(err, check.IsNil)

	// Validate
	// save-state and restore-state will only exist for sdk-1
	validateStateHooksTasksSetup(c, []*state.TaskSet{ts}, nil, nil)
	verifyDisconnectDependencies(c, ts)
}

func (s *requestSuite) TestRefreshSdkChannelUpdated(c *check.C) {
	// Setup
	s.state.Lock()
	defer s.state.Unlock()
	existingSdks := []sdk.Setup{
		{Name: "sdk-1", Channel: "latest/stable"}, {Name: "sdk-2", Channel: "latest/stable"},
	}

	file := &workshopbackend.WorkshopFile{
		Name: "ws",
		Base: "ubuntu@22.04",
		Sdks: workshopbackend.SdkList{{Name: "sdk-1", Channel: "latest/stable"}, {Name: "sdk-2", Channel: "latest/edge"}}}

	s.launchWorkshopWithSDKs(c, "ws", existingSdks)

	// Execute
	ts, err := workshopstate.Refresh(s.state, file, existingSdks, s.project)
	c.Assert(err, check.IsNil)

	// Validate
	// save-state and restore-state will only exist for sdk-1
	validateStateHooksTasksSetup(c, []*state.TaskSet{ts}, []string{"sdk-1", "sdk-2"}, []string{"sdk-1", "sdk-2"})
	verifyDisconnectDependencies(c, ts)
}

func (s *requestSuite) TestRefreshManyOneWorkshopHasNoSdks(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	test_sdk := workshopbackend.SdkRecord{Name: "sdk", Channel: "latest/stable"}

	files := []*workshopbackend.WorkshopFile{
		{
			Name: "ws",
			Base: "ubuntu@22.04",
			Sdks: workshopbackend.SdkList{}},
		{
			Name: "ws-1",
			Base: "ubuntu@22.04",
			Sdks: workshopbackend.SdkList{test_sdk}},
	}

	content := [][]sdk.Setup{
		nil,
		{{
			Name:    "sdk",
			Channel: "latest/stable",
		},
		},
	}

	ts, err := workshopstate.RefreshManyImpl(s.state, files, content, s.project)
	c.Assert(err, check.IsNil)

	expected_ws := []string{
		"create-state-storage",
		"remove-state-storage",
		"create-workshop",
		"stash-workshop",
		"mount-project",
		"start-workshop",
		"remove-workshop-stash",
	}

	expected_ws_1 := []string{
		"retrieve-sdk",
		"create-state-storage",
		"run-hook", // save-state hook
		"disconnect",
		"stash-workshop",
		"create-workshop",
		"mount-project",
		"start-workshop",
		"install-sdk",
		"link-sdk",
		"auto-connect",
		"run-hook", // setup-base hook
		"run-hook", // restore-state hook
		"run-hook", // check-health hook
		"remove-workshop-stash",
		"remove-state-storage",
	}

	verifyExpectedTasks(c, ts[0].Tasks(), expected_ws)
	verifyExpectedTasks(c, ts[1].Tasks(), expected_ws_1)

	c.Assert(ts[0].MaybeEdge(workshopstate.EdgeLastTaskBeforeRefreshIrreversible).Kind(), check.Equals, "start-workshop")
	waitFor := ts[0].MaybeEdge(workshopstate.EdgeRefreshCleanup).WaitTasks()
	c.Assert(waitFor, testutil.DeepUnsortedMatches, []*state.Task{
		ts[0].MaybeEdge(workshopstate.EdgeLastTaskBeforeRefreshIrreversible),
		ts[1].MaybeEdge(workshopstate.EdgeLastTaskBeforeRefreshIrreversible),
	})

	c.Assert(ts[1].MaybeEdge(workshopstate.EdgeLastTaskBeforeRefreshIrreversible).Kind(), check.Equals, "run-hook")
	waitFor = ts[1].MaybeEdge(workshopstate.EdgeRefreshCleanup).WaitTasks()
	c.Assert(waitFor, testutil.DeepUnsortedMatches, []*state.Task{
		ts[0].MaybeEdge(workshopstate.EdgeLastTaskBeforeRefreshIrreversible),
		ts[1].MaybeEdge(workshopstate.EdgeLastTaskBeforeRefreshIrreversible),
	})

	for i, t := range ts {
		s.ensureTaskHasWorkshopAndProjectKeys(c, files[i].Name, t.Tasks())
	}
}

func (s *requestSuite) TestRefreshManyAllWorkshopsHaveSdks(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	testSdk := workshopbackend.SdkRecord{Name: "sdk", Channel: "latest/stable"}

	files := []*workshopbackend.WorkshopFile{
		{
			Name: "ws",
			Base: "ubuntu@22.04",
			Sdks: workshopbackend.SdkList{testSdk},
		},
		{
			Name: "ws1",
			Base: "ubuntu@22.04",
			Sdks: workshopbackend.SdkList{testSdk},
		},
	}

	content := [][]sdk.Setup{
		{{
			Name:    "sdk",
			Channel: "latest/stable",
		},
		},
		{{
			Name:    "sdk",
			Channel: "latest/stable",
		},
		},
	}

	ts, err := workshopstate.RefreshManyImpl(s.state, files, content, s.project)
	c.Assert(err, check.IsNil)

	expected := []string{
		"retrieve-sdk",
		"create-state-storage",
		"run-hook",
		"disconnect",
		"stash-workshop",
		"create-workshop",
		"mount-project",
		"start-workshop",
		"install-sdk",
		"link-sdk",
		"auto-connect",
		"run-hook", // setup-base hook
		"run-hook", // restore state hook
		"run-hook", // check-health hook
		"remove-workshop-stash",
		"remove-state-storage",
	}

	for i, t := range ts {
		c.Assert(err, check.Equals, nil)
		verifyExpectedTasks(c, t.Tasks(), expected)
		s.ensureTaskHasWorkshopAndProjectKeys(c, files[i].Name, t.Tasks())
	}
}

func (s *requestSuite) TestRefreshManyWaitsOnAllSuccessfulBeforeRemovingStash(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	testSdk := workshopbackend.SdkRecord{Name: "sdk", Channel: "latest/stable"}

	files := []*workshopbackend.WorkshopFile{
		{
			Name: "ws",
			Base: "ubuntu@22.04",
			Sdks: workshopbackend.SdkList{testSdk},
		},
		{
			Name: "ws1",
			Base: "ubuntu@22.04",
			Sdks: workshopbackend.SdkList{testSdk},
		},
	}

	content := [][]sdk.Setup{
		{{
			Name:    "sdk",
			Channel: "latest/stable",
		},
		},
		{{
			Name:    "sdk",
			Channel: "latest/stable",
		},
		},
	}

	ts, err := workshopstate.RefreshManyImpl(s.state, files, content, s.project)
	c.Assert(err, check.IsNil)

	lastChanceWs := ts[0].MaybeEdge(workshopstate.EdgeLastTaskBeforeRefreshIrreversible)
	c.Assert(lastChanceWs, check.NotNil)
	c.Assert(lastChanceWs.Kind(), check.Equals, "run-hook")

	lastChanceWs1 := ts[1].MaybeEdge(workshopstate.EdgeLastTaskBeforeRefreshIrreversible)
	c.Assert(lastChanceWs1, check.NotNil)
	c.Assert(lastChanceWs1.Kind(), check.Equals, "run-hook")

	// Ensure that the remove-stash for ws will wait on the own
	// remove-state-storage and ws1's (i.e. all the other workshops') restore
	// state hooks
	removeWsStStorage := ts[0].MaybeEdge(workshopstate.EdgeRefreshCleanup)
	removeStash := removeWsStStorage.HaltTasks()[0]
	c.Assert(removeWsStStorage.WaitTasks(), testutil.DeepUnsortedMatches, []*state.Task{lastChanceWs, lastChanceWs1})

	// Ensure that the stash and state storage removals belong to a separate
	// lane. In the case of their abortion it must not trigger abortion of the
	// refresh that, by that moment, is already done
	c.Assert(removeWsStStorage.Lanes()[0], check.Not(check.Equals), lastChanceWs.Lanes()[0])
	c.Assert(removeStash.Lanes()[0], check.Not(check.Equals), lastChanceWs.Lanes()[0])
	c.Assert(removeStash.Lanes(), check.HasLen, 1)
	c.Assert(removeWsStStorage.Lanes(), check.HasLen, 1)

	// Ensure that the delete-copy for ws1 will wait on the own
	// remove-state-storage and ws's (i.e. all the other workshops') restore
	// state hooks
	removeWsStStorage = ts[1].MaybeEdge(workshopstate.EdgeRefreshCleanup)
	removeStash = removeWsStStorage.HaltTasks()[0]
	c.Assert(removeWsStStorage.WaitTasks(), testutil.DeepUnsortedMatches, []*state.Task{lastChanceWs1, lastChanceWs})

	// Ensure that the stash and state storage removals belong to a separate
	// lane. In the case of their abortion it must not trigger abortion of the
	// refresh that, by that moment, is already done
	c.Assert(removeWsStStorage.Lanes()[0], check.Not(check.Equals), lastChanceWs1.Lanes()[0])
	c.Assert(removeStash.Lanes()[0], check.Not(check.Equals), lastChanceWs1.Lanes()[0])
	c.Assert(removeStash.Lanes(), check.HasLen, 1)
	c.Assert(removeWsStStorage.Lanes(), check.HasLen, 1)
}

func (s *requestSuite) TestRestoreStateHooks(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	oneinst := workshopbackend.SdkRecord{Name: "one", Channel: "latest/stable"}
	twoinst := workshopbackend.SdkRecord{Name: "two", Channel: "latest/stable"}

	// installed SDKs
	one := sdk.Setup{Name: "one", Channel: "latest/stable"}
	two := sdk.Setup{Name: "two", Channel: "latest/stable"}

	ts := workshopstate.RestoreStateHooks(s.state, "ws",
		[]sdk.Setup{one, two}, workshopbackend.SdkList{oneinst, twoinst})
	c.Assert(ts.Tasks(), check.HasLen, 2)

	prev := (*state.Task)(nil)
	for _, t := range ts.Tasks() {
		if prev != nil {
			c.Assert(t.WaitTasks(), check.DeepEquals, []*state.Task{prev})
		}
		prev = t
		var setup hookstate.HookSetup
		err := t.Get("hook-setup", &setup)
		c.Assert(err, check.IsNil)
		c.Assert(setup.HookType, check.Equals, hookstate.RestoreState)
	}
}

func (s *requestSuite) TestCheckHealthHooks(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	file := &workshopbackend.WorkshopFile{
		Name: "ws",
		Base: "ubuntu@22.04",
		Sdks: workshopbackend.SdkList{{Name: "one", Channel: "latest/stable"}, {Name: "two", Channel: "latest/stable"}},
	}

	ts := workshopstate.CheckHealthHooks(s.state, file)
	c.Assert(ts.Tasks(), check.HasLen, 2)

	prev := (*state.Task)(nil)
	for _, t := range ts.Tasks() {
		if prev != nil {
			c.Assert(t.WaitTasks(), check.DeepEquals, []*state.Task{prev})
		}
		prev = t

		var setup hookstate.HookSetup
		err := t.Get("hook-setup", &setup)
		c.Assert(err, check.IsNil)
		c.Assert(setup.HookType, check.Equals, hookstate.CheckHealth)
		c.Assert(setup.Timeout, check.Equals, workshopstate.CheckHealthTimeout)
	}
}

func (s *requestSuite) TestSaveStateHooks(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	// the SDK list to be refreshed (i.e. coming from a workshop file)
	installed := workshopbackend.SdkList{workshopbackend.SdkRecord{Name: "one"},
		workshopbackend.SdkRecord{Name: "two"}}

	// installed SDKs
	one := sdk.Setup{Name: "one", Channel: "latest/stable"}
	two := sdk.Setup{Name: "two", Channel: "latest/stable"}

	ts := workshopstate.SaveStateHooks(s.state, "ws", []sdk.Setup{one, two}, installed)
	c.Assert(ts.Tasks(), check.HasLen, 2)

	prev := (*state.Task)(nil)
	for _, t := range ts.Tasks() {
		if prev != nil {
			c.Assert(t.WaitTasks(), check.DeepEquals, []*state.Task{prev})
		}
		prev = t
		var setup hookstate.HookSetup
		err := t.Get("hook-setup", &setup)
		c.Assert(err, check.IsNil)
		c.Assert(setup.HookType, check.Equals, hookstate.SaveState)
	}
}

func (s *requestSuite) TestStartMany(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	ts, err := workshopstate.StartManyImpl(s.state, []string{"ws-1", "ws-2"}, s.project)
	c.Assert(err, check.IsNil)
	c.Assert(ts.Tasks(), check.HasLen, 2)
	c.Assert(ts.Tasks()[0].Kind(), check.Equals, "start-workshop")
	c.Assert(ts.Tasks()[1].Kind(), check.Equals, "start-workshop")
}

func (s *requestSuite) TestStopMany(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	ts, err := workshopstate.StopManyImpl(s.state, []string{"ws-1", "ws-2"}, s.project)
	c.Assert(err, check.IsNil)
	c.Assert(ts.Tasks(), check.HasLen, 2)
	c.Assert(ts.Tasks()[0].Kind(), check.Equals, "stop-workshop")
	c.Assert(ts.Tasks()[1].Kind(), check.Equals, "stop-workshop")

	var force bool
	ts.Tasks()[0].Get("force", &force)
	c.Assert(force, check.Equals, false)

	ts.Tasks()[1].Get("force", &force)
	c.Assert(force, check.Equals, false)
}

func (s *requestSuite) TestRemoveMany(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	content := []sdk.Setup{
		{Name: "sdk-1", Channel: "latest/stable"},
		{Name: "sdk-2", Channel: "latest/stable"},
		{Name: "sdk-3", Channel: "latest/stable"},
	}

	s.launchWorkshopWithSDKs(c, "ws-1", content)

	ts, err := s.mgr.RemoveMany(s.ctx, []string{"ws-1"}, s.project.ProjectId, "1")
	c.Assert(err, check.IsNil)
	c.Assert(ts.Tasks(), check.HasLen, 4)

	verifyDisconnectDependencies(c, ts)
	for i, t := range ts.Tasks()[0:3] {
		c.Assert(t.Kind(), check.Equals, "disconnect")
		var sdkName string
		err = t.Get("sdk", &sdkName)
		c.Assert(err, check.IsNil)
		c.Assert(sdkName, check.Equals, content[i].Name)
		c.Assert(t.Summary(), check.Equals, fmt.Sprintf("Disconnect interfaces of %q SDK", content[i].Name))
	}
	c.Assert(ts.Tasks()[3].Kind(), check.Equals, "remove-workshop")
	s.ensureTaskHasWorkshopAndProjectKeys(c, "ws-1", ts.Tasks())
}
