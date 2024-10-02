package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/canonical/lxd/shared"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/revert"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/spf13/cobra"
)

type CmdHack struct {
	root    *CmdRoot
	drop    bool
	restore bool
}

func (c *CmdHack) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "hack <WORKSHOP>",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Edit hack SDK",
		RunE:  c.Run,
	}

	cmd.Flags().BoolVar(&c.drop, "drop", false, "Put hack SDK away and refresh.")
	cmd.Flags().BoolVar(&c.restore, "restore", false, "Restore hack SDK and refresh.")

	return cmd
}

var hackTemplate = `name: hack
base: %s
`

func (c *CmdHack) Run(cmd *cobra.Command, av []string) error {
	if c.drop && c.restore {
		return fmt.Errorf("cannot hack: '--drop' incompatible with '--replace'")
	}
	if (c.drop || c.restore) && len(av) != 1 {
		return fmt.Errorf("cannot hack: --drop or --replace require a single workshop name")
	}

	cli, err := c.root.client()
	if err != nil {
		return err
	}
	project, err := cli.Project(c.root.project)
	if err != nil {
		return err
	}

	workshop, err := cli.Workshop(project.Id, av[0])
	if err != nil {
		return err
	}

	user, err := osutil.UserMaybeSudoUser()
	if err != nil {
		return err
	}

	uid, gid, err := osutil.UidGid(user)
	if err != nil {
		return err
	}

	hackdir := sdk.WorkshopHackSdkCurrent(user.HomeDir, project.Id, workshop.Name)

	if c.drop {
		recs, err := os.ReadDir(hackdir)
		if err != nil && !osutil.IsDirNotExist(err) {
			return err
		}
		if len(recs) == 0 {
			// Nothing to do.
			return fmt.Errorf(`cannot drop: "hack" SDK does not exist`)
		}

		reverter := revert.New()
		defer reverter.Fail()

		stored := sdk.WorkshopHackSdkStored(user.HomeDir, project.Id, workshop.Name)
		if err := osutil.MkdirAllChown(stored, 0755, uid, gid); err != nil {
			return err
		}

		if err = osutil.Exchange(hackdir, stored); err != nil {
			return err
		}
		reverter.Add(func() { _ = osutil.Exchange(stored, hackdir) })

		if err = os.RemoveAll(hackdir); err != nil {
			return err
		}

		cmdrefresh := &CmdRefresh{root: c.root}
		if err = cmdrefresh.Run(cmd, []string{fmt.Sprintf("%s", av[0])}); err != nil {
			// Refresh failed, revert the drop operation so a possible subsequent
			// "workshop refresh <WORKSHOP>/hack" won't fail due to the lack of
			// hack SDK definition.
			return err
		}
		reverter.Success()
		return nil
	}

	if c.restore {
		cmdrefresh := &CmdRefresh{root: c.root}
		cmdrefresh.WaitOnError = true

		stored := sdk.WorkshopHackSdkStored(user.HomeDir, project.Id, workshop.Name)
		recs, err := os.ReadDir(stored)
		if err != nil && !osutil.IsDirNotExist(err) {
			return err
		}
		if len(recs) == 0 || osutil.IsDirNotExist(err) {
			// Nothing in stored.
			return fmt.Errorf(`cannot restore: no stored "hack" SDK found`)
		}

		// If hack does not exist (i.e. was dropped) - create it, we'll be
		// exchanging an empty hackdir with the content from stored in this case.
		if err := osutil.MkdirAllChown(hackdir, 0755, uid, gid); err != nil {
			return err
		}

		if err = osutil.Exchange(hackdir, stored); err != nil {
			return err
		}

		// Run refresh with the stored hack SDK. We do not revert dirs exchange
		// on a failed refresh here as it is run with the content from "stored"
		// and with --wait-on-error. Hence, there is always a possibility to
		// workshop refresh --abort and workshop hack --restore to restore the
		// original hack content.
		return cmdrefresh.Run(cmd, []string{fmt.Sprintf("%s/hack", av[0])})
	}

	var sdkfile, content string
	if len(av) == 1 {
		sdkfile = filepath.Join(hackdir, "meta", "sdk.yaml")
		content = fmt.Sprintf(hackTemplate, workshop.Base)
	} else {
		switch av[1] {
		case "setup-base", "save-state", "restore-state", "check-health":
			sdkfile = filepath.Join(hackdir, "hooks", av[1])
		default:
			return fmt.Errorf("cannot hack: unknown %q SDK hook, supported hooks: setup-base, save-state, restore-state, check-health", av[1])
		}
	}

	if osutil.FileExists(sdkfile) {
		old, err := os.ReadFile(sdkfile)
		if err != nil {
			return err
		}

		new, err := shared.TextEditor(sdkfile, []byte{})
		if err != nil {
			return err
		}

		if bytes.Equal(old, new) {
			return nil
		}
	} else {
		if err := osutil.MkdirAllChown(filepath.Dir(sdkfile), 0755, uid, gid); err != nil {
			return err
		}
		content, err := shared.TextEditor("", []byte(content))
		if err != nil {
			return err
		}

		if err = os.WriteFile(sdkfile, content, 0644); err != nil {
			return err
		}
	}

	cmdrefresh := &CmdRefresh{root: c.root}
	cmdrefresh.WaitOnError = true

	return cmdrefresh.Run(cmd, []string{fmt.Sprintf("%s/hack", av[0])})
}
