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

import "strings"

func LookupAttr(static, dynamic map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '.' })
	if len(parts) == 0 {
		return nil, false
	}

	var v interface{}
	if _, ok := dynamic[parts[0]]; ok {
		v = dynamic
	} else {
		v = static
	}

	for _, part := range parts {
		m, ok := v.(map[string]interface{})
		if !ok {
			return nil, false
		}
		v, ok = m[part]
		if !ok {
			return nil, false
		}
	}

	return v, true
}
