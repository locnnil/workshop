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

package ifacetest

import (
	"context"

	"github.com/canonical/workspace/internal/interfaces"
	"github.com/canonical/workspace/internal/sdk"
	"github.com/canonical/workspace/internal/workspacebackend"
)

// TestSecurityBackend is a security backend intended for testing.
type TestSecurityBackend struct {
	BackendName interfaces.SecuritySystem
	// SetupCalls stores information about all calls to Setup
	SetupCalls []TestSetupCall
	// RemoveCalls stores information about all calls to Remove
	RemoveCalls []string
	// SetupCallback is an callback that is optionally called in Setup
	SetupCallback func(context context.Context, sdkInfo *sdk.Info, repo *interfaces.Repository) error
	// RemoveCallback is a callback that is optionally called in Remove
	RemoveCallback func(sdkName string) error
	// SandboxFeaturesCallback is a callback that is optionally called in SandboxFeatures
	SandboxFeaturesCallback func() []string
}

// TestSetupCall stores details about calls to TestSecurityBackend.Setup
type TestSetupCall struct {
	// SdkInfo is a copy of the sdkInfo argument to a particular call to Setup
	SdkInfo *sdk.Info
}

// Initialize does nothing.
func (b *TestSecurityBackend) Initialize(backend workspacebackend.WorkspaceBackend) error {
	return nil
}

// Name returns the name of the security backend.
func (b *TestSecurityBackend) Name() interfaces.SecuritySystem {
	return b.BackendName
}

// Setup records information about the call and calls the setup callback if one is defined.
func (b *TestSecurityBackend) Setup(context context.Context, sdkInfo *sdk.Info, repo *interfaces.Repository) error {
	b.SetupCalls = append(b.SetupCalls, TestSetupCall{SdkInfo: sdkInfo})
	if b.SetupCallback == nil {
		return nil
	}
	return b.SetupCallback(context, sdkInfo, repo)
}

// Remove records information about the call and calls the remove callback if one is defined
func (b *TestSecurityBackend) Remove(sdkName string) error {
	b.RemoveCalls = append(b.RemoveCalls, sdkName)
	if b.RemoveCallback == nil {
		return nil
	}
	return b.RemoveCallback(sdkName)
}

func (b *TestSecurityBackend) NewSpecification(user, pid string) interfaces.Specification {
	return &Specification{user: user, pid: pid}
}

func (b *TestSecurityBackend) SandboxFeatures() []string {
	if b.SandboxFeaturesCallback == nil {
		return nil
	}
	return b.SandboxFeaturesCallback()
}

// TestSecurityBackendSetupMany is a security backend that implements SetupMany on top of TestSecurityBackend.
type TestSecurityBackendSetupMany struct {
	TestSecurityBackend

	// SetupManyCalls stores information about all calls to Setup
	SetupManyCalls []TestSetupManyCall

	// SetupManyCallback is an callback that is optionally called in Setup
	SetupManyCallback func(context context.Context, sdkInfo []*sdk.Info, repo *interfaces.Repository) []error
}

// TestSetupManyCall stores details about calls to TestSecurityBackendMany.SetupMany
type TestSetupManyCall struct {
	// SdkInfos is a copy of the sdkInfo arguments to a particular call to SetupMany
	SdkInfos []*sdk.Info
}

func (b *TestSecurityBackendSetupMany) SetupMany(context context.Context, sdkInfo []*sdk.Info, repo *interfaces.Repository) []error {
	b.SetupManyCalls = append(b.SetupManyCalls, TestSetupManyCall{SdkInfos: sdkInfo})
	if b.SetupManyCallback == nil {
		return nil
	}
	return b.SetupManyCallback(context, sdkInfo, repo)
}
