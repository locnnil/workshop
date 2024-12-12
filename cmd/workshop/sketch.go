package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/canonical/lxd/shared"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

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
		Use:   "sketch-sdk [--stash|--restore|--remove] <WORKSHOP>",
		Args:  cobra.ExactArgs(1),
		Short: "Edit the sketch SDK and graft it onto the workshop",
		Long: `
This command opens the default text editor to configure the 'sketch' SDK
and immediately installs it in the specified workshop,
enabling rapid experiments and tweaks at the SDK level.

Saving and exiting causes a refresh,
which installs the updated 'sketch' SDK in the workshop.

The '--stash' and '--restore' options stash the 'sketch' SDK,
reversing the changes, and quickly restore it to the workshop.

The '--remove' option removes the 'sketch' SDK permanently.

Notes:

- The 'sketch' SDK doesn't appear in the workshop definition
  and cannot include build-time data such as parts

- In addition to hooks, the 'sketch' SDK can use interfaces,
  define plugs, slots, connections and bindings

- You can partially refresh the workshop, targeting the 'sketch' SDK
  with the 'workshop refresh <WORKSHOP>/sketch' command
`,
		RunE: c.Run,
	}

	cmd.Flags().BoolVar(&c.stash, "stash", false, "Stash the sketch SDK and remove it from the workshop.")
	cmd.Flags().BoolVar(&c.restore, "restore", false, "Return the previously stashed SDK to the workshop.")
	cmd.Flags().BoolVar(&c.remove, "remove", false, "Remove the sketch SDK from the workshop.")

	cmd.MarkFlagsMutuallyExclusive("stash", "restore", "remove")

	return cmd
}

var sketchTemplate = `# Sketch SDK for %s
# Sketch SDK provides local customisation of this specific workshop.

# To read more about SDKs, their components and syntax, see:
# https://canonical-workshop.readthedocs-hosted.com/en/latest/explanation/sdks/
name: sketch
base: %s

hooks:
  # EXAMPLE: setup-base runs once at workshop launch, use it to install some packages.
  # See https://canonical-workshop.readthedocs-hosted.com/en/latest/explanation/sdks/hooks/
  # setup-base: |
    # apt-get install -y --no-install-recommends PACKAGE...
    # snap install SNAP...

  # EXAMPLE: check-health runs after all SDK setup completes, call 'workshopctl set-health okay' for OK.
  # See https://canonical-workshop.readthedocs-hosted.com/en/latest/explanation/sdks/hooks/
  # check-health: |
    # if CHECK_HEALTH_COMMAND ; then
    #   workshopctl set-health okay
    # else
    #   workshopctl set-health --code=installation-fails error "Installation fails"
    # fi

plugs:
  # EXAMPLE: forward your SSH agent into the workshop enabling 'git push' inside the workshop.
  # See https://canonical-workshop.readthedocs-hosted.com/en/latest/explanation/interfaces/ssh-interface/
  # ssh-agent:
  #   interface: ssh-agent

  # EXAMPLE: expose well-known config file locations to the workshop
  # See https://canonical-workshop.readthedocs-hosted.com/en/latest/explanation/interfaces/mount-interface/
  # vs-code-settings:
  #   interface: mount
  #   workshop-target: /home/workshop/.config/Code/User
`

var (
	runTextEditor = shared.TextEditor
)

func stashSketch(sketchdir, stashdir string) (*revert.Reverter, error) {
	reverter := revert.New()

	recs, err := os.ReadDir(sketchdir)
	if err != nil && !osutil.IsDirNotExist(err) {
		return nil, err
	}
	if len(recs) == 0 {
		// Nothing to do.
		return nil, fmt.Errorf(`cannot stash: the 'sketch' SDK doesn't exist`)
	}

	if err := os.MkdirAll(stashdir, 0755); err != nil {
		return nil, err
	}

	if err := osutil.Exchange(sketchdir, stashdir); err != nil {
		return nil, err
	}
	reverter.Add(func() { _ = osutil.Exchange(stashdir, sketchdir) })

	if err := os.RemoveAll(sketchdir); err != nil {
		return nil, err
	}
	reverter.Add(func() { _ = os.MkdirAll(sketchdir, 0755) })
	return reverter, nil
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

	if err := os.MkdirAll(sketchdir, 0755); err != nil {
		return err
	}

	if err = osutil.Exchange(sketchdir, stashdir); err != nil {
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

	return os.RemoveAll(sketchdir)
}

func (c *CmdSketch) Run(cmd *cobra.Command, av []string) error {
	cli, err := c.root.client()
	if err != nil {
		return err
	}
	p, err := cli.Project(c.root.project)
	if err != nil {
		return err
	}

	wp, err := cli.Workshop(p.Id, av[0])
	if err != nil {
		return err
	}

	user, err := osutil.UserMaybeSudoUser()
	if err != nil {
		return err
	}

	sketchdir := sdk.WorkshopSketchSdkCurrent(user.HomeDir, p.Id, wp.Name)

	if c.stash {
		stashdir := sdk.WorkshopSketchSdkStash(user.HomeDir, p.Id, wp.Name)
		reverter, err := stashSketch(sketchdir, stashdir)
		if err != nil {
			return err
		}
		defer reverter.Fail()

		cmdrefresh := &CmdRefresh{root: c.root}
		if err = cmdrefresh.Run(cmd, av[0:1]); err != nil {
			// Refresh failed, revert the stash operation so a possible subsequent
			// "workshop refresh <WORKSHOP>/sketch" won't fail due to the lack of
			// sketch SDK definition.
			return err
		}
		reverter.Success()
		return nil
	}

	if c.restore {
		cmdrefresh := &CmdRefresh{root: c.root}
		cmdrefresh.WaitOnError = true

		storedir := sdk.WorkshopSketchSdkStash(user.HomeDir, p.Id, wp.Name)

		if err = restoreSketch(sketchdir, storedir); err != nil {
			return err
		}

		// Run refresh with the stored sketch SDK. We do not revert dirs exchange
		// on a failed refresh here as it is run with the content from "stored"
		// and with --wait-on-error. Hence, there is always a possibility to
		// workshop refresh --abort and workshop sketch --restore to restore the
		// original sketch content.
		return cmdrefresh.Run(cmd, []string{fmt.Sprintf("%s/sketch", av[0])})
	}

	if c.remove {
		if err := removeSketch(sketchdir); err != nil {
			return err
		}

		cmdrefresh := &CmdRefresh{root: c.root}
		return cmdrefresh.Run(cmd, av[0:1])
	}

	metafile := filepath.Join(sketchdir, "meta", "sdk.yaml")
	boilerplate := fmt.Sprintf(sketchTemplate, workshop.Filepath(p.Path, wp.Name), wp.Base)

	if osutil.FileExists(metafile) {
		old, err := os.ReadFile(metafile)
		if err != nil {
			return err
		}

		new, err := runTextEditor(metafile, []byte{})
		if err != nil {
			return err
		}

		if bytes.Equal(old, new) {
			return nil
		}

		if err = writeSketchSdk(sketchdir, new); err != nil {
			return err
		}
	} else {
		res, err := runTextEditor("", []byte(boilerplate))
		if err != nil {
			return err
		}

		if err = writeSketchSdk(sketchdir, res); err != nil {
			return err
		}
	}

	cmdrefresh := &CmdRefresh{root: c.root}
	cmdrefresh.WaitOnError = true

	return cmdrefresh.Run(cmd, []string{fmt.Sprintf("%s/sketch", av[0])})
}

func writeSketchSdk(sketchdir string, content []byte) error {
	var rec workshop.SdkRecord
	r := revert.New()
	defer r.Fail()

	if err := yaml.Unmarshal(content, &rec); err != nil {
		return err
	}

	if rec.Name != sdk.Sketch {
		return fmt.Errorf("cannot sketch: SDK name must be %q", sdk.Sketch)
	}

	metadir := filepath.Join(sketchdir, "meta")
	metapath := filepath.Join(metadir, "sdk.yaml")
	if err := os.MkdirAll(metadir, 0755); err != nil {
		return err
	}
	r.Add(func() { os.RemoveAll(metadir) })
	if err := os.WriteFile(metapath, content, 0644); err != nil {
		return err
	}

	hooksdir := filepath.Join(sketchdir, "hooks")
	if len(rec.Hooks) > 0 {
		if err := os.MkdirAll(hooksdir, 0755); err != nil {
			return err
		}
		r.Add(func() { os.RemoveAll(hooksdir) })
	}
	for _, hook := range []string{"setup-base", "save-state", "restore-state", "check-health"} {
		hookpath := filepath.Join(hooksdir, hook)
		if script := rec.Hooks[hook]; len(script) > 0 {
			if err := os.WriteFile(hookpath, []byte(script), 0644); err != nil {
				return err
			}
		} else {
			if err := os.Remove(hookpath); err != nil && !osutil.IsDirNotExist(err) {
				return err
			}
		}
	}

	r.Success()
	return nil
}
