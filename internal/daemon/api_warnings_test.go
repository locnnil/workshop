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

package daemon

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"time"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/overlord/state"
)

func (s *apiSuite) testWarnings(c *check.C, all bool, body io.Reader) (calls string, result any) {
	s.daemon(c)

	okayWarns := func(*state.State, time.Time) int { calls += "ok"; return 0 }
	allWarns := func(*state.State) []*state.Warning { calls += "all"; return nil }
	pendingWarns := func(*state.State) ([]*state.Warning, time.Time) { calls += "show"; return nil, time.Time{} }
	restore := MockWarningsAccessors(okayWarns, allWarns, pendingWarns)
	defer restore()

	method := "GET"
	handler := v1GetWarnings
	if body != nil {
		method = "POST"
		handler = v1PostWarnings
	}
	q := url.Values{}
	if all {
		q.Set("select", "all")
	}
	cmd := apiCmd("/v1/warnings")

	req, err := http.NewRequest(method, "/v1/warnings?"+q.Encode(), body)
	c.Assert(err, check.IsNil)

	rsp := handler(cmd, req.WithContext(s.ctx), nil)

	c.Check(rsp.(*resp).Status, check.Equals, 200)
	c.Assert(rsp.(*resp).Result, check.NotNil)
	return calls, rsp.(*resp).Result
}

func (s *apiSuite) TestAllWarnings(c *check.C) {
	calls, result := s.testWarnings(c, true, nil)
	c.Check(calls, check.Equals, "all")
	c.Check(result, check.DeepEquals, []state.Warning{})
}

func (s *apiSuite) TestSomeWarnings(c *check.C) {
	calls, result := s.testWarnings(c, false, nil)
	c.Check(calls, check.Equals, "show")
	c.Check(result, check.DeepEquals, []state.Warning{})
}

func (s *apiSuite) TestAckWarnings(c *check.C) {
	calls, result := s.testWarnings(c, false, bytes.NewReader([]byte(`{"action": "okay", "timestamp": "2006-01-02T15:04:05Z"}`)))
	c.Check(calls, check.Equals, "ok")
	c.Check(result, check.DeepEquals, 0)
}
