package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/canonical/lxd/shared"
	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/spf13/cobra"
)

type CmdHack struct {
	clientMixin
}

func (c *CmdHack) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "hack <WORKSHOP>",
		Args:  cobra.RangeArgs(1, 1),
		Short: "Edit scratch SDK",
		RunE:  c.Run,
	}

	return cmd
}

var scratchTemplate = `name: scratch
base: %s
`

func (c *CmdHack) Run(cmd *cobra.Command, av []string) error {
	cli, err := client.New(&ClientConfig)
	if err != nil {
		return fmt.Errorf("cannot create client: %v", err)
	}

	c.setClient(cli)

	project, err := c.cli.Project(Project)
	if err != nil {
		return err
	}

	workshop, err := c.cli.Workshop(project.Id, av[0])
	if err != nil {
		return err
	}

	user, err := osutil.UserMaybeSudoUser()
	if err != nil {
		return err
	}

	scratchdir := sdk.WorkshopScratchSdkDir(user.HomeDir, project.Id, workshop.Name)

	metapath := filepath.Join(scratchdir, "meta", "sdk.yaml")
	if osutil.FileExists(metapath) {
		old, err := os.ReadFile(metapath)
		if err != nil {
			return err
		}

		new, err := shared.TextEditor(metapath, []byte{})
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
		if err := osutil.MkdirAllChown(filepath.Dir(metapath), 0755, uid, gid); err != nil {
			return err
		}
		content, err := shared.TextEditor("", []byte(fmt.Sprintf(scratchTemplate, workshop.Base)))
		if err != nil {
			return err
		}

		if err = os.WriteFile(metapath, content, 0644); err != nil {
			return err
		}
	}

	cmdrefresh := &CmdRefresh{}
	cmdrefresh.WaitOnError = true

	return cmdrefresh.Run(cmd, av)
}
