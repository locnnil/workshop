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

package interfaces

import (
	"context"

	"github.com/canonical/workspace/internal/sdk"
	"github.com/canonical/workspace/internal/timings"
	"github.com/canonical/workspace/internal/workspacebackend"
)

// SecurityBackend abstracts interactions between the interface system and the
// needs of a particular security system.
type SecurityBackend interface {
	// Initialize performs any initialization required by the backend.
	// It is called during workspaced startup process.
	Initialize(backend workspacebackend.WorkspaceBackend) error

	// Name returns the name of the backend.
	// This is intended for diagnostic messages.
	Name() SecuritySystem

	// Setup creates and loads security artefacts specific to a given sdk.
	// This method should be called after changing plug, slots, connections
	// between them.
	Setup(context context.Context, sdk *sdk.Info, repo *Repository) error

	// Remove removes and unloads security artefacts of a given sdk.
	//
	// This method should be called during the process of removing an sdk.
	Remove(context context.Context, workspace, sdkName string) error

	// NewSpecification returns a new specification associated with this backend.
	NewSpecification(user, pid string) Specification
}

// SecurityBackendSetupMany interface may be implemented by backends that can optimize their operations
// when setting up multiple sdks at once.
type SecurityBackendSetupMany interface {
	// SetupMany creates and loads apparmor profiles of multiple sdks. It tries to process all sdks and doesn't interrupt processing
	// on errors of individual sdks.
	SetupMany(sdks []*sdk.Info, repo *Repository, tm timings.Measurer) []error
}
