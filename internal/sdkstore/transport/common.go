// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package transport

import (
	"github.com/canonical/workshop/internal/timeutil"
)

// The following contains all the common DTOs for a gathering information from
// a given store.

// Channel defines a unique permutation that corresponds to the track, risk
// and platform. There can be multiple channels of the same track and risk, but
// with different platforms.
type Channel struct {
	Track      string           `json:"track,omitempty"`
	Risk       string           `json:"risk,omitempty"`
	Name       string           `json:"name,omitempty"`
	Platform   Platform         `json:"platform,omitzero"`
	ReleasedAt timeutil.TimeUTC `json:"released-at"`
}

// Platform is a typed tuple for identifying SDKs with a matching architecture,
// os and channel.
type Platform struct {
	Name         string `json:"name"`
	Channel      string `json:"channel"`
	Architecture string `json:"architecture"`
}

// Download represents the download structure from the SDK Store.
type Download struct {
	URL      string `json:"url"`
	Size     uint64 `json:"size"`
	Sha3_384 string `json:"sha3-384"`
}

// Category defines the category of a given SDK. Akin to a tag.
type Category struct {
	Featured bool   `json:"featured"`
	Name     string `json:"name"`
}

// Media defines media attached to an SDK.
type Media struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Publisher identifies who published a given SDK.
type Publisher struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display-name"`
	Validation  string `json:"validation"`
}
