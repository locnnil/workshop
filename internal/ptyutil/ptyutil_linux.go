// Copyright (c) 2021 Canonical Ltd
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License version 3 as
// published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package ptyutil

import (
	"unsafe"

	"github.com/pkg/term/termios"
	"golang.org/x/sys/unix"
)

// SetSize sets the dimensions of the terminal associated with fd.
func SetSize(fd int, width int, height int) (err error) {
	var dimensions [4]uint16
	dimensions[0] = uint16(height)
	dimensions[1] = uint16(width)

	if _, _, err := unix.Syscall6(unix.SYS_IOCTL, uintptr(fd), uintptr(unix.TIOCSWINSZ), uintptr(unsafe.Pointer(&dimensions)), 0, 0, 0); err != 0 {
		return err
	}
	return nil
}

// GetSize returns the dimensions of the given terminal.
func GetSize(fd int) (int, int, error) {
	winsize, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return -1, -1, err
	}

	return int(winsize.Col), int(winsize.Row), nil
}

// State contains the state of a terminal.
type State struct {
	Termios unix.Termios
}

// IsTerminal returns true if the given file descriptor is a terminal.
func IsTerminal(fd int) bool {
	_, err := GetState(fd)
	return err == nil
}

// GetState returns the current state of a terminal which may be useful to restore the terminal after a signal.
func GetState(fd int) (*State, error) {
	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}

	state := State{}
	state.Termios = *termios

	return &state, nil
}

// MakeRaw put the terminal connected to the given file descriptor into raw mode and returns the previous state of the terminal so that it can be restored.
func MakeRaw(fd int) (*State, error) {
	var err error
	var oldState, newState *State

	oldState, err = GetState(fd)
	if err != nil {
		return nil, err
	}

	newState = &State{}
	newState.Termios = oldState.Termios

	termios.Cfmakeraw(&newState.Termios)

	err = Restore(fd, newState)
	if err != nil {
		return nil, err
	}

	return oldState, nil
}

// Restore restores the terminal connected to the given file descriptor to a previous state.
func Restore(fd int, state *State) error {
	return termios.Tcsetattr(uintptr(fd), termios.TCSANOW, &state.Termios)
}
