// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package transport

import (
	"encoding/json"

	"github.com/canonical/workshop/internal/timeutil"
)

// InfoResponse is the result from an information query.
type InfoResponse struct {
	Name         string           `json:"name"`
	PackageID    string           `json:"package-id"`
	Metadata     InfoMetadata     `json:"metadata,omitzero"`
	ChannelMap   []InfoChannelMap `json:"channel-map,omitempty"`
	DefaultTrack string           `json:"default-track,omitempty"`
}

// InfoMetadata contains SDK details that apply to all revisions.
type InfoMetadata struct {
	Categories  []Category `json:"categories,omitempty"`
	Contact     string     `json:"contact,omitempty"`
	Description string     `json:"description,omitempty"`
	License     string     `json:"license,omitempty"`
	Links       InfoLinks  `json:"links,omitzero"`
	Media       []Media    `json:"media,omitempty"`
	Private     bool       `json:"private,omitempty"`
	Publisher   Publisher  `json:"publisher,omitzero"`
	Summary     string     `json:"summary,omitempty"`
	Title       string     `json:"title,omitempty"`
	Website     string     `json:"website,omitempty"`
}

// InfoLinks contains URLs associated with the SDK.
type InfoLinks struct {
	Contact   []string `json:"contact,omitempty"`
	Docs      []string `json:"docs,omitempty"`
	Donations []string `json:"donations,omitempty"`
	Issues    []string `json:"issues,omitempty"`
	Source    []string `json:"source,omitempty"`
	Website   []string `json:"website,omitempty"`
	Upstream  string   `json:"upstream,omitempty"`
}

// InfoChannelMap represents the information channel map. This defines a unique
// revision for a given channel from an info response.
type InfoChannelMap struct {
	Channel  Channel      `json:"channel,omitzero"`
	Revision InfoRevision `json:"revision,omitzero"`
}

// InfoRevision contains SDK details that apply to a single revision.
type InfoRevision struct {
	Platforms []Platform        `json:"platforms,omitzero"`
	CreatedAt *timeutil.TimeUTC `json:"created-at,omitempty"`
	Download  Download          `json:"download,omitzero"`
	Revision  int               `json:"revision,omitzero"`
	Version   string            `json:"version,omitempty"`
	SdkYAML   json.RawMessage   `json:"sdk-yaml,omitempty"`
}
