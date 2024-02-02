package workshopstate_test

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"testing"

	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/overlord/workshopstate"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/testutil"
	"github.com/canonical/workshop/internal/workshopbackend"

	"gopkg.in/check.v1"
)

type S struct {
	state   *state.State
	project *workshopbackend.Project
	backend workshopbackend.WorkshopBackend
	mgr     *workshopstate.WorkshopManager
	ctx     context.Context
}

var _ = check.Suite(&S{})

func Test(t *testing.T) { check.TestingT(t) }

func (s *S) SetUpTest(c *check.C) {
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

func (s *S) launchWorkshopWithSDKs(c *check.C, ws string, sdks []sdk.Setup) {
	t, err := template.New("workshop").Parse(fmt.Sprintf(workshopTemplate, ws))
	c.Assert(err, check.IsNil)

	var workshopFile = bytes.NewBuffer([]byte{})
	t.Execute(workshopFile, sdks)

	err = os.WriteFile(filepath.Join(s.project.Path, fmt.Sprintf(".workshop.%s.yaml", ws)), workshopFile.Bytes(), 0644)
	c.Assert(err, check.IsNil)

	err = s.backend.LaunchWorkshop(s.ctx, ws, "ubuntu@20.04")
	c.Assert(err, check.IsNil)
}

func (s *S) ensureTaskHasWorkshopAndProjectKeys(c *check.C, w string, ts []*state.Task) {
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

func (s *S) TestLaunchWorkshopNoSdk(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	file := &workshopbackend.WorkshopFile{Name: "test", Base: "ubuntu@22.04"}
	ts, err := workshopstate.Launch(s.state, file, s.project)

	expected := []string{"create-workshop",
		"mount-project",
		"start-workshop"}
	tasks := ts.Tasks()

	c.Assert(err, check.Equals, nil)
	verifyExpectedTasks(c, tasks, expected)

	var base string
	err = tasks[0].Get("base", &base)
	c.Assert(err, check.Equals, nil)
	c.Assert(base, check.Equals, "ubuntu@22.04")
	s.ensureTaskHasWorkshopAndProjectKeys(c, "test", ts.Tasks())
}

func (s *S) TestLaunchWorkshopWithSdks(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	sdk := workshopbackend.SdkRecord{Name: "sdk", Channel: "latest/stable"}
	sdk_2 := workshopbackend.SdkRecord{Name: "sdk_2", Channel: "latest/stable"}

	file := &workshopbackend.WorkshopFile{
		Name: "test",
		Base: "ubuntu@22.04",
		Sdks: workshopbackend.SdkList{sdk, sdk_2}}

	ts, err := workshopstate.Launch(s.state, file, s.project)

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
		"run-hook"}

	tasks := ts.Tasks()

	c.Assert(err, check.Equals, nil)
	verifyExpectedTasks(c, tasks, expected)

	var s1, s2 workshopbackend.SdkRecord
	err = tasks[3].Get("sdk-record", &s1)
	c.Assert(err, check.Equals, nil)
	c.Assert(s1, check.Equals, sdk)

	err = tasks[4].Get("sdk-record", &s2)
	c.Assert(err, check.Equals, nil)
	c.Assert(s2, check.Equals, sdk_2)

	// install-sdk task for sdk
	var id1, id2 string
	err = tasks[5].Get("sdk-retrieve-task", &id1)
	c.Assert(err, check.Equals, nil)
	c.Assert(id1, check.Equals, tasks[3].ID())

	// link-sdk task for sdk
	err = tasks[6].Get("sdk-retrieve-task", &id2)
	c.Assert(err, check.Equals, nil)
	c.Assert(id2, check.Equals, tasks[3].ID())

	// install-sdk task for sdk_2
	err = tasks[8].Get("sdk-retrieve-task", &id1)
	c.Assert(err, check.Equals, nil)
	c.Assert(id1, check.Equals, tasks[4].ID())

	// link-sdk task for sdk_2
	err = tasks[8].Get("sdk-retrieve-task", &id2)
	c.Assert(err, check.Equals, nil)
	c.Assert(id2, check.Equals, tasks[4].ID())

	s.ensureTaskHasWorkshopAndProjectKeys(c, "test", tasks)
}

func (s *S) TestRefreshEmptyWorkshop(c *check.C) {
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

func (s *S) TestRefreshManyEmptyWorkshopMany(c *check.C) {
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
		"remove-workshop-stash",
		"stash-workshop",
		"mount-project",
		"start-workshop",
	}

	expected_ws_1 := []string{
		"create-state-storage",
		"remove-state-storage",
		"create-workshop",
		"remove-workshop-stash",
		"run-hook",
		"stash-workshop",
		"disconnect",
		"mount-project",
		"start-workshop",
		"retrieve-sdk",
		"install-sdk",
		"link-sdk",
		"auto-connect",
		"run-hook",
		"run-hook", // restore state hook
	}

	verifyExpectedTasks(c, ts[0].Tasks(), expected_ws)
	verifyExpectedTasks(c, ts[1].Tasks(), expected_ws_1)

	c.Assert(ts[0].MaybeEdge(workshopstate.EdgeLastBeforeRefreshIrreversible).Kind(), check.Equals, "start-workshop")
	waitFor := ts[0].MaybeEdge(workshopstate.EdgeCleanupRefresh).WaitTasks()
	c.Assert(waitFor, testutil.DeepUnsortedMatches, []*state.Task{
		ts[0].MaybeEdge(workshopstate.EdgeLastBeforeRefreshIrreversible),
		ts[1].MaybeEdge(workshopstate.EdgeLastBeforeRefreshIrreversible),
	})

	c.Assert(ts[1].MaybeEdge(workshopstate.EdgeLastBeforeRefreshIrreversible).Kind(), check.Equals, "run-hook")
	waitFor = ts[1].MaybeEdge(workshopstate.EdgeCleanupRefresh).WaitTasks()
	c.Assert(waitFor, testutil.DeepUnsortedMatches, []*state.Task{
		ts[0].MaybeEdge(workshopstate.EdgeLastBeforeRefreshIrreversible),
		ts[1].MaybeEdge(workshopstate.EdgeLastBeforeRefreshIrreversible),
	})

	for i, t := range ts {
		s.ensureTaskHasWorkshopAndProjectKeys(c, files[i].Name, t.Tasks())
	}
}

func (s *S) TestRefreshWithAnSDK(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	testSdk := workshopbackend.SdkRecord{Name: "sdk-1", Channel: "latest/stable"}
	testSdk2 := workshopbackend.SdkRecord{Name: "sdk-2", Channel: "latest/stable"}

	file := &workshopbackend.WorkshopFile{
		Name: "ws",
		Base: "ubuntu@22.04",
		Sdks: workshopbackend.SdkList{testSdk, testSdk2}}

	ts, err := workshopstate.Refresh(s.state, file, []sdk.Setup{
		{Name: "sdk-1", Channel: "latest/stable"},
		{Name: "sdk-2", Channel: "latest/stable"},
	}, s.project)
	c.Assert(err, check.IsNil)

	expected := []string{
		"create-state-storage",
		"remove-state-storage",
		"create-workshop",
		"remove-workshop-stash",
		"run-hook",
		"run-hook",
		"stash-workshop",
		"disconnect",
		"disconnect",
		"mount-project",
		"start-workshop",
		"retrieve-sdk",
		"install-sdk",
		"link-sdk",
		"retrieve-sdk",
		"install-sdk",
		"link-sdk",
		"auto-connect",
		"auto-connect",
		"run-hook",
		"run-hook", // restore state hook
		"run-hook",
		"run-hook",
	}

	tasks := ts.Tasks()

	c.Assert(err, check.Equals, nil)
	verifyExpectedTasks(c, tasks, expected)
	verifyDisconnectDependencies(c, ts)

	s.ensureTaskHasWorkshopAndProjectKeys(c, "ws", tasks)
}

func (s *S) TestRefreshManyTasktest(c *check.C) {
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
		"create-state-storage",
		"remove-state-storage",
		"run-hook",
		"stash-workshop",
		"disconnect",
		"create-workshop",
		"mount-project",
		"start-workshop",
		"retrieve-sdk",
		"install-sdk",
		"link-sdk",
		"auto-connect",
		"run-hook",
		"run-hook", // restore state hook
		"remove-workshop-stash",
	}

	for i, t := range ts {
		c.Assert(err, check.Equals, nil)
		verifyExpectedTasks(c, t.Tasks(), expected)
		s.ensureTaskHasWorkshopAndProjectKeys(c, files[i].Name, t.Tasks())
	}

}

func (s *S) TestRefreshManyWaitsOnAllSuccessfulBeforeRemovingStash(c *check.C) {
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

	lastChanceWs := ts[0].MaybeEdge(workshopstate.EdgeLastBeforeRefreshIrreversible)
	c.Assert(lastChanceWs, check.NotNil)
	c.Assert(lastChanceWs.Kind(), check.Equals, "run-hook")

	lastChanceWs1 := ts[1].MaybeEdge(workshopstate.EdgeLastBeforeRefreshIrreversible)
	c.Assert(lastChanceWs1, check.NotNil)
	c.Assert(lastChanceWs1.Kind(), check.Equals, "run-hook")

	// Ensure that the remove-stash for ws will wait on the own
	// remove-state-storage and ws1's (i.e. all the other workshops') restore
	// state hooks
	removeWsStStorage := ts[0].MaybeEdge(workshopstate.EdgeCleanupRefresh)
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
	removeWsStStorage = ts[1].MaybeEdge(workshopstate.EdgeCleanupRefresh)
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

func (s *S) TestRefreshManyRestoreStateHooksExecutedSequentially(c *check.C) {
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
	}
	c.Assert(ts.MaybeEdge(workshopstate.EdgeLastBeforeRefreshIrreversible), check.Equals, prev)
}

func (s *S) TestRefreshManySaveStateHooksExecutedSequentially(c *check.C) {
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
	}
}

func (s *S) TestRefreshManyNoRestoreStateHooks(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	file := &workshopbackend.WorkshopFile{
		Name: "ws1",
		Base: "ubuntu@22.04",
		Sdks: workshopbackend.SdkList{},
	}

	ts := workshopstate.RestoreStateHooks(s.state, "ws", []sdk.Setup{}, file.Sdks)
	c.Assert(ts.Tasks(), check.HasLen, 0)
}

func (s *S) TestStartMany(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	ts, err := workshopstate.StartManyImpl(s.state, []string{"ws-1", "ws-2"}, s.project)
	c.Assert(err, check.IsNil)
	c.Assert(ts.Tasks(), check.HasLen, 2)
	c.Assert(ts.Tasks()[0].Kind(), check.Equals, "start-workshop")
	c.Assert(ts.Tasks()[1].Kind(), check.Equals, "start-workshop")

	c.Assert(ts.Tasks()[0].Has("stop-operation"), check.Equals, true)
	c.Assert(ts.Tasks()[0].Has("start-operation"), check.Equals, true)

	c.Assert(ts.Tasks()[1].Has("stop-operation"), check.Equals, true)
	c.Assert(ts.Tasks()[1].Has("start-operation"), check.Equals, true)
}

func (s *S) TestStopMany(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	ts, err := workshopstate.StopManyImpl(s.state, []string{"ws-1", "ws-2"}, s.project)
	c.Assert(err, check.IsNil)
	c.Assert(ts.Tasks(), check.HasLen, 2)
	c.Assert(ts.Tasks()[0].Kind(), check.Equals, "stop-workshop")
	c.Assert(ts.Tasks()[1].Kind(), check.Equals, "stop-workshop")

	var force bool
	c.Assert(ts.Tasks()[0].Has("stop-operation"), check.Equals, true)
	c.Assert(ts.Tasks()[0].Has("start-operation"), check.Equals, true)
	ts.Tasks()[0].Get("force", &force)
	c.Assert(force, check.Equals, false)

	c.Assert(ts.Tasks()[1].Has("stop-operation"), check.Equals, true)
	c.Assert(ts.Tasks()[1].Has("start-operation"), check.Equals, true)
	ts.Tasks()[1].Get("force", &force)
	c.Assert(force, check.Equals, false)
}

func (s *S) TestRemoveMany(c *check.C) {
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
	c.Assert(ts.Tasks()[0].Has("start-operation"), check.Equals, true)
	c.Assert(ts.Tasks()[3].Has("stop-operation"), check.Equals, true)
	s.ensureTaskHasWorkshopAndProjectKeys(c, "ws-1", ts.Tasks())
}
