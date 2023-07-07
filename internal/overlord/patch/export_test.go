// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016 Canonical Ltd
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

package patch

// PatchesForTest returns the registered set of patches for testing purposes.
func PatchesForTest() map[int][]PatchFunc {
	return patches
}

// MockLevel replaces the current implemented patch level
func MockLevel(lv, sublvl int) (restorer func()) {
	old := Level
	Level = lv
	oldSublvl := Sublevel
	Sublevel = sublvl
	oldPatches := make(map[int][]PatchFunc)
	for k, v := range patches {
		oldPatches[k] = v
	}

	for level, sublevels := range patches {
		if level > lv {
			delete(patches, level)
			continue
		}
		if level == lv && len(sublevels)-1 > sublvl {
			sublevels = sublevels[:sublvl+1]
			patches[level] = sublevels
		}
	}

	return func() {
		patches = oldPatches
		Level = old
		Sublevel = oldSublvl
	}
}
