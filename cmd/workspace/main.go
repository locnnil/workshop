package main

import (
	"os"
	"path/filepath"

	"github.com/canonical/workspace/client"
	util "github.com/canonical/workspace/internal"
	"github.com/canonical/workspace/internal/logger"
	"github.com/canonical/workspace/internal/project"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type clientSetter interface {
	setClient(*client.Client)
}

type clientMixin struct {
	client *client.Client
}

func (ch *clientMixin) setClient(cli *client.Client) {
	ch.client = cli
}

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
			if ok, err = afero.Exists(fs, project.LockPath(path)); err == nil && ok {
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
		panic(err)
	}

	cwd, err = util.CleanProjectPath(cwd)
	if err != nil {
		panic(err)
	}

	fs := afero.NewOsFs()
	cwd, err = getProjectDirectory(fs, cwd)
	if err != nil {
		panic(err)
	}

	logger.SetLogger(logger.New(os.Stderr, "[workspace] "))

	rootCmd.PersistentFlags().StringVarP(&Project, "project", "p", cwd, "specify a project's directory path")

	rootCmd.AddCommand((&CmdLaunch{}).Command())
	rootCmd.AddCommand((&CmdList{}).Command())
	rootCmd.AddCommand((&CmdChanges{}).Command())
	rootCmd.AddCommand((&CmdTasks{}).Command())

	rootCmd.Execute()
}
