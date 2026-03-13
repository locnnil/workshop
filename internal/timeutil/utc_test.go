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

package timeutil_test

import (
	"encoding/json"
	"time"

	"gopkg.in/check.v1"
	"gopkg.in/yaml.v3"

	"github.com/canonical/workshop/internal/timeutil"
)

type utcSuite struct{}

var _ = check.Suite(&utcSuite{})

func (utcSuite) TestMarshalText(c *check.C) {
	t0 := "2025-04-03T02:01:00.123456Z"

	t1, err := timeutil.TimeUTC(time.Date(2025, 4, 3, 2, 1, 0, 123456000, time.UTC)).MarshalText()
	c.Assert(err, check.IsNil)
	c.Check(string(t1), check.Equals, t0)

	t2, err := timeutil.TimeUTC(time.Date(2025, 4, 3, 3, 1, 0, 123456000, time.FixedZone("test", 3600))).MarshalText()
	c.Assert(err, check.IsNil)
	c.Check(string(t2), check.Equals, t0)
}

func (utcSuite) TestUnmarshalText(c *check.C) {
	t0 := timeutil.TimeUTC(time.Date(2025, 4, 3, 2, 1, 0, 123456000, time.UTC))

	var t1 timeutil.TimeUTC
	err := t1.UnmarshalText([]byte("2025-04-03T02:01:00.123456Z"))
	c.Assert(err, check.IsNil)
	c.Check(t1, check.Equals, t0)

	var t2 timeutil.TimeUTC
	err = t2.UnmarshalText([]byte("2025-04-03T02:01:00.123456+00:00"))
	c.Assert(err, check.IsNil)
	c.Check(t2, check.Equals, t0)

	var t3 timeutil.TimeUTC
	err = t3.UnmarshalText([]byte("2025-04-03T03:01:00.123456+01:00"))
	c.Assert(err, check.IsNil)
	c.Check(t3, check.Equals, t0)
}

func (utcSuite) TestMarshalJSON(c *check.C) {
	t0 := `"2025-04-03T02:01:00.123456Z"`

	t1, err := json.Marshal(timeutil.TimeUTC(time.Date(2025, 4, 3, 2, 1, 0, 123456000, time.UTC)))
	c.Assert(err, check.IsNil)
	c.Check(string(t1), check.Equals, t0)

	t2, err := json.Marshal(timeutil.TimeUTC(time.Date(2025, 4, 3, 3, 1, 0, 123456000, time.FixedZone("test", 3600))))
	c.Assert(err, check.IsNil)
	c.Check(string(t2), check.Equals, t0)
}

func (utcSuite) TestUnmarshalJSON(c *check.C) {
	t0 := timeutil.TimeUTC(time.Date(2025, 4, 3, 2, 1, 0, 123456000, time.UTC))

	var t1 timeutil.TimeUTC
	err := json.Unmarshal([]byte(`"2025-04-03T02:01:00.123456Z"`), &t1)
	c.Assert(err, check.IsNil)
	c.Check(t1, check.Equals, t0)

	var t2 timeutil.TimeUTC
	err = json.Unmarshal([]byte(`"2025-04-03T02:01:00.123456+00:00"`), &t2)
	c.Assert(err, check.IsNil)
	c.Check(t2, check.Equals, t0)

	var t3 timeutil.TimeUTC
	err = json.Unmarshal([]byte(`"2025-04-03T03:01:00.123456+01:00"`), &t3)
	c.Assert(err, check.IsNil)
	c.Check(t3, check.Equals, t0)
}

func (utcSuite) TestMarshalYAML(c *check.C) {
	t0 := "2025-04-03T02:01:00.123456Z\n"

	t1, err := yaml.Marshal(timeutil.TimeUTC(time.Date(2025, 4, 3, 2, 1, 0, 123456000, time.UTC)))
	c.Assert(err, check.IsNil)
	c.Check(string(t1), check.Equals, t0)

	t2, err := yaml.Marshal(timeutil.TimeUTC(time.Date(2025, 4, 3, 3, 1, 0, 123456000, time.FixedZone("test", 3600))))
	c.Assert(err, check.IsNil)
	c.Check(string(t2), check.Equals, t0)
}

func (utcSuite) TestUnmarshalYAML(c *check.C) {
	t0 := timeutil.TimeUTC(time.Date(2025, 4, 3, 2, 1, 0, 123456000, time.UTC))

	var t1 timeutil.TimeUTC
	err := yaml.Unmarshal([]byte("2025-04-03T02:01:00.123456Z"), &t1)
	c.Assert(err, check.IsNil)
	c.Check(t1, check.Equals, t0)

	var t2 timeutil.TimeUTC
	err = yaml.Unmarshal([]byte("2025-04-03T02:01:00.123456+00:00"), &t2)
	c.Assert(err, check.IsNil)
	c.Check(t2, check.Equals, t0)

	var t3 timeutil.TimeUTC
	err = yaml.Unmarshal([]byte("2025-04-03T03:01:00.123456+01:00"), &t3)
	c.Assert(err, check.IsNil)
	c.Check(t3, check.Equals, t0)
}
