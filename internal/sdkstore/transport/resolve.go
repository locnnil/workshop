// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3.

package transport

import (
	"github.com/canonical/workshop/internal/timeutil"
)

// ResolveRequest defines a typed request for resolving revisions.
type ResolveRequest struct {
	Packages []ResolvePackage `json:"packages"`
	Crafts   []ResolveCraft   `json:"crafts,omitempty"`
}

// ResolvePackage defines a typed request for resolving SDK revisions.
type ResolvePackage struct {
	// InstanceKey should be unique for every SDK, as results may not be
	// ordered in the same way, so it is expected to use this to ensure
	// completeness and ordering.
	InstanceKey string `json:"instance-key"`
	// Must be "sdk".
	Namespace string `json:"namespace"`
	// Either Name or ID must be supplied.
	Name     string   `json:"name,omitempty"`
	ID       string   `json:"id,omitempty"`
	Channel  string   `json:"channel"`
	Platform Platform `json:"platform"`
}

// ResolveCraft defines a typed request for resolving *craft revisions.
type ResolveCraft struct {
	InstanceKey string `json:"instance-key"`
	Namespace   string `json:"namespace"`
	// Either Name or ID must be supplied.
	Name    string `json:"name,omitempty"`
	ID      string `json:"id,omitempty"`
	Channel string `json:"channel"`
}

// ResolveResponse holds a list of resolved SDKs and *craft recipes.
type ResolveResponse struct {
	PackageResults []ResolvePackageResponse `json:"package-results"`
	CraftResults   []ResolveCraftResponse   `json:"craft-results"`
}

// ResolvePackageResponse describes the response for a single SDK.
type ResolvePackageResponse struct {
	InstanceKey string `json:"instance-key"`
	// Status can be "ok" or "error".
	Status    string               `json:"status"`
	Error     *APIError            `json:"error,omitempty"`
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	ID        string               `json:"id"`
	Result    ResolvePackageResult `json:"result"`
}

// ResolvePackageResult describes an SDK channel and revision.
type ResolvePackageResult struct {
	Channel  ResolvePackageChannel `json:"channel,omitzero"`
	Revision ResolveRevision       `json:"revision"`
}

// ResolvePackageChannel defines a unique permutation that corresponds to the
// track, risk, branch, and platform. There can be multiple channels of the
// same track and risk, but with different platforms.
type ResolvePackageChannel struct {
	Name             string           `json:"name"`
	ReleasedAt       timeutil.TimeUTC `json:"released-at"`
	Track            string           `json:"track"`
	Risk             string           `json:"risk"`
	Branch           string           `json:"branch"`
	EffectiveChannel string           `json:"effective-channel"`
	Platform         Platform         `json:"platform"`
}

// ResolveRevision contains SDK details that apply to a single revision.
type ResolveRevision struct {
	CreatedAt timeutil.TimeUTC `json:"created-at"`
	Platforms []Platform       `json:"platforms"`
	Download  Download         `json:"download"`
	Revision  int              `json:"revision"`
	Version   string           `json:"version"`
}

// ResolveCraftResponse describes the response for a single *craft recipe.
type ResolveCraftResponse struct {
	InstanceKey string `json:"instance-key"`
	// Status can be "ok" or "error".
	Status    string             `json:"status"`
	Error     *APIError          `json:"error,omitempty"`
	Namespace string             `json:"namespace"`
	Name      string             `json:"name"`
	ID        string             `json:"id"`
	Result    ResolveCraftResult `json:"result"`
}

// ResolveCraftResult describes a *craft recipe channel and commit.
type ResolveCraftResult struct {
	Channel ResolveCraftChannel `json:"channel,omitzero"`
	Commit  ResolveCommit       `json:"commit"`
}

// ResolveCraftChannel defines a unique permutation that corresponds to the
// track, risk, branch, and platform. There can be multiple channels of the
// same track and risk, but with different platforms.
type ResolveCraftChannel struct {
	Name             string           `json:"name"`
	ReleasedAt       timeutil.TimeUTC `json:"released-at"`
	Track            string           `json:"track"`
	Risk             string           `json:"risk"`
	Branch           string           `json:"branch"`
	EffectiveChannel string           `json:"effective-channel"`
}

// ResolveCommit describes the provenance of a *craft recipe.
type ResolveCommit struct {
	CreatedAt  timeutil.TimeUTC `json:"created-at"`
	Remote     ResolveRemote    `json:"remote"`
	GitBranch  string           `json:"git-branch"`
	CommitHash string           `json:"commit-hash"`
	Version    string           `json:"version"`
}

// ResolveRemote describes a VCS remote.
type ResolveRemote struct {
	URL string `json:"url"`
}
