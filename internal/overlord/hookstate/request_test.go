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

package hookstate_test

import (
	"fmt"
	"time"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/overlord/hookstate"
	"github.com/canonical/workshop/internal/overlord/state"
	"github.com/canonical/workshop/internal/workshop"
)

type hooksRequestSuite struct {
	state   *state.State
	project *workshop.Project
}

var _ = check.Suite(&hooksRequestSuite{})

func (s *hooksRequestSuite) SetUpTest(c *check.C) {
	s.state = state.New(nil)
	s.project = &workshop.Project{ProjectId: "42424242", Path: c.MkDir()}
}

func (s *hooksRequestSuite) TestCreateHook(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()

	for _, hook := range []hookstate.WorkshopHookType{
		hookstate.SetupBase,
		hookstate.SetupProject,
		hookstate.SaveState,
		hookstate.RestoreState,
	} {
		task := hookstate.Hook(s.state, "go", 0, hook)

		var hookSetup hookstate.HookSetup
		err := task.Get("hook-setup", &hookSetup)
		c.Assert(err, check.IsNil)
		c.Assert(hookSetup.Type(), check.Equals, hook.String())
		c.Assert(hookSetup.Sdk, check.Equals, "go")
		c.Check(task.Summary(), check.Equals, fmt.Sprintf("Run hook %q for \"go\" SDK", hookSetup.Type()))
	}
}

func (s *hooksRequestSuite) TestCreateHookWithTimeout(c *check.C) {
	s.state.Lock()
	defer s.state.Unlock()
	task := hookstate.Hook(s.state, "go", 5*time.Second, hookstate.CheckHealth)
	var hookSetup hookstate.HookSetup
	err := task.Get("hook-setup", &hookSetup)
	c.Assert(err, check.IsNil)
	c.Assert(hookSetup.Type(), check.Equals, "check-health")
	c.Assert(hookSetup.Sdk, check.Equals, "go")
	c.Assert(hookSetup.Timeout, check.Equals, 5*time.Second)
	c.Check(task.Summary(), check.Equals, fmt.Sprintf("Run hook %q for \"go\" SDK", hookSetup.Type()))
}
