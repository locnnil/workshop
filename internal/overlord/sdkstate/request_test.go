package sdkstate_test

import (
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/overlord/sdkstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
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

	sdk := workshop.SdkRecord{Name: "sdk"}

	tasks := sdkstate.Install(i.state, "42424242", "ws", sdk.Name, "retrieve").Tasks()

	c.Check(tasks, check.HasLen, 3)
	c.Check(tasks[1].WaitTasks(), check.HasLen, 1)
	c.Check(tasks[2].WaitTasks(), check.HasLen, 1)
	var id string
	tasks[0].Get("sdk-retrieve-task", &id)
	c.Check(tasks[0].Summary(), check.Equals, `Install "sdk" SDK`)
	c.Check(id, check.Equals, "retrieve")
	tasks[1].Get("sdk-retrieve-task", &id)
	c.Check(tasks[1].Summary(), check.Equals, `Link "sdk" SDK`)
	c.Check(id, check.Equals, "retrieve")
	c.Check(tasks[2].Summary(), check.Equals, `Run hook "setup-base" for "sdk" SDK`)
}

func (i *SdkStateTasks) TestRetrieve(c *check.C) {
	i.state.Lock()
	defer i.state.Unlock()

	rec := sdk.Setup{Name: "sdk", Channel: "latest/stable"}

	task := sdkstate.Retrieve(i.state, rec)

	var s sdk.Setup
	task.Get("sdk-setup", &s)
	c.Check(s, check.DeepEquals, rec)
	c.Check(task.Kind(), check.Equals, "retrieve-sdk")
	c.Check(task.Summary(), check.Equals, "Retrieve \"sdk\" SDK from channel \"latest/stable\"")
}
