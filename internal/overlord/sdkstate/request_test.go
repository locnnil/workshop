package sdkstate_test

import (
	"github.com/canonical/workspace/internal/overlord/sdkstate"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
	"gopkg.in/check.v1"
)

type SdkStateTasks struct {
	state *state.State
}

func (i *SdkStateTasks) SetUpTest(c *check.C) {
	i.state = state.New(nil)
}

var _ = check.Suite(&SdkStateTasks{})

func (i *SdkStateTasks) TestInstall(c *check.C) {
	i.state.Lock()
	defer i.state.Unlock()

	sdk := workspacebackend.Sdk{Name: "sdk", Channel: "latest/stable"}

	tasks := sdkstate.Install(i.state, sdk.Name, "retrieve").Tasks()

	c.Check(tasks, check.HasLen, 2)
	c.Check(tasks[1].WaitTasks(), check.HasLen, 1)
	var id string
	tasks[0].Get("sdk-retrieve-task", &id)
	c.Check(tasks[0].Summary(), check.Equals, "Install SDK \"sdk\"")
	c.Check(id, check.Equals, "retrieve")
	tasks[1].Get("sdk-retrieve-task", &id)
	c.Check(tasks[1].Summary(), check.Equals, "Link SDK \"sdk\"")
	c.Check(id, check.Equals, "retrieve")
}

func (i *SdkStateTasks) TestRetrieve(c *check.C) {
	i.state.Lock()
	defer i.state.Unlock()

	sdk := workspacebackend.Sdk{Name: "sdk", Channel: "latest/stable"}

	task := sdkstate.Retrieve(i.state, &sdk)

	var s workspacebackend.Sdk
	task.Get("sdk-setup", &s)
	c.Check(s, check.DeepEquals, sdk)
	c.Check(task.Kind(), check.Equals, "retrieve-sdk")
	c.Check(task.Summary(), check.Equals, "Retrieve SDK \"sdk\" from channel \"latest/stable\"")
}
