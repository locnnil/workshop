package workspacestate_test

import (
	"testing"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/overlord/workspacestate"
	workspace "github.com/canonical/workspace/internal/overlord/workspacestate"
	"github.com/canonical/workspace/internal/sdk"
	"github.com/canonical/workspace/internal/testutil"
	"github.com/canonical/workspace/internal/workspacebackend"

	"golang.org/x/exp/slices"
	"gopkg.in/check.v1"
)

type S struct {
	state   *state.State
	project *workspacebackend.Project
}

var _ = check.Suite(&S{})

func Test(t *testing.T) { check.TestingT(t) }

func (s *S) SetUpTest(c *check.C) {
	s.state = state.New(nil)
	s.project = &workspacebackend.Project{Path: c.MkDir(), ProjectId: "42ws42ws"}
}

func (s *S) ensureTaskHasWorkspaceAndProjectKeys(c *check.C, w string, ts []*state.Task) {
	for _, i := range ts {
		var prj workspacebackend.Project
		err := i.Get("project", &prj)
		c.Assert(err, check.IsNil)
		c.Assert(&prj, check.DeepEquals, s.project)

		var workspace string
		err = i.Get("workspace", &workspace)
		c.Assert(err, check.IsNil)
		c.Assert(workspace, check.Equals, w)
	}
}

func verifyExpectedTasks(c *check.C, ts []*state.Task, expected []string) {
	actual := make([]string, 0, len(ts))
	for _, i := range ts {
		actual = append(actual, i.Kind())
	}
	slices.Sort(actual)
	slices.Sort(expected)

	c.Assert(actual, testutil.DeepUnsortedMatches, expected)
}

func (s *S) TestLaunchWorkspaceNoSdk(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	file := &workspacebackend.WorkspaceFile{Name: "test", Base: "ubuntu@22.04"}
	ts, err := workspace.Launch(s.state, file, s.project)

	expected := []string{"create-workspace",
		"mount-project",
		"start-workspace"}
	tasks := ts.Tasks()

	c.Assert(err, check.Equals, nil)
	verifyExpectedTasks(c, tasks, expected)

	var base string
	err = tasks[0].Get("base", &base)
	c.Assert(err, check.Equals, nil)
	c.Assert(base, check.Equals, "ubuntu@22.04")
	s.ensureTaskHasWorkspaceAndProjectKeys(c, "test", ts.Tasks())
}

func (s *S) TestLaunchWorkspaceWithSdks(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	sdk := workspacebackend.Sdk{Name: "sdk", Channel: "latest/stable"}
	sdk_2 := workspacebackend.Sdk{Name: "sdk_2", Channel: "latest/stable"}

	file := &workspacebackend.WorkspaceFile{
		Name: "test",
		Base: "ubuntu@22.04",
		Sdks: workspacebackend.SdkList{sdk, sdk_2}}

	ts, err := workspace.Launch(s.state, file, s.project)

	expected := []string{
		"create-workspace",
		"mount-project",
		"start-workspace",
		"retrieve-sdk",
		"retrieve-sdk",
		"install-sdk",
		"install-sdk",
		"link-sdk",
		"link-sdk",
		"run-hook",
		"run-hook"}

	tasks := ts.Tasks()

	c.Assert(err, check.Equals, nil)
	verifyExpectedTasks(c, tasks, expected)

	var s1, s2 workspacebackend.Sdk
	err = tasks[3].Get("sdk-setup", &s1)
	c.Assert(err, check.Equals, nil)
	c.Assert(s1, check.Equals, sdk)

	err = tasks[4].Get("sdk-setup", &s2)
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
	err = tasks[7].Get("sdk-retrieve-task", &id1)
	c.Assert(err, check.Equals, nil)
	c.Assert(id1, check.Equals, tasks[4].ID())

	// link-sdk task for sdk_2
	err = tasks[8].Get("sdk-retrieve-task", &id2)
	c.Assert(err, check.Equals, nil)
	c.Assert(id2, check.Equals, tasks[4].ID())

	s.ensureTaskHasWorkspaceAndProjectKeys(c, "test", tasks)
}

func (s *S) TestRefreshEmptyWorkspace(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	file := &workspacebackend.WorkspaceFile{
		Name: "ws",
		Base: "ubuntu@22.04",
		Sdks: workspacebackend.SdkList{}}

	ts, err := workspace.Refresh(s.state, file, []*sdk.SdkInfo{}, s.project)
	c.Assert(err, check.IsNil)

	expected := []string{
		"create-state-storage",
		"remove-state-storage",
		"create-workspace",
		"delete-workspace-copy",
		"make-workspace-copy",
		"mount-project",
		"start-workspace",
	}

	tasks := ts.Tasks()

	c.Assert(err, check.Equals, nil)
	verifyExpectedTasks(c, tasks, expected)

	s.ensureTaskHasWorkspaceAndProjectKeys(c, "ws", tasks)
}

func (s *S) TestRefreshManyEmptyWorkspaceMany(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	test_sdk := workspacebackend.Sdk{Name: "sdk", Channel: "latest/stable"}

	files := []*workspacebackend.WorkspaceFile{
		{
			Name: "ws",
			Base: "ubuntu@22.04",
			Sdks: workspacebackend.SdkList{}},
		{
			Name: "ws-1",
			Base: "ubuntu@22.04",
			Sdks: workspacebackend.SdkList{test_sdk}},
	}

	content := [][]*sdk.SdkInfo{
		nil,
		[]*sdk.SdkInfo{{
			Name:    "sdk",
			Channel: "latest/stable",
		},
		},
	}

	ts, err := workspace.RefreshManyImpl(s.state, files, content, s.project)
	c.Assert(err, check.IsNil)

	expected_ws := []string{
		"create-state-storage",
		"remove-state-storage",
		"create-workspace",
		"delete-workspace-copy",
		"make-workspace-copy",
		"mount-project",
		"start-workspace",
	}

	expected_ws_1 := []string{
		"create-state-storage",
		"remove-state-storage",
		"create-workspace",
		"delete-workspace-copy",
		"run-hook",
		"make-workspace-copy",
		"mount-project",
		"start-workspace",
		"retrieve-sdk",
		"install-sdk",
		"link-sdk",
		"run-hook",
		"run-hook", // restore state hook
	}

	verifyExpectedTasks(c, ts[0].Tasks(), expected_ws)
	verifyExpectedTasks(c, ts[1].Tasks(), expected_ws_1)

	c.Assert(ts[0].MaybeEdge(workspace.LastBeforeRefreshIrreversibleEdge).Kind(), check.Equals, "start-workspace")
	waitFor := ts[0].MaybeEdge(workspace.CleanupRefreshEdge).WaitTasks()
	c.Assert(waitFor, testutil.DeepUnsortedMatches, []*state.Task{
		ts[0].MaybeEdge(workspace.LastBeforeRefreshIrreversibleEdge),
		ts[1].MaybeEdge(workspace.LastBeforeRefreshIrreversibleEdge),
	})

	c.Assert(ts[1].MaybeEdge(workspace.LastBeforeRefreshIrreversibleEdge).Kind(), check.Equals, "run-hook")
	waitFor = ts[1].MaybeEdge(workspace.CleanupRefreshEdge).WaitTasks()
	c.Assert(waitFor, testutil.DeepUnsortedMatches, []*state.Task{
		ts[0].MaybeEdge(workspace.LastBeforeRefreshIrreversibleEdge),
		ts[1].MaybeEdge(workspace.LastBeforeRefreshIrreversibleEdge),
	})

	for i, t := range ts {
		s.ensureTaskHasWorkspaceAndProjectKeys(c, files[i].Name, t.Tasks())
	}
}

func (s *S) TestRefreshWithAnSDK(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	testSdk := workspacebackend.Sdk{Name: "sdk", Channel: "latest/stable"}

	file := &workspacebackend.WorkspaceFile{
		Name: "ws",
		Base: "ubuntu@22.04",
		Sdks: workspacebackend.SdkList{testSdk}}

	ts, err := workspace.Refresh(s.state, file, []*sdk.SdkInfo{
		{
			Name:    "sdk",
			Channel: "latest/stable",
		},
	}, s.project)
	c.Assert(err, check.IsNil)

	expected := []string{
		"create-state-storage",
		"remove-state-storage",
		"create-workspace",
		"delete-workspace-copy",
		"run-hook",
		"make-workspace-copy",
		"mount-project",
		"start-workspace",
		"retrieve-sdk",
		"install-sdk",
		"link-sdk",
		"run-hook",
		"run-hook", // restore state hook
	}

	tasks := ts.Tasks()

	c.Assert(err, check.Equals, nil)
	verifyExpectedTasks(c, tasks, expected)

	s.ensureTaskHasWorkspaceAndProjectKeys(c, "ws", tasks)
}

func (s *S) TestRefreshManyTasktest(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	testSdk := workspacebackend.Sdk{Name: "sdk", Channel: "latest/stable"}

	files := []*workspacebackend.WorkspaceFile{
		{
			Name: "ws",
			Base: "ubuntu@22.04",
			Sdks: workspacebackend.SdkList{testSdk},
		},
		{
			Name: "ws1",
			Base: "ubuntu@22.04",
			Sdks: workspacebackend.SdkList{testSdk},
		},
	}

	content := [][]*sdk.SdkInfo{
		[]*sdk.SdkInfo{{
			Name:    "sdk",
			Channel: "latest/stable",
		},
		},
		[]*sdk.SdkInfo{{
			Name:    "sdk",
			Channel: "latest/stable",
		},
		},
	}

	ts, err := workspace.RefreshManyImpl(s.state, files, content, s.project)
	c.Assert(err, check.IsNil)

	expected := []string{
		"create-state-storage",
		"remove-state-storage",
		"run-hook",
		"make-workspace-copy",
		"create-workspace",
		"mount-project",
		"start-workspace",
		"retrieve-sdk",
		"install-sdk",
		"link-sdk",
		"run-hook",
		"run-hook", // restore state hook
		"delete-workspace-copy",
	}

	for i, t := range ts {
		c.Assert(err, check.Equals, nil)
		verifyExpectedTasks(c, t.Tasks(), expected)
		s.ensureTaskHasWorkspaceAndProjectKeys(c, files[i].Name, t.Tasks())
	}

}

func (s *S) TestRefreshManyWaitsOnAllSuccessfulBeforeRemovingCopy(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	testSdk := workspacebackend.Sdk{Name: "sdk", Channel: "latest/stable"}

	files := []*workspacebackend.WorkspaceFile{
		{
			Name: "ws",
			Base: "ubuntu@22.04",
			Sdks: workspacebackend.SdkList{testSdk},
		},
		{
			Name: "ws1",
			Base: "ubuntu@22.04",
			Sdks: workspacebackend.SdkList{testSdk},
		},
	}

	content := [][]*sdk.SdkInfo{
		[]*sdk.SdkInfo{{
			Name:    "sdk",
			Channel: "latest/stable",
		},
		},
		[]*sdk.SdkInfo{{
			Name:    "sdk",
			Channel: "latest/stable",
		},
		},
	}

	ts, err := workspace.RefreshManyImpl(s.state, files, content, s.project)
	c.Assert(err, check.IsNil)

	lastChanceWs := ts[0].MaybeEdge(workspacestate.LastBeforeRefreshIrreversibleEdge)
	c.Assert(lastChanceWs, check.NotNil)
	c.Assert(lastChanceWs.Kind(), check.Equals, "run-hook")

	lastChanceWs1 := ts[1].MaybeEdge(workspacestate.LastBeforeRefreshIrreversibleEdge)
	c.Assert(lastChanceWs1, check.NotNil)
	c.Assert(lastChanceWs1.Kind(), check.Equals, "run-hook")

	// Ensure that the delete-copy for ws will wait on the own
	// remove-state-storage and ws1's (i.e. all the other workspaces') restore
	// state hooks
	removeWsStStorage := ts[0].MaybeEdge(workspacestate.CleanupRefreshEdge)
	deleteCopy := removeWsStStorage.HaltTasks()[0]
	c.Assert(removeWsStStorage.WaitTasks(), testutil.DeepUnsortedMatches, []*state.Task{lastChanceWs, lastChanceWs1})

	// Ensure that the copy and state storage removals belong to a separate
	// lane. In the case of their abortion it must not trigger abortion of the
	// refresh that, by that moment, is already done
	c.Assert(removeWsStStorage.Lanes()[0], check.Not(check.Equals), lastChanceWs.Lanes()[0])
	c.Assert(deleteCopy.Lanes()[0], check.Not(check.Equals), lastChanceWs.Lanes()[0])
	c.Assert(deleteCopy.Lanes(), check.HasLen, 1)
	c.Assert(removeWsStStorage.Lanes(), check.HasLen, 1)

	// Ensure that the delete-copy for ws1 will wait on the own
	// remove-state-storage and ws's (i.e. all the other workspaces') restore
	// state hooks
	removeWsStStorage = ts[1].MaybeEdge(workspacestate.CleanupRefreshEdge)
	deleteCopy = removeWsStStorage.HaltTasks()[0]
	c.Assert(removeWsStStorage.WaitTasks(), testutil.DeepUnsortedMatches, []*state.Task{lastChanceWs1, lastChanceWs})

	// Ensure that the copy and state storage removals belong to a separate
	// lane. In the case of their abortion it must not trigger abortion of the
	// refresh that, by that moment, is already done
	c.Assert(removeWsStStorage.Lanes()[0], check.Not(check.Equals), lastChanceWs1.Lanes()[0])
	c.Assert(deleteCopy.Lanes()[0], check.Not(check.Equals), lastChanceWs1.Lanes()[0])
	c.Assert(deleteCopy.Lanes(), check.HasLen, 1)
	c.Assert(removeWsStStorage.Lanes(), check.HasLen, 1)
}

func (s *S) TestRefreshManyRestoreStateHooksExecutedSequentially(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	one := workspacebackend.Sdk{Name: "one", Channel: "latest/stable"}
	two := workspacebackend.Sdk{Name: "two", Channel: "latest/stable"}

	file := &workspacebackend.WorkspaceFile{
		Name: "ws1",
		Base: "ubuntu@22.04",
		Sdks: workspacebackend.SdkList{one, two},
	}

	ts := workspace.RestoreStateHooks(s.state, file, s.project)
	c.Assert(ts.Tasks(), check.HasLen, 2)

	prev := (*state.Task)(nil)
	for _, t := range ts.Tasks() {
		if prev != nil {
			c.Assert(t.WaitTasks(), check.DeepEquals, []*state.Task{prev})
		}
		prev = t
	}
	c.Assert(ts.MaybeEdge(workspace.LastBeforeRefreshIrreversibleEdge), check.Equals, prev)
}

func (s *S) TestRefreshManySaveStateHooksExecutedSequentially(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	one := &sdk.SdkInfo{Name: "one", Channel: "latest/stable"}
	two := &sdk.SdkInfo{Name: "two", Channel: "latest/stable"}

	ts := workspace.SaveStateHooks(s.state, []*sdk.SdkInfo{one, two}, s.project)
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

	file := &workspacebackend.WorkspaceFile{
		Name: "ws1",
		Base: "ubuntu@22.04",
		Sdks: workspacebackend.SdkList{},
	}

	ts := workspace.RestoreStateHooks(s.state, file, s.project)
	c.Assert(ts.Tasks(), check.HasLen, 0)
}
