package workspace_test

import (
	"path/filepath"
	"testing"

	util "github.com/canonical/workspace/internal"
	"github.com/canonical/workspace/internal/overlord/projectstate"
	"github.com/canonical/workspace/internal/overlord/state"
	workspace "github.com/canonical/workspace/internal/overlord/workspacestate"
	"github.com/canonical/workspace/internal/server"
	"github.com/spf13/afero"
	"golang.org/x/exp/slices"
	. "gopkg.in/check.v1"
)

type S struct {
	project *projectstate.Project
	state   *state.State
}

var _ = Suite(&S{})

func Test(t *testing.T) { TestingT(t) }

func fakeEvalSymlinks(path string) (string, error) {
	return path, nil
}

func (s *S) SetUpTest(c *C) {
	util.EvalSymlinks = fakeEvalSymlinks
	fs := afero.NewMemMapFs()
	server := server.WorkspaceServer(nil)
	s.project, _ = projectstate.NewProject(server, fs, "/")
	s.state = state.New(nil)
}

func (s *S) TearDownTest(c *C) {
	util.EvalSymlinks = filepath.EvalSymlinks
}

func verifyExpectedTasks(c *C, ts []*state.Task, tasks []string) {
	taskset, expected := make([]*state.Task, 0), make([]string, 0)
	copy(taskset, ts)
	copy(expected, tasks)
	slices.SortFunc(taskset, func(t, t1 *state.Task) bool {
		return t.Kind() < t1.Kind()
	})

	slices.Sort(expected)

	compare := func(t *state.Task, t1 string) int {
		if t.Kind() != t1 {
			return 1
		}
		return 0

	}
	c.Assert(slices.CompareFunc(taskset, expected, compare), Equals, 0)
}

func (s *S) TestLaunchWorkspaceNoSdk(c *C) {
	s.state.Lock()
	defer s.state.Unlock()
	file := &workspace.WorkspaceFile{Name: "test", Base: "ubuntu@22.04"}
	ts, err := workspace.Launch(s.state, s.project, file)

	expected := []string{"create-workspace",
		"add-workspace-device",
		"set-workspace-state"}
	tasks := ts.Tasks()

	c.Assert(err, Equals, nil)
	verifyExpectedTasks(c, tasks, expected)

	var base, wstate string
	err = tasks[0].Get("base", &base)
	c.Assert(err, Equals, nil)
	c.Assert(base, Equals, "ubuntu@22.04")

	err = tasks[2].Get("workspace-state", &wstate)
	c.Assert(err, Equals, nil)
	c.Assert(wstate, Equals, "start")
}

func (s *S) TestLaunchWorkspaceWithSdks(c *C) {
	s.state.Lock()
	defer s.state.Unlock()
	sdk := workspace.Sdk{Name: "sdk", Channel: "latest/stable"}
	sdk_2 := workspace.Sdk{Name: "sdk_2", Channel: "latest/stable"}

	file := &workspace.WorkspaceFile{
		Name: "test",
		Base: "ubuntu@22.04",
		Sdks: workspace.SdkList{sdk, sdk_2}}

	ts, err := workspace.Launch(s.state, s.project, file)

	expected := []string{"create-workspace",
		"add-workspace-device",
		"set-workspace-state",
		"retrieve-sdk",
		"retrieve-sdk",
		"install-sdk",
		"install-sdk"}

	tasks := ts.Tasks()

	c.Assert(err, Equals, nil)
	verifyExpectedTasks(c, tasks, expected)

	var s1, s2 workspace.Sdk
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
