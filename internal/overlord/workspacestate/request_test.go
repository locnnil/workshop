package workspacestate_test

import (
	"testing"

	"github.com/canonical/workspace/internal/overlord/state"
	workspace "github.com/canonical/workspace/internal/overlord/workspacestate"
	"github.com/canonical/workspace/internal/workspacebackend"
	"golang.org/x/exp/slices"
	. "gopkg.in/check.v1"
)

type S struct {
	state *state.State
}

var _ = Suite(&S{})

func Test(t *testing.T) { TestingT(t) }

func (s *S) SetUpTest(c *C) {
	s.state = state.New(nil)
}

func verifyExpectedTasks(c *C, ts []*state.Task, expected []string) {
	actual := make([]string, 0, len(ts))
	for _, i := range ts {
		actual = append(actual, i.Kind())
	}
	slices.Sort(actual)
	slices.Sort(expected)

	c.Assert(actual, DeepEquals, expected)
}

func (s *S) TestLaunchWorkspaceNoSdk(c *C) {
	s.state.Lock()
	defer s.state.Unlock()
	file := &workspacebackend.WorkspaceFile{Name: "test", Base: "ubuntu@22.04"}
	ts, err := workspace.Launch(s.state, file)

	expected := []string{"create-workspace",
		"mount-project",
		"start-workspace"}
	tasks := ts.Tasks()

	c.Assert(err, Equals, nil)
	verifyExpectedTasks(c, tasks, expected)

	var base string
	err = tasks[0].Get("base", &base)
	c.Assert(err, Equals, nil)
	c.Assert(base, Equals, "ubuntu@22.04")
}

func (s *S) TestLaunchWorkspaceWithSdks(c *C) {
	s.state.Lock()
	defer s.state.Unlock()
	sdk := workspacebackend.Sdk{Name: "sdk", Channel: "latest/stable"}
	sdk_2 := workspacebackend.Sdk{Name: "sdk_2", Channel: "latest/stable"}

	file := &workspacebackend.WorkspaceFile{
		Name: "test",
		Base: "ubuntu@22.04",
		Sdks: workspacebackend.SdkList{sdk, sdk_2}}

	ts, err := workspace.Launch(s.state, file)

	expected := []string{"create-workspace",
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

	c.Assert(err, Equals, nil)
	verifyExpectedTasks(c, tasks, expected)

	var s1, s2 workspacebackend.Sdk
	err = tasks[3].Get("sdk", &s1)
	c.Assert(err, Equals, nil)
	c.Assert(s1, Equals, sdk)

	err = tasks[4].Get("sdk", &s2)
	c.Assert(err, Equals, nil)
	c.Assert(s2, Equals, sdk_2)

	var id1, id2 string
	err = tasks[5].Get("sdk-retrieve-task", &id1)
	c.Assert(err, Equals, nil)
	c.Assert(id1, Equals, tasks[3].ID())

	err = tasks[6].Get("sdk-retrieve-task", &id2)
	c.Assert(err, Equals, nil)
	c.Assert(id2, Equals, tasks[4].ID())
}
