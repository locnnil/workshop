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

	task := sdkstate.Install(i.state, sdk.Name, "retrieve")

	var id string
	c.Assert(task.Get("sdk-retrieve-task", &id), check.IsNil)
	c.Check(id, check.Equals, "retrieve")
	c.Check(task.Kind(), check.Equals, "install-sdk")
	c.Check(task.Summary(), check.Equals, `Install "sdk" SDK`)
}

func (i *SdkStateTasks) TestRetrieve(c *check.C) {
	i.state.Lock()
	defer i.state.Unlock()

	rec := sdk.Setup{
		Name:     "sdk",
		Channel:  "latest/stable",
		Source:   sdk.TrySource,
		Revision: sdk.R(42),
		Sha3_384: "e1cfa86c92b87afc44a68c2f8f7c3b62b8dc926aa99234ab8ac2e94e242736c05c80a431041a686f3f2bcb5c648676ea",
	}

	task := sdkstate.Retrieve(i.state, rec)

	var s sdk.Setup
	c.Assert(task.Get("sdk-setup", &s), check.IsNil)
	c.Check(s, check.DeepEquals, rec)
	c.Check(task.Kind(), check.Equals, "retrieve-sdk")
	c.Check(task.Summary(), check.Equals, `Retrieve "sdk" SDK`)

	rec.Source = sdk.StoreSource
	task = sdkstate.Retrieve(i.state, rec)
	c.Check(task.Summary(), check.Equals, `Retrieve "sdk" SDK from channel "latest/stable"`)
}
