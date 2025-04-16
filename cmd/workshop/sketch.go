package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/canonical/lxd/shared"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/revert"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
)

type CmdSketch struct {
	root    *CmdRoot
	stash   bool
	restore bool
	remove  bool
}

func (c *CmdSketch) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "sketch-sdk [--stash|--restore|--remove] [<WORKSHOP>]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Edit the sketch SDK and graft it onto the workshop",
		Long: `
This opens the 'sketch' SDK definition in the default text editor,
enabling rapid experiments and tweaks at the SDK level.

Saving the definition and exiting the editor causes a refresh,
which installs the configured 'sketch' SDK in the workshop.

The '--stash' and '--restore' options respectively stash the SDK,
reversing the changes, and quickly restore it to the workshop.
The '--remove' option removes the SDK permanently.

Notes:

- The 'sketch' SDK doesn't appear in the workshop definition
  and cannot include build-time data such as parts.

- In addition to hooks, the 'sketch' SDK can use interfaces,
  define plugs, slots, connections and bindings.

- You can partially refresh the workshop, targeting the 'sketch' SDK
  with the 'workshop refresh <WORKSHOP>/sketch' command.
`,
		Example: `
Edit the sketch SDK definition for the 'nimble' workshop
and apply it after saving by automatically refreshing the workshop:
$ workshop sketch-sdk nimble

The name is optional if the project has only one workshop:
$ workshop sketch-sdk

Stash the sketch SDK, temporarily reverting the changes in the workshop:
$ workshop sketch-sdk nimble --stash`,
		RunE:              c.Run,
		ValidArgsFunction: c.root.completeWorkshopName([]string{"Ready", "Waiting"}),
	}

	cmd.Flags().BoolVar(&c.stash, "stash", false, "Stash the sketch SDK and remove it from the workshop.")
	cmd.Flags().BoolVar(&c.restore, "restore", false, "Return the previously stashed SDK to the workshop.")
	cmd.Flags().BoolVar(&c.remove, "remove", false, "Remove the sketch SDK from the workshop.")

	cmd.MarkFlagsMutuallyExclusive("stash", "restore", "remove")

	return cmd
}

var sketchTemplate = `# Sketch SDK for %s
# Sketch SDK provides local customisation of this specific workshop.

# To read more about the sketch SDK, its and syntax, see:
# https://canonical-workshop.readthedocs-hosted.com/en/latest/explanation/sdks/sdks/#sketch-sdk
name: sketch

hooks:
  # EXAMPLE: setup-base runs once at workshop launch, use it to install some packages.
  # setup-base: |
    # apt-get update
    # apt-get install PACKAGE...
    # snap install SNAP...

  # EXAMPLE: check-health runs after all SDK setup completes, call 'workshopctl set-health okay' for OK.
  # check-health: |
    # if CHECK_HEALTH_COMMAND ; then
    #   workshopctl set-health okay
    # else
    #   workshopctl set-health --code=installation-failed error "Installation failed"
    # fi

plugs:
  # EXAMPLE: forward your SSH agent into the workshop enabling 'git push' inside the workshop.
  # ssh-agent:
  #   interface: ssh-agent

  # EXAMPLE: expose well-known config file locations to the workshop
  # vs-code-settings:
  #   interface: mount
  #   workshop-target: /home/workshop/.config/Code/User

slots:
  # EXAMPLE: expose SDK services to the host
  # dashboard:
  #   interface: tunnel
  #   endpoint: 8080
`

var (
	runTextEditor = shared.TextEditor
)

func stashSketch(sketchdir, stashdir string) (*revert.Reverter, error) {
	recs, err := os.ReadDir(sketchdir)
	if err != nil && !osutil.IsDirNotExist(err) {
		return nil, err
	}
	if len(recs) == 0 {
		// Nothing to do.
		return nil, fmt.Errorf(`cannot stash: the 'sketch' SDK doesn't exist`)
	}

	// Ensure stashdir exists but is empty.
	if err := clearStash(stashdir); err != nil {
		return nil, err
	}

	reverter := revert.New()
	defer reverter.Fail()

	if err := osutil.Exchange(sketchdir, stashdir); err != nil {
		return nil, err
	}
	reverter.Add(func() { _ = osutil.Exchange(stashdir, sketchdir) })

	if err := os.Remove(sketchdir); err != nil {
		return nil, err
	}
	reverter.Add(func() { _ = os.MkdirAll(sketchdir, 0755) })

	clone := reverter.Clone()
	reverter.Success()
	return clone, nil
}

func clearStash(stashdir string) error {
	temp, err := os.MkdirTemp(filepath.Dir(stashdir), "stash-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	if err := os.Chmod(temp, 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(stashdir, 0755); err != nil {
		return err
	}
	return osutil.Exchange(stashdir, temp)
}

func restoreSketch(sketchdir, stashdir string) error {
	recs, err := os.ReadDir(stashdir)
	if err != nil && !osutil.IsDirNotExist(err) {
		return err
	}
	if len(recs) == 0 || osutil.IsDirNotExist(err) {
		// Nothing in stored.
		return fmt.Errorf(`cannot restore: no stashed 'sketch' SDK found`)
	}

	// We don't overwrite current sketch SDK as opposed to overwriting stashed
	// sketch SDK.
	if _, err = os.Stat(sketchdir); err == nil {
		return fmt.Errorf(`cannot restore: the 'sketch' SDK exists; run 'workshop sketch-sdk --remove' to remove it from the workshop`)
	}

	if err = osutil.Rename(stashdir, sketchdir); err != nil {
		return err
	}
	return nil
}

func removeSketch(sketchdir string) error {
	_, err := os.Stat(sketchdir)
	if err != nil && !osutil.IsDirNotExist(err) {
		return err
	}
	if osutil.IsDirNotExist(err) {
		// Nothing to do.
		return fmt.Errorf(`cannot remove: the 'sketch' SDK doesn't exist`)
	}

	temp, err := os.MkdirTemp(filepath.Dir(sketchdir), "sketch-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	if err := os.Chmod(temp, 0755); err != nil {
		return err
	}

	if err := osutil.Exchange(sketchdir, temp); err != nil {
		return err
	}

	return os.Remove(sketchdir)
}

func (c *CmdSketch) Run(cmd *cobra.Command, av []string) error {
	cli, err := c.root.client()
	if err != nil {
		return err
	}
	p, err := cli.Project(c.root.project())
	if err != nil {
		return err
	}

	var wp *client.Workshop
	if len(av) > 0 {
		wp, err = cli.Workshop(p.Id, av[0])
		if err != nil {
			return err
		}
	} else {
		wp, err = cli.SingleWorkshop(p)
		if err != nil {
			return err
		}
	}

	// Ensure that the workshop is Ready, aborting previous sketch if necessary.
	if wp.Status == "Waiting" {
		fmt.Fprintf(Stdout, "Reverting incomplete change for %q...\n", wp.Name)
		cmdabort := &CmdRefresh{root: c.root, Abort: true}
		if err = cmdabort.RunRefresh(cli, p, []string{wp.Name}); err != nil {
			return err
		}
	} else if wp.Status != "Ready" {
		return fmt.Errorf(`error: cannot sketch %q: workshop currently %q, must be "Ready"`, wp.Name, wp.Status)
	}

	user, env, err := osutil.CurrentUserAndEnv()
	if err != nil {
		return err
	}

	userDataDir := workshop.UserDataRootDir(user.HomeDir, env)
	sketchdir := workshop.SketchSdkCurrent(userDataDir, p.Id, wp.Name)

	if c.stash {
		stashdir := workshop.SketchSdkStash(userDataDir, p.Id, wp.Name)

		reverter, err := stashSketch(sketchdir, stashdir)
		if err != nil {
			return err
		}
		defer reverter.Fail()

		cmdrefresh := &CmdRefresh{root: c.root}
		if err = cmdrefresh.RunRefresh(cli, p, []string{wp.Name}); err != nil {
			// Refresh failed, revert the stash operation so a possible subsequent
			// "workshop refresh <WORKSHOP>/sketch" won't fail due to the lack of
			// sketch SDK definition.
			return err
		}
		fmt.Fprintf(Stdout, "%q sketch stashed\n", wp.Name)
		reverter.Success()
		return nil
	}

	if c.restore {
		cmdrefresh := &CmdRefresh{root: c.root}
		cmdrefresh.WaitOnError = true

		stashdir := workshop.SketchSdkStash(userDataDir, p.Id, wp.Name)

		if err = restoreSketch(sketchdir, stashdir); err != nil {
			return err
		}

		// Run refresh with the stashed sketch SDK. We do not revert dirs exchange
		// on a failed refresh here as it is run with the content from "stored"
		// and with --wait-on-error. Hence, there is always a possibility to
		// workshop refresh --abort and workshop sketch-sdk --stash to restore the
		// original stash content.
		if err = cmdrefresh.RunRefresh(cli, p, []string{wp.Name}); err != nil {
			return err
		}
		fmt.Fprintf(Stdout, "%q sketch restored\n", wp.Name)
		return nil
	}

	if c.remove {
		if err := removeSketch(sketchdir); err != nil {
			return err
		}

		cmdrefresh := &CmdRefresh{root: c.root}
		if err = cmdrefresh.RunRefresh(cli, p, []string{wp.Name}); err != nil {
			return err
		}
		fmt.Fprintf(Stdout, "%q sketch removed\n", wp.Name)
		return nil
	}

	if err = editSketchSdk(sketchdir, wp.Path); err != nil {
		return err
	}

	cmdrefresh := &CmdRefresh{root: c.root}
	cmdrefresh.WaitOnError = true

	if err = cmdrefresh.RunRefresh(cli, p, []string{wp.Name}); err != nil {
		return err
	}
	fmt.Fprintf(Stdout, "%q sketch refreshed\n", wp.Name)
	return nil
}

func editSketchSdk(sketchdir, workshopFile string) error {
	content, err := os.ReadFile(filepath.Join(sketchdir, "meta", "sdk.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(sketchdir, 0755); err != nil {
			return err
		}

		// Format sketch SDK template header.
		content = []byte(fmt.Sprintf(sketchTemplate, workshopFile))
	} else if err != nil {
		return err
	}

	temp, err := os.MkdirTemp(filepath.Dir(sketchdir), "current-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	if err := os.Chmod(temp, 0755); err != nil {
		return err
	}

	target := filepath.Join(temp, "meta", "sdk.yaml")
	if err := writeSketchSdk(target, content); err != nil {
		return err
	}
	content, err = runTextEditor(target, content)
	if err != nil {
		return err
	}
	if err := writeSketchHooks(temp, content); err != nil {
		return err
	}

	return osutil.Exchange(temp, sketchdir)
}

func writeSketchSdk(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}

func writeSketchHooks(sketchdir string, content []byte) error {
	var rec workshop.SdkRecord
	if err := yaml.Unmarshal(content, &rec); err != nil {
		return err
	}

	if !sdk.IsSketch(rec.Name) {
		return fmt.Errorf("cannot sketch: SDK must be named %q (now: %q)", sdk.Sketch, rec.Name)
	}

	hooksdir := filepath.Join(sketchdir, "sdk", "hooks")
	if len(rec.Hooks) > 0 {
		if err := os.MkdirAll(hooksdir, 0755); err != nil {
			return err
		}
	}

	for _, hook := range []string{"setup-base", "save-state", "restore-state", "check-health"} {
		hookpath := filepath.Join(hooksdir, hook)
		if script := rec.Hooks[hook]; len(script) > 0 {
			if err := os.WriteFile(hookpath, []byte(script+"\n"), 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

type CmdSketches struct {
	root *CmdRoot
}

func (c *CmdSketches) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "sketches",
		Args:  cobra.ExactArgs(0),
		Short: "List sketches",
		Long: `
This command enumerates all sketches in the project, printing a compact list:

- Project:  absolute pathname of the project

- Workshop: workshop name, as set in its definition

- Rev:      sketch SDK revision, if present

- Notes:    current, stashed, or both
`,
		Example: `
List the sketches in the current project directory:
$ workshop sketches`,
		RunE: c.Run,
	}

	return cmd
}

func (c *CmdSketches) Run(cmd *cobra.Command, _ []string) error {
	cli, err := c.root.client()
	if err != nil {
		return err
	}

	w := tabWriter()

	p, err := cli.Project(c.root.project())
	if err != nil {
		return err
	}

	wps, _, err := cli.List(&client.ListOptions{ProjectId: p.Id})
	if err != nil {
		return err
	}

	user, env, err := osutil.CurrentUserAndEnv()
	if err != nil {
		return err
	}

	userDataDir := workshop.UserDataRootDir(user.HomeDir, env)

	var entries []string
	for _, wp := range wps {
		entry, err := stashEntry(userDataDir, wp, p)
		if err != nil {
			return err
		}

		if entry != nil {
			entries = append(entries, strings.Join(entry, "\t"))
		}
	}

	if entries != nil {
		fmt.Fprintln(w, "Project\tWorkshop\tRev\tNotes")
		fmt.Fprintln(w, strings.Join(entries, "\n"))
	}

	w.Flush()

	return nil
}

func stashEntry(userDataDir string, w *client.WorkshopInfo, p *client.Project) ([]string, error) {
	rev := "-"
	notes := ""
	exists := false
	idx := slices.IndexFunc(w.Sdks, func(s *client.Sdk) bool { return sdk.IsSketch(s.Name) })
	if idx != -1 {
		info := w.Sdks[idx]
		rev = info.Revision
		notes = "current"
		exists = true
	}

	stashdir := workshop.SketchSdkStash(userDataDir, p.Id, w.Name)

	if osutil.IsDir(stashdir) {
		if len(notes) > 0 {
			notes += ","
		}
		notes += "stashed"
		exists = true
	}

	if !exists {
		return nil, nil
	}

	return []string{contractHomeDirectory(p.Path), w.Name, rev, notes}, nil
}
