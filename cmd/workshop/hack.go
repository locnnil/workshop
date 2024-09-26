package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/canonical/lxd/shared"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/spf13/cobra"
)

type CmdHack struct {
	root *CmdRoot
}

func (c *CmdHack) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "hack <WORKSHOP>",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Edit hack SDK",
		RunE:  c.Run,
	}

	return cmd
}

var hackTemplate = `name: hack
base: %s
`

func (c *CmdHack) Run(cmd *cobra.Command, av []string) error {
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

	hackdir := sdk.WorkshopHackSdkDir(user.HomeDir, project.Id, workshop.Name)

	var sdkfile, content string
	if len(av) == 1 {
		sdkfile = filepath.Join(hackdir, "meta", "sdk.yaml")
		content = fmt.Sprintf(hackTemplate, workshop.Base)
	} else {
		switch av[1] {
		case "setup-base", "save-state", "restore-state", "check-health":
			sdkfile = filepath.Join(hackdir, "hooks", av[1])
		default:
			return fmt.Errorf("unknown SDK hook type: %q, supported hooks: setup-base, save-state, restore-state, check-health", av[1])
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
		uid, gid, err := osutil.UidGid(user)
		if err != nil {
			return err
		}
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

	cmdrefresh := &CmdRefresh{}
	cmdrefresh.WaitOnError = true

	return cmdrefresh.Run(cmd, []string{fmt.Sprintf("%s/hack", av[0])})
}
