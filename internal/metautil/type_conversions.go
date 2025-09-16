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

package metautil

import (
	"errors"
	"fmt"
	"math"
	"reflect"
)

func convertValue(value reflect.Value, outputType reflect.Type) (reflect.Value, error) {
	inputType := value.Type()
	if inputType == outputType {
		return value, nil
	}

	var nullValue reflect.Value
	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		if outputType.Kind() != reflect.Array && outputType.Kind() != reflect.Slice {
			break
		}
		outputValue := reflect.MakeSlice(outputType, 0, value.Len())
		for i := range value.Len() {
			convertedElem, err := convertValue(value.Index(i), outputType.Elem())
			if err != nil {
				return nullValue, err
			}
			outputValue = reflect.Append(outputValue, convertedElem)
		}
		return outputValue, nil
	case reflect.Interface:
		return convertValue(value.Elem(), outputType)
	case reflect.Map:
		if outputType.Kind() != reflect.Map {
			break
		}
		outputValue := reflect.MakeMapWithSize(outputType, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			convertedKey, err := convertValue(iter.Key(), outputType.Key())
			if err != nil {
				return nullValue, err
			}
			convertedValue, err := convertValue(iter.Value(), outputType.Elem())
			if err != nil {
				return nullValue, err
			}
			outputValue.SetMapIndex(convertedKey, convertedValue)
		}
		return outputValue, nil
	case reflect.Float64:
		// Special case: encoding/json can mangle int64 -> float64.
		v, ok := maybeFloatToInt(value.Float())
		if ok && outputType.Kind() == reflect.Int64 {
			return reflect.ValueOf(v), nil
		}
	}
	return nullValue, fmt.Errorf(`cannot convert value "%v" into a %v`, value, outputType)
}

func maybeFloatToInt(v float64) (int64, bool) {
	// Recall that -0.0 == 0.0 in Go (and IEEE 754).
	if _, frac := math.Modf(v); frac != 0 {
		return 0, false
	}
	if v < float64(math.MinInt64) || v > float64(math.MaxInt64) {
		return 0, false
	}
	return int64(v), true
}

// SetValueFromAttribute attempts to convert the attribute value read from the
// given sdk/interface into the desired type. This function only
// operates converting the attrVal parameter into a value which can fit into
// the val parameter, which therefore must be a pointer.
func SetValueFromAttribute(attrVal any, val any) error {
	rt := reflect.TypeOf(val)
	if rt.Kind() != reflect.Pointer || val == nil {
		return errors.New("internal error: value must be a pointer")
	}

	converted, err := convertValue(reflect.ValueOf(attrVal), rt.Elem())
	if err != nil {
		return fmt.Errorf("expected %s but found %s", nameOrString(rt.Elem()), nameOrString(reflect.TypeOf(attrVal)))
	}

	reflect.ValueOf(val).Elem().Set(converted)
	return nil
}

func nameOrString(t reflect.Type) string {
	name := t.Name()
	if name != "" {
		return name
	}
	return t.String()
}
