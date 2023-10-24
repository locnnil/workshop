// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2018 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package ifacestate_test

import (
	"testing"

	"github.com/canonical/workshop/internal/overlord/ifacestate"
	"github.com/canonical/workshop/internal/overlord/ifacestate/schema"
	"github.com/canonical/workshop/internal/overlord/state"

	"gopkg.in/check.v1"
)

type helpersSuite struct {
	st *state.State
}

func Test(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&helpersSuite{})

func (s *helpersSuite) SetUpTest(c *check.C) {
	s.st = state.New(nil)
}

func (s *helpersSuite) TestGetConns(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()
	s.st.Set("conns", map[string]interface{}{
		"ws:app:content ws:core:content": map[string]interface{}{
			"auto":      true,
			"interface": "content",
			"slot-static": map[string]interface{}{
				"number": int(78),
			},
		},
	})

	conns, err := ifacestate.GetConns(s.st)
	c.Assert(err, check.IsNil)
	for id, connState := range conns {
		c.Assert(id, check.Equals, "ws:app:content ws:core:content")
		c.Assert(connState.Auto, check.Equals, true)
		c.Assert(connState.Interface, check.Equals, "content")
		c.Assert(connState.StaticSlotAttrs["number"], check.Equals, int64(78))
	}
}

func (s *helpersSuite) TestSetConns(c *check.C) {
	s.st.Lock()
	defer s.st.Unlock()

	conns := map[string]*schema.ConnState{
		"ws:app:content ws:core:content": {Auto: true, Interface: "content"},
	}

	ifacestate.SetConns(s.st, conns)
	var readconns map[string]interface{}
	err := s.st.Get("conns", &readconns)
	c.Assert(err, check.IsNil)
	c.Assert(readconns, check.DeepEquals, map[string]interface{}{
		"ws:app:content ws:core:content": map[string]interface{}{
			"auto":      true,
			"interface": "content",
		}})
}
