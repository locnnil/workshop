// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2014-2021 Canonical Ltd
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

package bootloader

import (
	"errors"
	"fmt"
)

var (
	// ErrBootloader is returned if the bootloader can not be determined.
	ErrBootloader = errors.New("cannot determine bootloader")

	// ErrNoTryKernelRef is returned if the bootloader finds no enabled
	// try-kernel.
	ErrNoTryKernelRef = errors.New("no try-kernel referenced")
)

// Role indicates whether the bootloader is used for recovery or run mode.
type Role string

const (
	// RoleSole applies to the sole bootloader used by UC16/18.
	RoleSole Role = ""
	// RoleRunMode applies to the run mode booloader.
	RoleRunMode Role = "run-mode"
	// RoleRecovery apllies to the recovery bootloader.
	RoleRecovery Role = "recovery"
)

// Options carries bootloader options.
type Options struct {
	// PrepareImageTime indicates whether the booloader is being
	// used at prepare-image time, that means not on a runtime
	// system.
	PrepareImageTime bool

	// Role specifies to use the bootloader for the given role.
	Role Role

	// NoSlashBoot indicates to use the native layout of the
	// bootloader partition and not the /boot mount.
	// It applies only for RoleRunMode.
	// It is implied and ignored for RoleRecovery.
	// It is an error to set it for RoleSole.
	NoSlashBoot bool
}

func (o *Options) validate() error {
	if o == nil {
		return nil
	}
	if o.NoSlashBoot && o.Role == RoleSole {
		return fmt.Errorf("internal error: bootloader.RoleSole doesn't expect NoSlashBoot set")
	}
	if o.PrepareImageTime && o.Role == RoleRunMode {
		return fmt.Errorf("internal error: cannot use run mode bootloader at prepare-image time")
	}
	return nil
}

// Bootloader provides an interface to interact with the system
// bootloader.
type Bootloader interface {
	// Return the value of the specified bootloader variable.
	GetBootVars(names ...string) (map[string]string, error)

	// Set the value of the specified bootloader variable.
	SetBootVars(values map[string]string) error

	// Name returns the bootloader name.
	Name() string

	// Present returns whether the bootloader is currently present on the
	// system - in other words whether this bootloader has been installed to the
	// current system. Implementations should only return non-nil error if they
	// can positively identify that the bootloader is installed, but there is
	// actually an error with the installation.
	Present() (bool, error)

	// InstallBootConfig will try to install the boot config in the
	// given gadgetDir to rootdir.
	InstallBootConfig(gadgetDir string, opts *Options) error
}

// ComamndLineComponents carries the components of the kernel command line. The
// bootloader is expected to combine the provided components, optionally
// including its built-in static set of arguments, and produce a command line
// that will be passed to the kernel during boot.
type CommandLineComponents struct {
	// Argument related to mode selection.
	ModeArg string
	// Argument related to recovery system selection, relevant for given
	// mode argument.
	SystemArg string
	// Extra arguments requested by the system.
	ExtraArgs string
	// A complete set of arguments that overrides both the built-in static
	// set and ExtraArgs. Note that, it is an error if extra and full
	// arguments are non-empty.
	FullArgs string
}

func (c *CommandLineComponents) Validate() error {
	if c.ExtraArgs != "" && c.FullArgs != "" {
		return fmt.Errorf("cannot use both full and extra components of command line")
	}
	return nil
}

// TrustedAssetsBootloader has boot assets that take part in the secure boot
// process and need to be tracked, while other boot assets (typically boot
// config) are managed by snapd.
type TrustedAssetsBootloader interface {
	Bootloader

	// ManagedAssets returns a list of boot assets managed by the bootloader
	// in the boot filesystem. Does not require rootdir to be set.
	ManagedAssets() []string
	// UpdateBootConfig attempts to update the boot config assets used by
	// the bootloader. Returns true when assets were updated.
	UpdateBootConfig() (bool, error)
	// CommandLine returns the kernel command line composed of mode and
	// system arguments, followed by either a built-in bootloader specific
	// static arguments corresponding to the on-disk boot asset edition, and
	// any extra arguments or a separate set of arguments provided in the
	// components. The command line may be different when using a recovery
	// bootloader.
	CommandLine(pieces CommandLineComponents) (string, error)
	// CandidateCommandLine is similar to CommandLine, but uses the current
	// edition of managed built-in boot assets as reference.
	CandidateCommandLine(pieces CommandLineComponents) (string, error)

	// TrustedAssets returns the list of relative paths to assets inside the
	// bootloader's rootdir that are measured in the boot process in the
	// order of loading during the boot. Does not require rootdir to be set.
	TrustedAssets() ([]string, error)

	// RecoveryBootChain returns the load chain for recovery modes.
	// It should be called on a RoleRecovery bootloader.
	RecoveryBootChain(kernelPath string) ([]BootFile, error)

	// BootChain returns the load chain for run mode.
	// It should be called on a RoleRecovery bootloader passing the
	// RoleRunMode bootloader.
	BootChain(runBl Bootloader, kernelPath string) ([]BootFile, error)
}

// NotScriptableBootloader cannot change the bootloader environment
// because it supports no scripting or cannot do any writes. This
// applies to piboot for the moment.
type NotScriptableBootloader interface {
	Bootloader

	// Sets boot variables from initramfs - this is needed in
	// addition to SetBootVars() to prevent side effects like
	// re-writing the bootloader configuration.
	SetBootVarsFromInitramfs(values map[string]string) error
}

// RebootBootloader needs arguments to the reboot syscall when snaps
// are being updated.
type RebootBootloader interface {
	Bootloader

	// GetRebootArguments returns the needed reboot arguments
	GetRebootArguments() (string, error)
}

// BootFile represents each file in the chains of trusted assets and
// kernels used in the boot process. For example a boot file can be an
// EFI binary or a snap file containing an EFI binary.
type BootFile struct {
	// Path is the path to the file in the filesystem or, if Snap
	// is set, the relative path inside the snap file.
	Path string
	// Snap contains the path to the snap file if a snap file is used.
	Snap string
	// Role is set to the role of the bootloader this boot file
	// originates from.
	Role Role
}

func NewBootFile(snap, path string, role Role) BootFile {
	return BootFile{
		Snap: snap,
		Path: path,
		Role: role,
	}
}

// WithPath returns a copy of the BootFile with path updated to the
// specified value.
func (b BootFile) WithPath(path string) BootFile {
	b.Path = path
	return b
}
