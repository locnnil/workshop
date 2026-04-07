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

package sdk_test

import (
	"strings"

	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/sdk"
)

type sdkChannel struct{}

var _ = check.Suite(&sdkChannel{})

func (sdkChannel) TestParse(c *check.C) {
	ch, err := sdk.ParseChannel("stable")
	c.Assert(err, check.IsNil)
	c.Check(ch, check.DeepEquals, sdk.Channel{
		Name:   "stable",
		Track:  "",
		Risk:   "stable",
		Branch: "",
	})

	ch, err = sdk.ParseChannel("latest/stable")
	c.Assert(err, check.IsNil)
	c.Check(ch, check.DeepEquals, sdk.Channel{
		Name:   "latest/stable",
		Track:  "latest",
		Risk:   "stable",
		Branch: "",
	})

	ch, err = sdk.ParseChannel("1.0/edge")
	c.Assert(err, check.IsNil)
	c.Check(ch, check.DeepEquals, sdk.Channel{
		Name:   "1.0/edge",
		Track:  "1.0",
		Risk:   "edge",
		Branch: "",
	})

	ch, err = sdk.ParseChannel("1.0")
	c.Assert(err, check.IsNil)
	c.Check(ch, check.DeepEquals, sdk.Channel{
		Name:   "1.0",
		Track:  "1.0",
		Risk:   "",
		Branch: "",
	})

	ch, err = sdk.ParseChannel("1.0/beta/foo")
	c.Assert(err, check.IsNil)
	c.Check(ch, check.DeepEquals, sdk.Channel{
		Name:   "1.0/beta/foo",
		Track:  "1.0",
		Risk:   "beta",
		Branch: "foo",
	})

	ch, err = sdk.ParseChannel("candidate/foo")
	c.Assert(err, check.IsNil)
	c.Check(ch, check.DeepEquals, sdk.Channel{
		Name:   "candidate/foo",
		Track:  "",
		Risk:   "candidate",
		Branch: "foo",
	})
}

func (sdkChannel) TestParseErrors(c *check.C) {
	for _, tc := range []struct {
		channel string
		err     string
	}{
		{"", `invalid track "" in channel ""`},
		{strings.Repeat("a", 100), `invalid track "a*" in channel "a*"`},
		{"!@#", `invalid track "!@#" in channel "!@#"`},
		{"/edge", `invalid track "" in channel "/edge"`},
		{strings.Repeat("a", 100) + "/beta", `invalid track "a*" in channel "a*/beta"`},
		{"!@#/stable", `invalid track "!@#" in channel "!@#/stable"`},
		{"1.0/cand", `invalid risk "cand" in channel "1.0/cand"`},
		{"beta/", `invalid branch "" in channel "beta/"`},
		{"edge/" + strings.Repeat("a", 200), `invalid branch "a*" in channel "edge/a*"`},
		{"stable/!@#", `invalid branch "!@#" in channel "stable/!@#"`},
		{"candidate/edge", `invalid branch "edge" in channel "candidate/edge"`},
		{"/stable/fix", `invalid track "" in channel "/stable/fix"`},
		{strings.Repeat("a", 100) + "/edge/branch", `invalid track "a*" in channel "a*/edge/branch"`},
		{"!@#/beta/temp", `invalid track "!@#" in channel "!@#/beta/temp"`},
		{"beta/edge/fix", `invalid track "beta" in channel "beta/edge/fix"`},
		{"1.0//branch", `invalid risk "" in channel "1.0//branch"`},
		{"track/edge/", `invalid branch "" in channel "track/edge/"`},
		{"2022/beta/" + strings.Repeat("a", 200), `invalid branch "a*" in channel "2022/beta/a*"`},
		{"latest/stable/!@#", `invalid branch "!@#" in channel "latest/stable/!@#"`},
		{"0.9/candidate/edge", `invalid branch "edge" in channel "0.9/candidate/edge"`},
		{"///", `channel "///" has too many components`},
	} {
		_, err := sdk.ParseChannel(tc.channel)
		c.Check(err, check.ErrorMatches, tc.err)
	}
}

func (sdkChannel) TestChannelFull(c *check.C) {
	tests := []struct {
		channel string
		name    string
		track   string
		risk    string
	}{
		{"stable", "latest/stable", "latest", "stable"},
		{"latest/stable", "latest/stable", "latest", "stable"},
		{"1.0/edge", "1.0/edge", "1.0", "edge"},
		{"1.0/beta/foo", "1.0/beta/foo", "1.0", "beta"},
		{"1.0", "1.0/stable", "1.0", "stable"},
		{"candidate/foo", "latest/candidate/foo", "latest", "candidate"},
	}

	for _, t := range tests {
		ch, err := sdk.ParseChannel(t.channel)
		c.Assert(err, check.IsNil)

		full := ch.Full()
		c.Check(full.Name, check.Equals, t.name)
		c.Check(full.Track, check.Equals, t.track)
		c.Check(full.Risk, check.Equals, t.risk)
	}
}
