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

	"github.com/spf13/cobra"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/version"
)

// exitStatus can be used in panic(&exitStatus{code}) to cause Workshops's main
// function to exit with a given exit code, for the rare cases when you want
// to return an exit code other than 0 or 1, or when an error return is not
// possible.
type exitStatus struct {
	code int
}

func (e *exitStatus) Error() string {
	return fmt.Sprintf("internal error: exitStatus{%d} being handled as normal error", e.code)
}

var workshopd = &cobra.Command{
	Use:              "workshopd",
	Version:          version.Version,
	SilenceErrors:    false,
	SilenceUsage:     true,
	TraverseChildren: true,
}

var (
	osExit = os.Exit
)

func main() {
	l, err := logger.New(os.Stderr, 0)
	if err != nil {
		panic(err)
	}
	logger.SetLogger(l)
	defer func() {
		if v := recover(); v != nil {
			if e, ok := v.(*exitStatus); ok {
				osExit(e.code)
			}
			panic(v)
		}
	}()

	workshopd.SetVersionTemplate("{{.Version}}\n")
	workshopd.AddCommand((&cmdRun{}).Command())
	workshopd.AddCommand((&cmdConfigureDNS{}).Command())
	if err = workshopd.Execute(); err != nil {
		os.Exit(1)
	}
}
