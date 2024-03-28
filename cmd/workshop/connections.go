package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/canonical/workshop/client"
	"github.com/canonical/x-go/i18n"
	"github.com/spf13/cobra"
)

type CmdConnections struct {
	clientMixin
	all bool
}

func (c *CmdConnections) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "connections [<WORKSHOP>] [--all]",
		Args:  cobra.MaximumNArgs(1),
		Short: "List interface connections.",
		RunE:  c.Run,
	}

	cmd.Flags().BoolVar(&c.all, "all", false, "Lists connected and unconnected plugs and slots.")

	return cmd
}

func isAgentSdk(sdkName string) bool {
	return sdkName == "agent"
}

func endpoint(workshop, sdkName, name string) string {
	if isAgentSdk(sdkName) {
		return ":" + name
	}
	return workshop + "/" + sdkName + ":" + name
}

type connection struct {
	slot          string
	plug          string
	interfaceName string
	manual        bool
}

func (cn connection) String() string {
	opts := []string{}
	if cn.manual {
		opts = append(opts, "manual")
	}
	if len(opts) == 0 {
		return "-"
	}
	return strings.Join(opts, ",")
}

type byConnectionData []connection

func (b byConnectionData) Len() int      { return len(b) }
func (b byConnectionData) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byConnectionData) Less(i, j int) bool {
	iCon, jCon := b[i], b[j]
	if iCon.interfaceName != jCon.interfaceName {
		return iCon.interfaceName < jCon.interfaceName
	}
	if iCon.plug != jCon.plug {
		return iCon.plug < jCon.plug
	}
	return iCon.slot < jCon.slot
}

func (c *CmdConnections) Run(cmd *cobra.Command, av []string) error {
	var err error

	cli, err := client.New(&ClientConfig)
	if err != nil {
		return fmt.Errorf("cannot create client: %v", err)
	}

	c.setClient(cli)
	defer func() {
		if cli != nil {
			maybePresentWarnings(cli.WarningsSummary())
		}
	}()

	project, err := c.client.Project(Project)
	if err != nil {
		return err
	}

	workshop := ""
	if len(av) > 0 {
		workshop = av[0]
		if c.all {
			// passing a workshop name already implies --all, error out
			// when it was passed explicitly
			return fmt.Errorf("cannot use --all with workshop name")
		}
		c.all = true
	}

	connections, err := c.client.Connections(&client.ConnectionOptions{ProjectId: project.Id, Workshop: workshop, All: c.all})
	if err != nil {
		return err
	}

	if len(connections.Plugs) == 0 && len(connections.Slots) == 0 {
		return nil
	}

	annotatedConns := make([]connection, 0, len(connections.Established)+len(connections.Undesired))
	for _, conn := range connections.Established {
		annotatedConns = append(annotatedConns, connection{
			plug:          endpoint(conn.Plug.Workshop, conn.Plug.Sdk, conn.Plug.Name),
			slot:          endpoint(conn.Slot.Workshop, conn.Slot.Sdk, conn.Slot.Name),
			manual:        conn.Manual,
			interfaceName: conn.Interface,
		})
	}

	w := tabWriter()
	fmt.Fprintln(w, i18n.G("Interface\tPlug\tSlot\tNotes"))

	for _, plug := range connections.Plugs {
		if len(plug.Connections) == 0 && c.all {
			annotatedConns = append(annotatedConns, connection{
				plug:          endpoint(plug.Workshop, plug.Sdk, plug.Name),
				slot:          "-",
				interfaceName: plug.Interface,
			})
		}
	}
	for _, slot := range connections.Slots {
		if isAgentSdk(slot.Sdk) {
			// displaying unconnected system snap slots is boring,
			// unless explicitly asked to show them
			continue
		}
		if len(slot.Connections) == 0 && c.all {
			annotatedConns = append(annotatedConns, connection{
				plug:          "-",
				slot:          endpoint(slot.Workshop, slot.Sdk, slot.Name),
				interfaceName: slot.Interface,
			})
		}
	}

	sort.Sort(byConnectionData(annotatedConns))

	for _, note := range annotatedConns {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", note.interfaceName, note.plug, note.slot, note)
	}

	if len(annotatedConns) > 0 {
		w.Flush()
	}

	return nil
}
