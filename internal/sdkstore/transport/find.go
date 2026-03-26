// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package transport

type FindResponses struct {
	Results []FindResponse `json:"results,omitempty"`
}

type FindResponse struct {
	Name           string         `json:"name"`
	PackageID      string         `json:"package-id"`
	Metadata       FindMetadata   `json:"metadata,omitzero"`
	DefaultRelease FindChannelMap `json:"default-release,omitzero"`
}

// FindMetadata contains SDK details that apply to all revisions.
type FindMetadata struct {
	Contact     string    `json:"contact,omitempty"`
	Description string    `json:"description,omitempty"`
	License     string    `json:"license,omitempty"`
	Links       Links     `json:"links,omitzero"`
	Media       []Media   `json:"media,omitempty"`
	Publisher   Publisher `json:"publisher,omitzero"`
	Summary     string    `json:"summary,omitempty"`
}

type FindChannelMap struct {
	Channel  Channel `json:"channel"`
	Revision int     `json:"revision"`
	Version  string  `json:"version"`
}
