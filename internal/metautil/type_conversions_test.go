// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2022 Canonical Ltd
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

package metautil_test

import (
	"math"
	"reflect"

	. "gopkg.in/check.v1"

	"github.com/canonical/workshop/internal/metautil"
)

type conversionssSuite struct{}

var _ = Suite(&conversionssSuite{})

func (s *conversionssSuite) TestConvertHappy(c *C) {
	data := []struct {
		inputValue    any
		expectedValue any
	}{
		// Basic types
		{"a string", "a string"},
		{42, 42},
		{true, true},

		// Special case of float64 -> int64.
		{float64(42.0), int64(42)},
		{float64(-42.0), int64(-42)},

		// Complex types with no conversion
		{[]string{"one", "two"}, []string{"one", "two"}},
		{[]int{24, 42}, []int{24, 42}},

		// Complex types with conversion
		{[]any{"one", "two"}, []string{"one", "two"}},
		{[]any{24, 42}, []int{24, 42}},
		{[]any{[]string{"one"}, []string{"two"}}, [][]string{{"one"}, {"two"}}},
		{[]any{map[string]int{"one": 1}, map[string]int{"two": 2}}, []map[string]int{{"one": 1}, {"two": 2}}},
		{map[any]any{"one": 1, "two": 2}, map[string]int{"one": 1, "two": 2}},
	}

	for _, testData := range data {
		inputValue := reflect.ValueOf(testData.inputValue)
		outputType := reflect.TypeOf(testData.expectedValue)
		expectedValue := testData.expectedValue
		outputValue, err := metautil.ConvertValue(inputValue, outputType)
		testTag := Commentf("%v -> %v", inputValue, expectedValue)
		c.Check(err, IsNil, testTag)
		c.Check(outputValue.Interface(), DeepEquals, expectedValue, testTag)
	}
}

func (s *conversionssSuite) TestConvertUnhappy(c *C) {
	t := reflect.TypeOf
	data := []struct {
		inputValue    any
		outputType    reflect.Type
		expectedError string
	}{
		// Basic types
		{"a string", t(42), `cannot convert value "a string" into a int`},
		{true, t(""), `cannot convert value "true" into a string`},

		// Special case of float64 -> int64.
		{float64(4.2), t(int64(0)), `cannot convert value "4.2" into a int64`},
		{math.Nextafter(math.MinInt64, math.Inf(-1)), t(int64(0)), `cannot convert value "-[0-9.]*e\+18" into a int64`},
		{math.Nextafter(math.MaxInt64, math.Inf(1)), t(int64(0)), `cannot convert value "[0-9.]*e\+18" into a int64`},

		// Complex types
		{[]any{"one", "two", 3}, t([]string{}), `cannot convert value "3" into a string`},
		{[]any{1, "two", 3}, t([]int{}), `cannot convert value "two" into a int`},
		{[]int{1, 2}, t([]string{}), `cannot convert value "1" into a string`},
		{[]int{1, 2}, t(1), `cannot convert value "\[1 2\]" into a int`},
		{map[any]any{"one": 1}, t(map[int]int{}), `cannot convert value "one" into a int`},
		{map[any]any{1: 2}, t(map[int]string{}), `cannot convert value "2" into a string`},
		{map[any]any{"one": 1}, t([]string{}), `cannot convert value "map\[one:1\]" into a \[\]string`},
	}

	for _, testData := range data {
		inputValue := reflect.ValueOf(testData.inputValue)
		outputType := testData.outputType
		expectedError := testData.expectedError
		outputValue, err := metautil.ConvertValue(inputValue, outputType)
		testTag := Commentf("%v -> %T", inputValue, outputType)
		c.Check(err, ErrorMatches, expectedError, testTag)
		c.Check(outputValue.IsValid(), Equals, false, testTag)
	}
}

func (s *conversionssSuite) TestSetValueFromAttributeHappy(c *C) {
	interfaceArray := []any{12, -3}
	var outputValue []int
	err := metautil.SetValueFromAttribute(interfaceArray, &outputValue)
	c.Assert(err, IsNil)
	c.Check(outputValue, DeepEquals, []int{12, -3})
}

func (s *conversionssSuite) TestSetValueFromAttributeUnhappy(c *C) {
	var outputBool bool
	data := []struct {
		inputValue    any
		outputValue   any
		expectedError string
	}{
		// error if output value parameter is not a pointer
		{
			"input value",
			"I'm not a pointer",
			"internal error: value must be a pointer",
		},

		// error if value cannot be converted
		{
			"input value",
			&outputBool,
			`expected bool but found string`,
		},
	}

	for _, td := range data {
		err := metautil.SetValueFromAttribute(td.inputValue, td.outputValue)
		c.Check(err, ErrorMatches, td.expectedError, Commentf("input value %v", td.inputValue))
	}
}
