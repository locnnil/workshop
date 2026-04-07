// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2018-2019 Canonical Ltd
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

package sdk

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	MAX_TRACK_LENGTH  = 28
	MAX_BRANCH_LENGTH = 128
)

var (
	channelTrack  = regexp.MustCompile(`^[a-zA-Z0-9](?:[_.-]?[a-zA-Z0-9])*$`)
	channelRisks  = []string{"stable", "candidate", "beta", "edge"}
	channelBranch = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9]$`)
)

// Channel identifies and describes completely a store channel.
type Channel struct {
	Name   string `json:"name"`
	Track  string `json:"track"`
	Risk   string `json:"risk"`
	Branch string `json:"branch,omitempty"`
}

func (c Channel) MarshalText() ([]byte, error) {
	return []byte(c.String()), nil
}

func (c *Channel) UnmarshalText(data []byte) error {
	channel, err := ParseChannel(string(data))
	if err == nil {
		*c = channel
	}
	return err
}

func (c Channel) String() string {
	return c.Name
}

func ParseChannel(channel string) (Channel, error) {
	parts := strings.Split(channel, "/")
	var risk, track, branch *string
	switch len(parts) {
	case 1:
		if slices.Contains(channelRisks, parts[0]) {
			risk = &parts[0]
		} else {
			track = &parts[0]
		}
	case 2:
		if slices.Contains(channelRisks, parts[0]) {
			risk, branch = &parts[0], &parts[1]
		} else {
			track, risk = &parts[0], &parts[1]
		}
	case 3:
		track, risk, branch = &parts[0], &parts[1], &parts[2]
	default:
		return Channel{}, fmt.Errorf("channel %q has too many components", channel)
	}

	ch := Channel{Name: channel}
	if track != nil {
		if len(*track) > MAX_TRACK_LENGTH || !channelTrack.MatchString(*track) || slices.Contains(channelRisks, *track) {
			return Channel{}, fmt.Errorf("invalid track %q in channel %q", *track, channel)
		}
		ch.Track = *track
	}
	if risk != nil {
		if !slices.Contains(channelRisks, *risk) {
			return Channel{}, fmt.Errorf("invalid risk %q in channel %q", *risk, channel)
		}
		ch.Risk = *risk
	}
	if branch != nil {
		if len(*branch) > MAX_BRANCH_LENGTH || !channelBranch.MatchString(*branch) || slices.Contains(channelRisks, *branch) {
			return Channel{}, fmt.Errorf("invalid branch %q in channel %q", *branch, channel)
		}
		ch.Branch = *branch
	}
	return ch, nil
}

// Full replaces empty fields with their defaults.
func (c *Channel) Full() Channel {
	track := c.Track
	if track == "" {
		track = "latest"
	}
	risk := c.Risk
	if risk == "" {
		risk = "stable"
	}

	var name strings.Builder
	fmt.Fprint(&name, track, "/", risk)
	if c.Branch != "" {
		fmt.Fprint(&name, "/", c.Branch)
	}

	return Channel{
		Name:   name.String(),
		Track:  track,
		Risk:   risk,
		Branch: c.Branch,
	}
}
