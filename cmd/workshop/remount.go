package main

import (
	"fmt"
	"strings"

	"github.com/canonical/workshop/client"
	"github.com/spf13/cobra"
)

type CmdRemount struct {
	waitMixin
}

func (c *CmdRemount) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "remount <workshop>[/<sdk>]:<plug> <SOURCE>",
		Args:  cobra.ExactArgs(2),
		Short: "Mount the content interface plug's source directory to the new location.",
		RunE:  c.Run,
	}

	cmd.PersistentFlags().BoolVar(&c.NoWait, "no-wait",
		false,
		"Do not wait for the operation to finish but just print the change id")

	return cmd
}

func parsePlugRef(plug string) (*client.PlugRef, error) {
	// the expected format of the plug ref is <workshop>[/<sdk>]:<plug>
	var plugRef client.PlugRef
	parts := strings.Split(plug, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("cannot remount: unknown plug reference %q", plug)
	}

	wssdk := strings.Split(parts[0], "/")
	if len(wssdk) != 2 {
		return nil, fmt.Errorf("cannot remount: unknown plug reference %q", plug)
	}

	plugRef.Workshop = wssdk[0]
	plugRef.Sdk = wssdk[1]
	plugRef.Name = parts[1]
	return &plugRef, nil
}

func (c *CmdRemount) Run(cmd *cobra.Command, av []string) error {
	var err error

	plugRef, err := parsePlugRef(av[0])
	if err != nil {
		return err
	}

	cli, err := client.New(&ClientConfig)
	if err != nil {
		return fmt.Errorf("cannot create client: %v", err)
	}

	c.setClient(cli)

	project, err := c.client.Project(Project)
	if err != nil {
		return err
	}

	plugRef.ProjectId = project.Id

	changeId, err := c.client.Remount(plugRef, av[1])
	if err != nil {
		return err
	}

	if _, err := c.wait(changeId, false); err != nil {
		if err == errNoWait {
			return nil
		}
		return err
	}

	return nil
}
