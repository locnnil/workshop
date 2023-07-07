// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2019-2022 Canonical Ltd
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

package boot

import (
	"github.com/canonical/workspace/internal/bootloader"
)

const (
	// DefaultStatus is the value of a status boot variable when nothing is
	// being tried
	DefaultStatus = ""
	// TryStatus is the value of a status boot variable when something is about
	// to be tried
	TryStatus = "try"
	// TryingStatus is the value of a status boot variable after we have
	// attempted a boot with a try snap - this status is only set in the early
	// boot sequence (bootloader, initramfs, etc.)
	TryingStatus = "trying"
)

// RebootInfo contains information about how to perform a reboot if
// required
type RebootInfo struct {
	// RebootRequired is true if we need to reboot after an update
	RebootRequired bool
	// RebootBootloader will not be nil if the bootloader has something to say on
	// how to perform the reboot
	RebootBootloader bootloader.RebootBootloader
}

// NextBootContext carries additional significative information used when
// setting the next boot.
type NextBootContext struct {
	// BootWithoutTry is sets if we don't want to use the "try" logic. This
	// is useful if the next boot is part of an installation undo.
	BootWithoutTry bool
}

// A BootParticipant handles the boot process details for a snap involved in it.
type BootParticipant interface {
	// SetNextBoot will schedule the snap to be used in the next
	// boot. bootCtx contains context information that influences how the
	// next boot is performed. For base snaps it is up to the caller to
	// select the right bootable base (from the model assertion). It is a
	// noop for not relevant snaps.  Otherwise it returns whether a reboot
	// is required.
	SetNextBoot(bootCtx NextBootContext) (rebootInfo RebootInfo, err error)

	// Is this a trivial implementation of the interface?
	IsTrivial() bool
}
