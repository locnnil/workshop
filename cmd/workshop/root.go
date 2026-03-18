package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/version"
)

type CmdRoot struct {
	cwd string
	cli *client.Client
	prj string
}

func (c *CmdRoot) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workshop",
		Version: version.Version,
		// Avoid printing errors twice
		SilenceErrors:    true,
		SilenceUsage:     true,
		TraverseChildren: true,

		RunE:                       c.run,
		PersistentPostRun:          c.postRun,
		SuggestionsMinimumDistance: 2,
	}
	cmd.SetVersionTemplate("{{.Version}}\n")

	cmd.AddGroup(
		&cobra.Group{
			ID:    "create-update-delete",
			Title: "Create new workshops; start, stop, update or delete existing ones:",
		},
		&cobra.Group{
			ID:    "sketch",
			Title: "Customize a workshop:",
		},
		&cobra.Group{
			ID:    "explore-troubleshoot",
			Title: "Enumerate workshops, list their details:",
		},
		&cobra.Group{
			ID:    "changes-tasks",
			Title: "List recent changes and individual activities:",
		},
		&cobra.Group{
			ID:    "connect",
			Title: "Create, manage, list and drop interface connections:",
		},
		&cobra.Group{
			ID:    "utilise",
			Title: "Run commands inside a workshop:",
		},
		&cobra.Group{
			ID:    "warnings",
			Title: "List and acknowledge warnings:",
		},
		&cobra.Group{
			ID:    "misc",
			Title: "Additional commands:",
		},
	)

	cmd.SetHelpCommandGroupID("misc")
	cmd.SetCompletionCommandGroupID("misc")

	launchCmd := (&CmdLaunch{root: c}).Command()
	launchCmd.GroupID = "create-update-delete"
	cmd.AddCommand(launchCmd)

	listCmd := (&CmdList{root: c}).Command()
	listCmd.GroupID = "explore-troubleshoot"
	cmd.AddCommand(listCmd)

	changesCmd := (&CmdChanges{root: c}).Command()
	changesCmd.GroupID = "changes-tasks"
	cmd.AddCommand(changesCmd)

	tasksCmd := (&CmdTasks{root: c}).Command()
	tasksCmd.GroupID = "changes-tasks"
	cmd.AddCommand(tasksCmd)

	refreshCmd := (&CmdRefresh{root: c}).Command()
	refreshCmd.GroupID = "create-update-delete"
	cmd.AddCommand(refreshCmd)

	startCmd := (&CmdStart{root: c}).Command()
	startCmd.GroupID = "create-update-delete"
	cmd.AddCommand(startCmd)

	stopCmd := (&CmdStop{root: c}).Command()
	stopCmd.GroupID = "create-update-delete"
	cmd.AddCommand(stopCmd)

	infoCmd := (&CmdInfo{root: c}).Command()
	infoCmd.GroupID = "explore-troubleshoot"
	cmd.AddCommand(infoCmd)

	execCmd := (&CmdExec{root: c}).Command()
	execCmd.GroupID = "utilise"
	cmd.AddCommand(execCmd)

	shellCmd := (&CmdShell{root: c}).Command()
	shellCmd.GroupID = "utilise"
	cmd.AddCommand(shellCmd)

	runCmd := (&CmdRun{root: c}).Command()
	runCmd.GroupID = "utilise"
	cmd.AddCommand(runCmd)

	actionsCmd := (&CmdActions{root: c}).Command()
	actionsCmd.GroupID = "explore-troubleshoot"
	cmd.AddCommand(actionsCmd)

	removeCmd := (&CmdRemove{root: c}).Command()
	removeCmd.GroupID = "create-update-delete"
	cmd.AddCommand(removeCmd)

	remountCmd := (&CmdRemount{root: c}).Command()
	remountCmd.GroupID = "connect"
	cmd.AddCommand(remountCmd)

	connectionsCmd := (&CmdConnections{root: c}).Command()
	connectionsCmd.GroupID = "connect"
	cmd.AddCommand(connectionsCmd)

	connectCmd := (&CmdConnect{root: c}).Command()
	connectCmd.GroupID = "connect"
	cmd.AddCommand(connectCmd)

	disconnectCmd := (&CmdDisconnect{root: c}).Command()
	disconnectCmd.GroupID = "connect"
	cmd.AddCommand(disconnectCmd)

	warningsCmd := (&CmdWarnings{root: c}).Command()
	warningsCmd.GroupID = "warnings"
	cmd.AddCommand(warningsCmd)

	okayCmd := (&CmdOkay{root: c}).Command()
	okayCmd.GroupID = "warnings"
	cmd.AddCommand(okayCmd)

	sketchCmd := (&CmdSketch{root: c}).Command()
	sketchCmd.GroupID = "sketch"
	cmd.AddCommand(sketchCmd)

	sketchesCmd := (&CmdSketches{root: c}).Command()
	sketchesCmd.GroupID = "sketch"
	cmd.AddCommand(sketchesCmd)

	cmd.AddCommand((&CmdDocs{root: c}).Command())

	cmd.PersistentFlags().StringVarP(&c.prj, "project", "p", c.cwd, "Specify the project's directory path.")
	cmd.PersistentFlags().BoolP("help", "h", false, "Print the help message for the command.")
	cmd.PersistentFlags().BoolP("version", "v", false, "Print Workshop version.")

	_ = cmd.RegisterFlagCompletionFunc("project", c.completeProjectPaths())

	cmd.DisableAutoGenTag = true

	return cmd
}

func (c *CmdRoot) run(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	msg := fmt.Sprintf("unknown command %q", args[0])
	if suggestions := cmd.SuggestionsFor(args[0]); len(suggestions) > 0 {
		msg += "\n\nDid you mean this?\n\t" + strings.Join(suggestions, "\n\t")
	}
	return errors.New(msg)
}

func (c *CmdRoot) client() (*client.Client, error) {
	if c.cli != nil {
		return c.cli, nil
	}

	cli, err := client.New(&ClientConfig)
	if err == nil {
		c.cli = cli
	} else {
		err = fmt.Errorf("cannot create client: %v", err)
	}

	return cli, err
}

func (c *CmdRoot) project() string {
	if c.cwd == "" {
		return c.prj
	}
	// Avoid filepath.Abs because it returns an error.
	return abs(c.cwd, c.prj)
}

func abs(cwd, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(cwd, path)
}

func (c *CmdRoot) postRun(cmd *cobra.Command, args []string) {
	if c.cli != nil && cmd.Name() != cobra.ShellCompRequestCmd {
		maybePresentWarnings(c.cli.WarningsSummary())
		c.cli.CloseIdleConnections()
	}
}

func (c *CmdRoot) completeProjectPaths() cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return c.doCompleteProjectPaths(cmd, toComplete)
	}
}

func (c *CmdRoot) doCompleteProjectPaths(cmd *cobra.Command, toComplete string) ([]string, cobra.ShellCompDirective) {
	requireProject := []string{
		"refresh",
		"start",
		"stop",
		"info",
		"exec",
		"shell",
		"run",
		"remove",
		"remount",
		"connections",
		"connect",
		"disconnect",
		"sketch-sdk",
		"sketches",
	}
	if !slices.Contains(requireProject, cmd.Name()) {
		// Project might be unknown (e.g. for `workshop launch`); any
		// directory is potentially a new project.
		return nil, cobra.ShellCompDirectiveFilterDirs
	}

	cli, err := c.client()
	if err != nil {
		cobra.CompDebugln(err.Error(), false)
		return nil, cobra.ShellCompDirectiveFilterDirs
	}

	projects, err := cli.Projects()
	if err != nil {
		cobra.CompDebugln(err.Error(), false)
		return nil, cobra.ShellCompDirectiveFilterDirs
	}

	// We can complete absolute or relative paths. Unfortunately, paths
	// starting with ~/ aren't supported. If we return a path starting with
	// ~, cobra will incorrectly escape it to \~ for both bash and zsh.
	var completions []string
	if filepath.IsAbs(toComplete) {
		completions = completeAbsProjectPaths(projects)
	} else {
		completions, err = completeRelProjectPaths(projects)
		if err != nil {
			cobra.CompDebugln(err.Error(), false)
			return nil, cobra.ShellCompDirectiveFilterDirs
		}
	}

	// Filter completions by prefix. Cobra usually does this for us, but
	// if there aren't any matches we'd like to fall back to completing
	// directories only. This doesn't work when toComplete was expanded
	// from shell syntax (e.g. ~/...), but there's no way to distinguish
	// that from the absolute path case.
	completions = slices.DeleteFunc(completions, func(path string) bool {
		return !strings.HasPrefix(path, toComplete)
	})
	if len(completions) == 0 {
		return nil, cobra.ShellCompDirectiveFilterDirs
	}

	return completions, cobra.ShellCompDirectiveDefault
}

func completeAbsProjectPaths(projects []client.Project) []string {
	completions := make([]string, 0, len(projects))
	for _, p := range projects {
		completions = append(completions, p.Path)
	}
	return completions
}

func completeRelProjectPaths(projects []client.Project) ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	completions := make([]string, 0, len(projects))
	for _, p := range projects {
		path, err := filepath.Rel(cwd, p.Path)
		if err != nil {
			return nil, err
		}
		completions = append(completions, path)
	}
	return completions, nil
}

func (c *CmdRoot) completeWorkshopName(status []string) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		return c.doCompleteWorkshopNames(args, status)
	}
}

func (c *CmdRoot) completeWorkshopNames(status []string) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return c.doCompleteWorkshopNames(args, status)
	}
}

func (c *CmdRoot) doCompleteWorkshopNames(args []string, status []string) ([]string, cobra.ShellCompDirective) {
	cli, err := c.client()
	if err != nil {
		cobra.CompDebugln(err.Error(), false)
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	project, err := cli.Project(c.project())
	if err != nil {
		cobra.CompDebugln(err.Error(), false)
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return completeWorkshopNames(cli, project, args, status)
}

func completeWorkshopNames(cli *client.Client, project *client.Project, args []string, status []string) ([]string, cobra.ShellCompDirective) {
	workshopInfo, _, err := cli.List(&client.ListOptions{ProjectId: project.Id})
	if err != nil {
		cobra.CompDebugln(err.Error(), false)
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	desiredStatus := func(s string) bool {
		if status == nil {
			return true
		}
		return slices.Contains(status, s)
	}

	var workshops []string
	for _, workshop := range workshopInfo {
		if desiredStatus(workshop.Status) && !slices.Contains(args, workshop.Name) {
			workshops = append(workshops, workshop.Name)
		}
	}
	return workshops, cobra.ShellCompDirectiveNoFileComp
}

var (
	Stdin  io.Reader = os.Stdin
	Stdout io.Writer = os.Stdout
	Stderr io.Writer = os.Stderr
)

// ClientConfig is the configuration of the Client used by all commands.
var ClientConfig = client.Config{
	Socket: dirs.SocketPath,
}
