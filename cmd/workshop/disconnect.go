package main

import (
	"fmt"
	"strings"

	"github.com/canonical/workshop/client"
	"github.com/spf13/cobra"
)

type CmdDisconnect struct {
	waitMixin
	forget bool
}

func (c *CmdDisconnect) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "disconnect [OPTIONS] <workshop>/<sdk>:<plug> [<workshop>/<sdk>]:<slot>",
		Args:  cobra.RangeArgs(1, 2),
		Short: "The disconnect command disconnects a plug from a slot.",
		RunE:  c.Run,
	}

	cmd.PersistentFlags().BoolVar(&c.forget, "forget",
		false,
		"Forget remembered state about the given connection.")

	cmd.PersistentFlags().BoolVar(&c.NoWait, "no-wait",
		false,
		"Do not wait for the operation to finish but just print the change id")

	return cmd
}

func (c *CmdDisconnect) Run(cmd *cobra.Command, av []string) error {
	var err error

	cli, err := client.New(&ClientConfig)
	if err != nil {
		return fmt.Errorf("cannot create client: %v", err)
	}

	c.setClient(cli)

	project, err := c.client.Project(Project)
	if err != nil {
		return err
	}

	plugRef, err := client.ParsePlugRef(av[0])
	if err != nil {
		return err
	}
	plugRef.ProjectId = project.Id

	slotRef := &client.SlotRef{}
	if len(av) > 1 {
		// check if the second arg is a short version of the agent-provided slot reference
		if strings.HasPrefix(av[1], ":") {
			slotRef.Workshop = plugRef.Workshop
			slotRef.Sdk = "agent"
			slotRef.Name = av[1][1:]
		} else {
			slotRef, err = client.ParseSlotRef(av[1])
			if err != nil {
				return err
			}
		}
		slotRef.ProjectId = plugRef.ProjectId
	}

	var opts = client.DisconnectOptions{Forget: c.forget}
	changeId, err := c.client.Disconnect(plugRef.ProjectId, plugRef.Workshop, plugRef.Sdk, plugRef.Name,
		slotRef.ProjectId, slotRef.Workshop, slotRef.Sdk, slotRef.Name, &opts)
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
