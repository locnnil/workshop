// Copyright (c) 2026 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package sdkstate_test

import (
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/overlord/handlersetup"
	"github.com/canonical/workshop/internal/overlord/sdkstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/sdk"
)

type SdkStateTasks struct {
	state *state.State
}

func (i *SdkStateTasks) SetUpTest(c *check.C) {
	i.state = state.New(nil)
}

var _ = check.Suite(&SdkStateTasks{})

func (i *SdkStateTasks) TestRetrieve(c *check.C) {
	i.state.Lock()

	trySdk := sdk.Setup{
		Name:     "test-sdk",
		Source:   sdk.TrySource,
		Revision: sdk.R(-42),
		Sha3_384: "d024fbe91c6b99d0064306d52006c17a5d0406822ff253fbbe6a934ca9be50d3ff9a6ec3bac3be8396006029a1ff453a",
	}

	storeSdk := sdk.Setup{
		Name:      "test-sdk-2",
		PackageID: "iCJybjjWd2n48hKoMdjGEIWwA3i2TmX7",
		Channel:   "latest/stable",
		Revision:  sdk.R(42),
		Sha3_384:  "d4089378c26310627268153caa216240311f2a3193c778e96ed6dd895dc10c82db50f4f39676b29d23d9813b21e14b9b",
	}

	change := i.state.NewChange("sample", "...")
	change.Set("ws_new_sdks", []sdk.Setup{trySdk, storeSdk})

	t1 := sdkstate.Retrieve(i.state, trySdk)
	t2 := sdkstate.Retrieve(i.state, storeSdk)
	change.AddAll(state.NewTaskSet(t1, t2))

	i.state.Unlock()

	s, err := sdkstate.SdkSetup(t1, "ws", handlersetup.NewWorkshop)
	c.Assert(err, check.IsNil)
	c.Check(s, check.Equals, trySdk)
	c.Check(t1.Kind(), check.Equals, "retrieve-sdk")
	c.Check(t1.Summary(), check.Equals, `Retrieve "test-sdk" SDK`)

	s, err = sdkstate.SdkSetup(t2, "ws", handlersetup.NewWorkshop)
	c.Assert(err, check.IsNil)
	c.Check(s, check.Equals, storeSdk)
	c.Check(t2.Kind(), check.Equals, "retrieve-sdk")
	c.Check(t2.Summary(), check.Equals, `Retrieve "test-sdk-2" SDK from channel "latest/stable"`)
}
