package main

import (
	"fmt"
	"os"
	"path/filepath"

	util "github.com/canonical/workspace/internal"
	"github.com/canonical/workspace/internal/logger"
	"github.com/canonical/workspace/internal/overlord/projectstate"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:              "workspace",
	SilenceErrors:    false,
	SilenceUsage:     true,
	TraverseChildren: true,
}

var Project string

func getProjectDirectory(fs afero.Fs, cwd string) (string, error) {

	/* let's now see if we are in a directory nested under a workspace project
	and if so, return this project directory instead of a CWD */
	path := cwd

	for {
		var err error
		var ok bool
		if ok, err = afero.Exists(fs, path); err == nil && ok {
			if ok, err = afero.Exists(fs, projectstate.LockPath(path)); err == nil && ok {
				return path, nil
			}
		}
		if err != nil {
			return "", err
		}

		if path == string(os.PathSeparator) {
			break
		}
		path = filepath.Join(path, "..", string(os.PathSeparator))
	}

	return cwd, nil
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	cwd, err = util.CleanProjectPath(cwd)
	if err != nil {
		panic(err)
	}

	fs := afero.NewOsFs()
	cwd, err = getProjectDirectory(fs, cwd)
	if err != nil {
		fmt.Println(err)
		panic("cannot get project directory")
	}

	logger.SetLogger(logger.New(os.Stderr, "[workspace] "))

	rootCmd.PersistentFlags().StringVarP(&Project, "project", "p", cwd, "specify a project's directory path")

	rootCmd.AddCommand((&CmdLaunch{}).Command())
	rootCmd.AddCommand((&CmdList{}).Command())
	rootCmd.Execute()
}
