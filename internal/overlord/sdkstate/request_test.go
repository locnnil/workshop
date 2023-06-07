package sdkstate_test

import (
	"github.com/canonical/workspace/internal/overlord/sdkstate"
	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
	. "gopkg.in/check.v1"
)

type SdkStateTasks struct {
	state *state.State
}

func (i *SdkStateTasks) SetUpTest(c *C) {
	i.state = state.New(nil)
}

var _ = Suite(&SdkStateTasks{})

func (i *SdkStateTasks) TestInstall(c *C) {
	i.state.Lock()
	defer i.state.Unlock()

	sdk := workspacebackend.Sdk{Name: "sdk", Channel: "latest/stable"}

	tasks := sdkstate.Install(i.state, sdk.Name, "retrieve").Tasks()

	c.Check(tasks, HasLen, 2)
	c.Check(tasks[1].WaitTasks(), HasLen, 1)
	var id string
	tasks[0].Get("sdk-retrieve-task", &id)
	c.Check(id, Equals, "retrieve")
	tasks[1].Get("sdk-retrieve-task", &id)
	c.Check(id, Equals, "retrieve")
}
