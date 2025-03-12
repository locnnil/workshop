package main

import (
	"cmp"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/sdk"
)

type CmdInfo struct {
	waitMixin
	root *CmdRoot
}

func (c *CmdInfo) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "info [<WORKSHOP>]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Print the current status and details of a workshop as YAML",
		Long: `
This command outputs the basic settings, current status and individual SDK
details for a workshop, formatting them as YAML. Specifically, it prints:

- Essential workshop attributes, such as name, base and project directory

- Current status (e.g. 'Ready', 'Pending', 'Stopped') and notes for the workshop

- Individual SDK details, such as name, channel, installation date and revision

- Currently connected mount interface plugs


Notes:

- Avoid assumptions based on SDK channels: 'latest/stable' may be neither.
`,
		Example: `
List details for the 'nimble' workshop in the current project directory:
$ workshop info nimble

The name is optional if the project has only one workshop:
$ workshop info`,
		RunE:              c.Run,
		ValidArgsFunction: c.root.completeWorkshopName(nil),
	}

	return cmd
}

// The default paths created by workshop are not human friendly. Thus, the
// client checks if that path is a lengthy default and substitutes its common
// prefix with .../. Hence something like:
//
//	/home/dmitry/.local/share/workshop/project/17942561/mount/albert_go_mod-cache.sdk
//
// becomes:
//
//	.../17942561/mount/albert_go_mod-cache.sdk
func shortenDefaulPath(source string) string {
	user, _ := user.Current()
	defaultPathPrefix := filepath.Join(user.HomeDir, ".local", "share", "workshop", "project")
	if after, ok := strings.CutPrefix(source, defaultPathPrefix); ok {
		return "..." + after
	}
	return source
}

func (c *CmdInfo) Run(cmd *cobra.Command, av []string) error {
	cli, err := c.root.client()
	if err != nil {
		return err
	}

	project, err := cli.Project(c.root.project())
	if err != nil {
		return err
	}

	if len(av) == 0 {
		name, err := cli.SingleWorkshopName(project)
		if err != nil {
			return err
		}
		av = []string{name}
	}

	workshop, err := cli.Workshop(project.Id, av[0])
	if err != nil {
		return err
	}
	slices.SortFunc(workshop.Sdks, func(a, b *client.Sdk) int { return cmp.Compare(a.Name, b.Name) })

	w := tabWriter()

	fmt.Fprintf(w, "name:\t%s\n", workshop.Name)
	fmt.Fprintf(w, "base:\t%s\n", workshop.Base)
	fmt.Fprintf(w, "project:\t%s\n", project.Path)
	fmt.Fprintf(w, "status:\t%s\n", strings.ToLower(workshop.Status))

	// get the workshop notes
	notes := workshop.Notes

	// get the SDKs notes (if there is an ongoing health check)
	for _, sdk := range workshop.Sdks {
		if sdk.Health != nil && sdk.Health.Code != "" {
			notes = append(notes, sdk.Health.Code)
		}
	}

	// combine notes from workshop and its SDKs
	notesFormatted := strings.Join(notes, ",")
	if len(workshop.Notes) == 0 {
		notesFormatted = "-"
	}

	fmt.Fprintf(w, "notes:\t%s\n", notesFormatted)

	if len(workshop.Sdks) > 0 {
		fmt.Fprintf(w, "sdks:\n")

		var tunnels []*client.Tunnel
		for _, sk := range workshop.Sdks {
			if sk.Tunnels == nil {
				continue
			}
			tunnels = append(tunnels, sk.Tunnels.Slots...)
		}
		if len(tunnels) > 0 {
			fmt.Fprintf(w, "  system:\n")
			fmt.Fprintf(w, "    tunnels:\n")

			slices.SortFunc(tunnels, func(a, b *client.Tunnel) int {
				return cmp.Compare(a.Plug.Name, b.Plug.Name)
			})
			for _, tunnel := range tunnels {
				fmt.Fprintf(w, "      %s:\n", tunnel.Plug.Name)
				fmt.Fprintf(w, "        from:\t%s\n", formatEndpoint(tunnel.From))
				fmt.Fprintf(w, "        to:\t%s\n", formatEndpoint(tunnel.To))
			}
		}

		for _, sk := range workshop.Sdks {
			fmt.Fprintf(w, "  %s:\n", sk.Name)
			if sk.Name == sdk.Sketch {
				sk.Channel = sketchSdkChannel(project.Id, workshop.Name)
				if sk.BuildTime.IsZero() {
					sk.BuildTime = sk.InstallTime
				}
			} else if sk.Channel == "" {
				sk.Channel = "~"
			}
			fmt.Fprintf(w, "    tracking:\t%s\n", sk.Channel)

			var buildTime string
			if !sk.BuildTime.IsZero() {
				buildTime = "\t" + sk.BuildTime.Format(time.DateOnly)
			}
			var version string
			if sk.Version != "" {
				version = "\t" + sk.Version
			}
			fmt.Fprintf(w, "    installed:%s%s\t(%s)\n", version, buildTime, sk.Revision)
			if sk.Health != nil {
				fmt.Fprintf(w, "    message:\t%s\n", sk.Health.Message)
			}

			if len(sk.Mounts) > 0 {
				fmt.Fprintf(w, "    mounts:\n")
				slices.SortFunc(sk.Mounts, func(a, b *client.Mount) int { return cmp.Compare(a.Plug.Name, b.Plug.Name) })
				for _, mount := range sk.Mounts {
					if mount.HostSource != "" {
						fmt.Fprintf(w, "      %s:\n", mount.Plug.Name)
						fmt.Fprintf(w, "        host-source:\t%s\n", shortenDefaulPath(mount.HostSource))
						fmt.Fprintf(w, "        workshop-target:\t%s\n", mount.WorkshopTarget)
						continue
					}
					if mount.WorkshopSource != "" {
						fmt.Fprintf(w, "      %s:\n", mount.Plug.Name)
						fmt.Fprintf(w, "        workshop-source:\t%s\n", mount.WorkshopSource)
						fmt.Fprintf(w, "        workshop-target:\t%s\n", mount.WorkshopTarget)
						continue
					}
				}
			}

			if sk.Tunnels != nil && len(sk.Tunnels.Plugs) > 0 {
				fmt.Fprintf(w, "    tunnels:\n")
				slices.SortFunc(sk.Tunnels.Plugs, func(a, b *client.Tunnel) int {
					return cmp.Compare(a.Plug.Name, b.Plug.Name)
				})
				for _, tunnel := range sk.Tunnels.Plugs {
					fmt.Fprintf(w, "      %s:\n", tunnel.Plug.Name)
					fmt.Fprintf(w, "        from:\t%s\n", formatEndpoint(tunnel.From))
					fmt.Fprintf(w, "        to:\t%s\n", formatEndpoint(tunnel.To))
				}
			}
		}
	}

	w.Flush()

	return nil
}

func sketchSdkChannel(projectId, workshop string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~"
	}

	path := sdk.WorkshopSketchSdk(home, projectId, workshop)
	return contractHomeDirectory(path)
}

func formatEndpoint(endpoint client.Endpoint) string {
	if endpoint.Protocol == "unix" {
		return endpoint.Path
	}

	port := strconv.FormatUint(uint64(endpoint.Port), 10)
	hostPort := net.JoinHostPort(endpoint.Host, port)
	return fmt.Sprintf("%s/%s", hostPort, endpoint.Protocol)
}
