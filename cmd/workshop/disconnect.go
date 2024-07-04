package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/canonical/workshop/client"
)

type CmdDisconnect struct {
	waitMixin
	forget bool
}

func (c *CmdDisconnect) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "disconnect <WORKSHOP>/<SDK>:<PLUG OR SLOT> [<WORKSHOP>/<SDK>]:[<SLOT>]",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Disconnect a plug or a slot.",
		Long: `
This command disconnects a plug from its slot, or a slot from all its plugs.

- A single argument can be a fully qualified plug or slot reference;
  with two arguments, the first one is the plug, and the second one is the slot

- If the second argument only names the slot itself, the target is
  <WORKSHOP>/agent:<SLOT>; <WORKSHOP> comes from the first argument

- If the second argument only names the workshop and SDK, the target is
  <WORKSHOP>/<SDK>:<INTERFACE>; <INTERFACE> comes from the plug definition

Notes:
- After an auto-connected plug is thus disconnected,
  it is reconnected during 'workshop refresh'
  only if the '--forget' option was used with 'workshop disconnect'
`,
		RunE:    c.Run,
		PostRun: postRunWarnings(&c.clientMixin),
	}

	cmd.PersistentFlags().BoolVar(&c.forget, "forget",
		false,
		"Reconnect the plugs at 'workshop refresh' if auto-connected initially")

	cmd.PersistentFlags().BoolVar(&c.NoWait, "no-wait",
		false,
		"Return the change ID, don't wait for the operation to finish")

	return cmd
}

func (c *CmdDisconnect) Run(cmd *cobra.Command, av []string) error {
	var err error

	cli, err := client.New(&ClientConfig)
	if err != nil {
		return fmt.Errorf("cannot create client: %v", err)
	}

	c.setClient(cli)

	project, err := c.cli.Project(Project)
	if err != nil {
		return err
	}

	plugRef, err := client.ParseShortPlugRef(av[0])
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
			slotRef, err = client.ParseShortSlotRef(av[1])
			if err != nil {
				return err
			}
		}
		slotRef.ProjectId = plugRef.ProjectId
	}

	var opts = client.DisconnectOptions{Forget: c.forget}
	changeId, err := c.cli.Disconnect(plugRef.ProjectId, plugRef.Workshop, plugRef.Sdk, plugRef.Name,
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
