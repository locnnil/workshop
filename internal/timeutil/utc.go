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

package timeutil

import (
	"time"

	"gopkg.in/yaml.v3"
)

// TimeUTC is like time.Time but converts to UTC when (un)marshalled.
type TimeUTC time.Time

func (t TimeUTC) MarshalText() ([]byte, error) {
	return time.Time(t).UTC().MarshalText()
}

func (t *TimeUTC) UnmarshalText(data []byte) error {
	var temp time.Time
	if err := temp.UnmarshalText(data); err != nil {
		return err
	}
	*t = TimeUTC(temp.UTC())
	return nil
}

func (t TimeUTC) MarshalYAML() (any, error) {
	return time.Time(t).UTC(), nil
}

func (t *TimeUTC) UnmarshalYAML(value *yaml.Node) error {
	var temp time.Time
	if err := value.Decode(&temp); err != nil {
		return err
	}
	*t = TimeUTC(temp.UTC())
	return nil
}
