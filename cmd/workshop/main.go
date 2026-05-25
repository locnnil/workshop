// Copyright (c) 2026 Canonical Ltd
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

package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/logger"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	l, err := logger.New(Stderr, 0)
	if err != nil {
		panic(err)
	}

	logger.SetLogger(l)

	rootCmd := (&CmdRoot{cwd: cwd}).Command()
	// Work around https://github.com/spf13/cobra/issues/2257.
	rootCmd.SetArgs(slices.Clone(os.Args[1:]))

	if err = rootCmd.Execute(); err != nil {
		exitError, ok := err.(*client.ExitError)
		if ok {
			os.Exit(exitError.ExitCode())
		}
		fmt.Fprintf(Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
