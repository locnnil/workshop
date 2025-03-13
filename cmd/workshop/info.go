package main

import (
	"cmp"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/canonical/workshop/client"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/sdk"
	"github.com/canonical/workshop/internal/workshop"
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
//	/home/dmitry/.local/share/workshop/id/17942561/ws/mount/go/mod-cache
//
// becomes:
//
//	.../17942561/mount/go/mod-cache
func shortenDefaultPath(source, xdg string) string {
	defaultPathPrefix := filepath.Join(xdg, "workshop", "id")
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

	home, _ := os.LookupEnv("HOME")
	if home == "" {
		usr, err := osutil.UserCurrent()
		if err != nil || usr.HomeDir == "" {
			return fmt.Errorf("cannot show info: could not determine home directory")
		}
		home = usr.HomeDir
	}

	xdg, _ := os.LookupEnv("XDG_DATA_HOME")
	if xdg == "" {
		// If XDG_DATA_HOME is unset, fallback to $HOME/.local/share
		xdg = filepath.Join(home, ".local", "share")
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
		for _, sk := range workshop.Sdks {
			fmt.Fprintf(w, "  %s:\n", sk.Name)

			switch sk.Name {
			case sdk.Sketch:
				sk.Channel = sketchSdkChannel(project.Id, workshop.Name)
				if sk.BuildTime.IsZero() {
					sk.BuildTime = sk.InstallTime
				}
			default:
				if sk.Channel == "" {
					sk.Channel = "~"
				}
			}

			// Tracking info is always the same for the system SDK. Omit it to
			// highlight the difference between it and a regular type SDK.
			if sk.Name != sdk.System.String() {
				fmt.Fprintf(w, "    tracking:\t%s\n", sk.Channel)
			}

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
					hostSource := shortenDefaultPath(mount.HostSource, xdg)
					if mount.HostSource != "" {
						fmt.Fprintf(w, "      %s:\n", mount.Plug.Name)
						fmt.Fprintf(w, "        host-source:\t%s\n", hostSource)
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

func sketchSdkChannel(projectId, w string) string {
	user, env, err := osutil.CurrentUserAndEnv()
	if err != nil {
		return "~"
	}

	userDataDir := workshop.UserDataRootDir(user.HomeDir, env)
	return contractHomeDirectory(workshop.SketchSdkDir(userDataDir, projectId, w))
}

func formatEndpoint(endpoint client.Endpoint) string {
	if endpoint.Protocol == "unix" {
		return endpoint.Path
	}

	port := strconv.FormatUint(uint64(endpoint.Port), 10)
	hostPort := net.JoinHostPort(endpoint.Host, port)
	return fmt.Sprintf("%s/%s", hostPort, endpoint.Protocol)
}
