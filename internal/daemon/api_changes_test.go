// Copyright (c) 2014-2020 Canonical Ltd
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

package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/canonical/workspace/internal/overlord/state"
	"github.com/canonical/workspace/internal/workspacebackend"
	"golang.org/x/exp/slices"

	"gopkg.in/check.v1"
)

func setupChanges(st *state.State) []string {
	chg1 := st.NewChange("launch", "launch...")
	chg1.Set("workspace", "one")
	chg1.Set("project-key", &workspacebackend.Project{ProjectId: "123", Path: "/home/user/test"})
	t1 := st.NewTask("create-workspace", "1...")
	t2 := st.NewTask("start-workspace", "2...")
	chg1.AddAll(state.NewTaskSet(t1, t2))
	t1.Logf("l11")
	t1.Logf("l12")
	chg2 := st.NewChange("remove", "remove...")
	chg2.Set("workspace", "two")
	chg2.Set("project-key", &workspacebackend.Project{ProjectId: "123", Path: "/home/user/test"})
	t3 := st.NewTask("unlink-sdk", "1...")
	chg2.AddTask(t3)
	t3.SetStatus(state.ErrorStatus)
	t3.Errorf("unlink failed")

	return []string{chg1.ID(), chg2.ID(), t1.ID(), t2.ID(), t3.ID()}
}

func (s *apiSuite) TestStateChangesProjectAndWorkspaceMustBeProvidedTogether(c *check.C) {
	restore := state.MockTime(time.Date(2016, 04, 21, 1, 2, 3, 0, time.UTC))
	defer restore()

	// Setup
	d := s.daemon(c)
	st := d.overlord.State()
	st.Lock()
	setupChanges(st)
	st.Unlock()

	stateChangesCmd := apiCmd("/v1/changes")

	// Execute
	req, err := http.NewRequest("GET", "/v1/changes?workspaces=test", nil)
	c.Assert(err, check.IsNil)
	rsp := v1GetChanges(stateChangesCmd, req, nil).(*resp)

	// Verify
	c.Check(rsp.Type, check.Equals, ResponseTypeError)
	c.Check(rsp.Status, check.Equals, 400)
	c.Check(rsp.Result.(*errorResult).Message, check.Equals,
		"project-id must be provided if workspaces are specified")
}

func (s *apiSuite) TestStateChangesDefaultToAll(c *check.C) {
	restore := state.MockTime(time.Date(2016, 04, 21, 1, 2, 3, 0, time.UTC))
	defer restore()

	// Setup
	d := s.daemon(c)
	st := d.overlord.State()
	st.Lock()
	setupChanges(st)
	st.Unlock()

	stateChangesCmd := apiCmd("/v1/changes")

	// Execute
	req, err := http.NewRequest("GET", "/v1/changes", nil)
	c.Assert(err, check.IsNil)
	rsp := v1GetChanges(stateChangesCmd, req, nil).(*resp)

	// Verify
	c.Check(rsp.Type, check.Equals, ResponseTypeSync)
	c.Check(rsp.Status, check.Equals, 200)
	c.Assert(rsp.Result, check.HasLen, 2)

	res, err := rsp.MarshalJSON()
	c.Assert(err, check.IsNil)

	c.Check(string(res), check.Matches, `.*{"id":"\w+","kind":"launch","summary":"launch...","status":"Do","tasks":\[{"id":"\w+","kind":"create-workspace","summary":"1...","status":"Do","log":\["2016-04-21T01:02:03Z INFO l11","2016-04-21T01:02:03Z INFO l12"],"progress":{"label":"","done":0,"total":1},"spawn-time":"2016-04-21T01:02:03Z"}.*],"ready":false,"spawn-time":"2016-04-21T01:02:03Z","path":"/home/user/test"}.*`)
	c.Check(string(res), check.Matches, `.*{"id":"\w+","kind":"remove","summary":"remove...","status":"Error","tasks":\[{"id":"\w+","kind":"unlink-sdk","summary":"1...","status":"Error","log":\["2016-04-21T01:02:03Z ERROR unlink failed"],"progress":{"label":"","done":1,"total":1},"spawn-time":"2016-04-21T01:02:03Z","ready-time":"2016-04-21T01:02:03Z"}.*],"ready":true,"err":"[^"]+".*`)
}

func (s *apiSuite) TestStateChangesInProgress(c *check.C) {
	restore := state.MockTime(time.Date(2016, 04, 21, 1, 2, 3, 0, time.UTC))
	defer restore()

	// Setup
	d := s.daemon(c)
	st := d.overlord.State()
	st.Lock()
	setupChanges(st)
	st.Unlock()

	stateChangesCmd := apiCmd("/v1/changes")

	// Execute
	req, err := http.NewRequest("GET", "/v1/changes?select=in-progress", nil)
	c.Assert(err, check.IsNil)
	rsp := v1GetChanges(stateChangesCmd, req, nil).(*resp)

	// Verify
	c.Check(rsp.Type, check.Equals, ResponseTypeSync)
	c.Check(rsp.Status, check.Equals, 200)
	c.Assert(rsp.Result, check.HasLen, 1)

	res, err := rsp.MarshalJSON()
	c.Assert(err, check.IsNil)

	c.Check(string(res), check.Matches, `.*{"id":"\w+","kind":"launch","summary":"launch...","status":"Do","tasks":\[{"id":"\w+","kind":"create-workspace","summary":"1...","status":"Do","log":\["2016-04-21T01:02:03Z INFO l11","2016-04-21T01:02:03Z INFO l12"],"progress":{"label":"","done":0,"total":1},"spawn-time":"2016-04-21T01:02:03Z"}.*],"ready":false,"spawn-time":"2016-04-21T01:02:03Z","path":"/home/user/test"}.*`)
}

func (s *apiSuite) TestStateChangesAll(c *check.C) {
	restore := state.MockTime(time.Date(2016, 04, 21, 1, 2, 3, 0, time.UTC))
	defer restore()

	// Setup
	d := s.daemon(c)
	st := d.overlord.State()
	st.Lock()
	setupChanges(st)
	st.Unlock()

	stateChangesCmd := apiCmd("/v1/changes")

	// Execute
	req, err := http.NewRequest("GET", "/v1/changes?select=all", nil)
	c.Assert(err, check.IsNil)
	rsp := v1GetChanges(stateChangesCmd, req, nil).(*resp)

	// Verify
	c.Check(rsp.Status, check.Equals, 200)
	c.Assert(rsp.Result, check.HasLen, 2)

	res, err := rsp.MarshalJSON()
	c.Assert(err, check.IsNil)

	c.Check(string(res), check.Matches, `.*{"id":"\w+","kind":"launch","summary":"launch...","status":"Do","tasks":\[{"id":"\w+","kind":"create-workspace","summary":"1...","status":"Do","log":\["2016-04-21T01:02:03Z INFO l11","2016-04-21T01:02:03Z INFO l12"],"progress":{"label":"","done":0,"total":1},"spawn-time":"2016-04-21T01:02:03Z"}.*],"ready":false,"spawn-time":"2016-04-21T01:02:03Z","path":"/home/user/test"}.*`)
	c.Check(string(res), check.Matches, `.*{"id":"\w+","kind":"remove","summary":"remove...","status":"Error","tasks":\[{"id":"\w+","kind":"unlink-sdk","summary":"1...","status":"Error","log":\["2016-04-21T01:02:03Z ERROR unlink failed"],"progress":{"label":"","done":1,"total":1},"spawn-time":"2016-04-21T01:02:03Z","ready-time":"2016-04-21T01:02:03Z"}.*],"ready":true,"err":"[^"]+".*`)
}

func (s *apiSuite) TestStateChangesReady(c *check.C) {
	restore := state.MockTime(time.Date(2016, 04, 21, 1, 2, 3, 0, time.UTC))
	defer restore()

	// Setup
	d := s.daemon(c)
	st := d.overlord.State()
	st.Lock()
	setupChanges(st)
	st.Unlock()

	stateChangesCmd := apiCmd("/v1/changes")

	// Execute
	req, err := http.NewRequest("GET", "/v1/changes?select=ready", nil)
	c.Assert(err, check.IsNil)
	rsp := v1GetChanges(stateChangesCmd, req, nil).(*resp)

	// Verify
	c.Check(rsp.Status, check.Equals, 200)
	c.Assert(rsp.Result, check.HasLen, 1)

	res, err := rsp.MarshalJSON()
	c.Assert(err, check.IsNil)

	c.Check(string(res), check.Matches, `.*{"id":"\w+","kind":"remove","summary":"remove...","status":"Error","tasks":\[{"id":"\w+","kind":"unlink-sdk","summary":"1...","status":"Error","log":\["2016-04-21T01:02:03Z ERROR unlink failed"],"progress":{"label":"","done":1,"total":1},"spawn-time":"2016-04-21T01:02:03Z","ready-time":"2016-04-21T01:02:03Z"}.*],"ready":true,"err":"[^"]+".*`)
}

func (s *apiSuite) TestStateChangesForWorkspace(c *check.C) {
	restore := state.MockTime(time.Date(2016, 04, 21, 1, 2, 3, 0, time.UTC))
	defer restore()

	// Setup
	d := s.daemon(c)
	st := d.overlord.State()
	st.Lock()
	setupChanges(st)
	st.Unlock()

	stateChangesCmd := apiCmd("/v1/changes")

	// Execute
	req, err := http.NewRequest("GET", "/v1/changes?workspaces=one,two&project-id=123", nil)
	c.Assert(err, check.IsNil)
	rsp := v1GetChanges(stateChangesCmd, req, nil).(*resp)

	// Verify
	c.Check(rsp.Type, check.Equals, ResponseTypeSync)
	c.Check(rsp.Status, check.Equals, 200)
	c.Assert(rsp.Result, check.FitsTypeOf, []*changeInfo(nil))

	res := rsp.Result.([]*changeInfo)
	// sort the result to ensure the order
	slices.SortFunc(res, func(a, b *changeInfo) bool { return a.Kind < b.Kind })
	c.Assert(res, check.HasLen, 2)
	c.Check(res[0].Kind, check.Equals, "launch")
	c.Check(res[1].Kind, check.Equals, "remove")
	c.Check(res[0].Project, check.Equals, "/home/user/test")
	c.Check(res[1].Project, check.Equals, "/home/user/test")

	_, err = rsp.MarshalJSON()
	c.Assert(err, check.IsNil)
}

func (s *apiSuite) TestStateChange(c *check.C) {
	restore := state.MockTime(time.Date(2016, 04, 21, 1, 2, 3, 0, time.UTC))
	defer restore()

	// Setup
	d := s.daemon(c)
	st := d.overlord.State()
	st.Lock()
	ids := setupChanges(st)
	chg := st.Change(ids[0])
	chg.Set("api-data", map[string]int{"n": 42})
	task := chg.Tasks()[0]
	task.Set("api-data", map[string]string{"foo": "bar"})
	st.Unlock()
	s.vars = map[string]string{"id": ids[0]}

	stateChangeCmd := apiCmd("/v1/changes/{id}")

	// Execute
	req, err := http.NewRequest("POST", "/v1/change/"+ids[0], nil)
	c.Assert(err, check.IsNil)
	rsp := v1GetChange(stateChangeCmd, req, nil).(*resp)
	rec := httptest.NewRecorder()
	rsp.ServeHTTP(rec, req)

	// Verify
	c.Check(rec.Code, check.Equals, 200)
	c.Check(rsp.Status, check.Equals, 200)
	c.Check(rsp.Type, check.Equals, ResponseTypeSync)
	c.Check(rsp.Result, check.NotNil)

	var body map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &body)
	c.Check(err, check.IsNil)
	c.Check(body["result"], check.DeepEquals, map[string]interface{}{
		"id":         ids[0],
		"kind":       "launch",
		"summary":    "launch...",
		"status":     "Do",
		"ready":      false,
		"spawn-time": "2016-04-21T01:02:03Z",
		"path":       "/home/user/test",
		"tasks": []interface{}{
			map[string]interface{}{
				"id":         ids[2],
				"kind":       "create-workspace",
				"summary":    "1...",
				"status":     "Do",
				"log":        []interface{}{"2016-04-21T01:02:03Z INFO l11", "2016-04-21T01:02:03Z INFO l12"},
				"progress":   map[string]interface{}{"label": "", "done": 0., "total": 1.},
				"spawn-time": "2016-04-21T01:02:03Z",
				"data": map[string]interface{}{
					"foo": "bar",
				},
			},
			map[string]interface{}{
				"id":         ids[3],
				"kind":       "start-workspace",
				"summary":    "2...",
				"status":     "Do",
				"progress":   map[string]interface{}{"label": "", "done": 0., "total": 1.},
				"spawn-time": "2016-04-21T01:02:03Z",
			},
		},
		"data": map[string]interface{}{
			"n": float64(42),
		},
	})
}

func (s *apiSuite) TestStateChangeAbort(c *check.C) {
	restore := state.MockTime(time.Date(2016, 04, 21, 1, 2, 3, 0, time.UTC))
	defer restore()

	soon := 0
	restore = FakeStateEnsureBefore(func(st *state.State, d time.Duration) {
		soon++
	})
	defer restore()

	// Setup
	d := s.daemon(c)
	st := d.overlord.State()
	st.Lock()
	ids := setupChanges(st)
	st.Unlock()
	s.vars = map[string]string{"id": ids[0]}

	buf := bytes.NewBufferString(`{"action": "abort"}`)

	stateChangeCmd := apiCmd("/v1/changes/{id}")

	// Execute
	req, err := http.NewRequest("POST", "/v1/changes/"+ids[0], buf)
	c.Assert(err, check.IsNil)
	rsp := v1PostChange(stateChangeCmd, req, nil).(*resp)
	rec := httptest.NewRecorder()
	rsp.ServeHTTP(rec, req)

	// Ensure scheduled
	c.Check(soon, check.Equals, 1)

	// Verify
	c.Check(rec.Code, check.Equals, 200)
	c.Check(rsp.Status, check.Equals, 200)
	c.Check(rsp.Type, check.Equals, ResponseTypeSync)
	c.Check(rsp.Result, check.NotNil)

	var body map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &body)
	c.Check(err, check.IsNil)
	c.Check(body["result"], check.DeepEquals, map[string]interface{}{
		"id":         ids[0],
		"kind":       "launch",
		"summary":    "launch...",
		"status":     "Hold",
		"ready":      true,
		"spawn-time": "2016-04-21T01:02:03Z",
		"ready-time": "2016-04-21T01:02:03Z",
		"path":       "/home/user/test",
		"tasks": []interface{}{
			map[string]interface{}{
				"id":         ids[2],
				"kind":       "create-workspace",
				"summary":    "1...",
				"status":     "Hold",
				"log":        []interface{}{"2016-04-21T01:02:03Z INFO l11", "2016-04-21T01:02:03Z INFO l12"},
				"progress":   map[string]interface{}{"label": "", "done": 1., "total": 1.},
				"spawn-time": "2016-04-21T01:02:03Z",
				"ready-time": "2016-04-21T01:02:03Z",
			},
			map[string]interface{}{
				"id":         ids[3],
				"kind":       "start-workspace",
				"summary":    "2...",
				"status":     "Hold",
				"progress":   map[string]interface{}{"label": "", "done": 1., "total": 1.},
				"spawn-time": "2016-04-21T01:02:03Z",
				"ready-time": "2016-04-21T01:02:03Z",
			},
		},
	})
}

func (s *apiSuite) TestStateChangeAbortIsReady(c *check.C) {
	restore := state.MockTime(time.Date(2016, 04, 21, 1, 2, 3, 0, time.UTC))
	defer restore()

	// Setup
	d := s.daemon(c)
	st := d.overlord.State()
	st.Lock()
	ids := setupChanges(st)
	st.Change(ids[0]).SetStatus(state.DoneStatus)
	st.Unlock()
	s.vars = map[string]string{"id": ids[0]}

	buf := bytes.NewBufferString(`{"action": "abort"}`)

	stateChangeCmd := apiCmd("/v1/changes/{id}")

	// Execute
	req, err := http.NewRequest("POST", "/v1/changes/"+ids[0], buf)
	c.Assert(err, check.IsNil)
	rsp := v1PostChange(stateChangeCmd, req, nil).(*resp)
	rec := httptest.NewRecorder()
	rsp.ServeHTTP(rec, req)

	// Verify
	c.Check(rec.Code, check.Equals, 400)
	c.Check(rsp.Status, check.Equals, 400)
	c.Check(rsp.Type, check.Equals, ResponseTypeError)
	c.Check(rsp.Result, check.NotNil)

	var body map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &body)
	c.Check(err, check.IsNil)
	c.Check(body["result"], check.DeepEquals, map[string]interface{}{
		"message": fmt.Sprintf("cannot abort change %s with nothing pending", ids[0]),
	})
}
