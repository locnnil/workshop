// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2015 Canonical Ltd
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

package asserts_test

import (
	"gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/asserts"
)

type headersSuite struct{}

var _ = check.Suite(&headersSuite{})

func (s *headersSuite) TestParseHeadersSimple(c *check.C) {
	m, err := asserts.ParseHeaders([]byte(`foo: 1
bar: baz`))
	c.Assert(err, check.IsNil)
	c.Check(m, check.DeepEquals, map[string]interface{}{
		"foo": "1",
		"bar": "baz",
	})
}

func (s *headersSuite) TestParseHeadersMultiline(c *check.C) {
	m, err := asserts.ParseHeaders([]byte(`foo:
    abc
    
bar: baz`))
	c.Assert(err, check.IsNil)
	c.Check(m, check.DeepEquals, map[string]interface{}{
		"foo": "abc\n",
		"bar": "baz",
	})

	m, err = asserts.ParseHeaders([]byte(`foo: 1
bar:
    baz`))
	c.Assert(err, check.IsNil)
	c.Check(m, check.DeepEquals, map[string]interface{}{
		"foo": "1",
		"bar": "baz",
	})

	m, err = asserts.ParseHeaders([]byte(`foo: 1
bar:
    baz
    `))
	c.Assert(err, check.IsNil)
	c.Check(m, check.DeepEquals, map[string]interface{}{
		"foo": "1",
		"bar": "baz\n",
	})

	m, err = asserts.ParseHeaders([]byte(`foo: 1
bar:
    baz
    
    baz2`))
	c.Assert(err, check.IsNil)
	c.Check(m, check.DeepEquals, map[string]interface{}{
		"foo": "1",
		"bar": "baz\n\nbaz2",
	})
}

func (s *headersSuite) TestParseHeadersSimpleList(c *check.C) {
	m, err := asserts.ParseHeaders([]byte(`foo:
  - x
  - y
  - z
bar: baz`))
	c.Assert(err, check.IsNil)
	c.Check(m, check.DeepEquals, map[string]interface{}{
		"foo": []interface{}{"x", "y", "z"},
		"bar": "baz",
	})
}

func (s *headersSuite) TestParseHeadersListNestedMultiline(c *check.C) {
	m, err := asserts.ParseHeaders([]byte(`foo:
  - x
  -
      y1
      y2
      
  - z
bar: baz`))
	c.Assert(err, check.IsNil)
	c.Check(m, check.DeepEquals, map[string]interface{}{
		"foo": []interface{}{"x", "y1\ny2\n", "z"},
		"bar": "baz",
	})

	m, err = asserts.ParseHeaders([]byte(`bar: baz
foo:
  -
    - u1
    - u2
  -
      y1
      y2
      `))
	c.Assert(err, check.IsNil)
	c.Check(m, check.DeepEquals, map[string]interface{}{
		"foo": []interface{}{[]interface{}{"u1", "u2"}, "y1\ny2\n"},
		"bar": "baz",
	})
}

func (s *headersSuite) TestParseHeadersSimpleMap(c *check.C) {
	m, err := asserts.ParseHeaders([]byte(`foo:
  x: X
  yy: YY
  z5: 
bar: baz`))
	c.Assert(err, check.IsNil)
	c.Check(m, check.DeepEquals, map[string]interface{}{
		"foo": map[string]interface{}{
			"x":  "X",
			"yy": "YY",
			"z5": "",
		},
		"bar": "baz",
	})
}

func (s *headersSuite) TestParseHeadersMapNestedMultiline(c *check.C) {
	m, err := asserts.ParseHeaders([]byte(`foo:
  x: X
  yy:
      YY1
      YY2
  u:
    - u1
    - u2
bar: baz`))
	c.Assert(err, check.IsNil)
	c.Check(m, check.DeepEquals, map[string]interface{}{
		"foo": map[string]interface{}{
			"x":  "X",
			"yy": "YY1\nYY2",
			"u":  []interface{}{"u1", "u2"},
		},
		"bar": "baz",
	})

	m, err = asserts.ParseHeaders([]byte(`one:
  two:
    three: `))
	c.Assert(err, check.IsNil)
	c.Check(m, check.DeepEquals, map[string]interface{}{
		"one": map[string]interface{}{
			"two": map[string]interface{}{
				"three": "",
			},
		},
	})

	m, err = asserts.ParseHeaders([]byte(`one:
  two:
      three`))
	c.Assert(err, check.IsNil)
	c.Check(m, check.DeepEquals, map[string]interface{}{
		"one": map[string]interface{}{
			"two": "three",
		},
	})

	m, err = asserts.ParseHeaders([]byte(`map-within-map:
  lev1:
    lev2: x`))
	c.Assert(err, check.IsNil)
	c.Check(m, check.DeepEquals, map[string]interface{}{
		"map-within-map": map[string]interface{}{
			"lev1": map[string]interface{}{
				"lev2": "x",
			},
		},
	})

	m, err = asserts.ParseHeaders([]byte(`list-of-maps:
  -
    entry: foo
    bar: baz
  -
    entry: bar`))
	c.Assert(err, check.IsNil)
	c.Check(m, check.DeepEquals, map[string]interface{}{
		"list-of-maps": []interface{}{
			map[string]interface{}{
				"entry": "foo",
				"bar":   "baz",
			},
			map[string]interface{}{
				"entry": "bar",
			},
		},
	})
}

func (s *headersSuite) TestParseHeadersMapErrors(c *check.C) {
	_, err := asserts.ParseHeaders([]byte(`foo:
  x X
bar: baz`))
	c.Check(err, check.ErrorMatches, `map entry missing ':' separator: "x X"`)

	_, err = asserts.ParseHeaders([]byte(`foo:
  0x: X
bar: baz`))
	c.Check(err, check.ErrorMatches, `invalid map entry key: "0x"`)

	_, err = asserts.ParseHeaders([]byte(`foo:
  a: a
  a: b`))
	c.Check(err, check.ErrorMatches, `repeated map entry: "a"`)
}

func (s *headersSuite) TestParseHeadersErrors(c *check.C) {
	_, err := asserts.ParseHeaders([]byte(`foo: 1
bar:baz`))
	c.Check(err, check.ErrorMatches, `header entry should have a space or newline \(for multiline\) before value: "bar:baz"`)

	_, err = asserts.ParseHeaders([]byte(`foo:
 - x
  - y
  - z
bar: baz`))
	c.Check(err, check.ErrorMatches, `expected 4 chars nesting prefix after multiline introduction "foo:": " - x"`)

	_, err = asserts.ParseHeaders([]byte(`foo:
  - x
  - y
  - z
bar:`))
	c.Check(err, check.ErrorMatches, `expected 4 chars nesting prefix after multiline introduction "bar:": EOF`)
}
