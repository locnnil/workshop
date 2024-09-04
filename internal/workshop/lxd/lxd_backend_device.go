package lxdbackend

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/canonical/workshop/internal/workshop"
)

func HostWorkshopMount(name, source, target string) workshop.Device {
	return workshop.Device{Name: name,
		Type: workshop.HostWorkshopMount,
		Properties: map[string]string{"type": "disk", "source": source,
			"path": target},
	}
}

func WorkshopToWorkshopMount(name, source, target string) workshop.Device {
	return workshop.Device{Name: name,
		Type:       workshop.WorkshopWorkshopMount,
		Properties: map[string]string{"type": "none"},
	}
}

func Volume(name, mountTo, volume string) workshop.Device {
	return workshop.Device{
		Name: name,
		Type: workshop.DiskVolume,
		Properties: map[string]string{"type": "disk",
			"pool":   "default",
			"path":   mountTo,
			"source": volume},
	}
}

func Gpu(name string) workshop.Device {
	return workshop.Device{
		Name: name,
		Type: workshop.GPU,

		// The default workshop user must be able to acces the GPU device.
		// Workshop assigns the GPU devices to workshop.workshop. A more
		// traditional way here would be to add dri devices to the video/render
		// groups, but it requires an additional workshop exec to find out the
		// groups' ids at the LXD profile generation time. Given that we are
		// solving the problem of access in a confined environment and workshop
		// is a passwordless sudo user anyway, it was decided that it is OK if
		// the workshop user owns GPU devices.

		// On another note, the render and video groups are not assigned to the
		// card*/render* dri devices by LXD properly. Both will be assigned to
		// the group provided in "gid"; there is no way to assign video to card*
		// and render to render* devices.
		Properties: map[string]string{"type": "gpu", "gputype": "physical", "uid": "1000", "gid": "1000"},
	}
}

// A network protocol proxy device, opens a port on the host or in a workhop.
// from, to are the source and destination addresses (paths in the case of unix sockets),
// see https://documentation.ubuntu.com/lxd/en/latest/reference/devices_proxy/#device-proxy-device-conf:bind
// bind denotes where the port is open (can be: instance, host)
func SshAgent(name string, from, to string) workshop.Device {
	return workshop.Device{
		Name:       name,
		Type:       workshop.SshAgentProxy,
		Properties: map[string]string{"type": "proxy", "connect": "unix:" + from, "listen": "unix:" + to, "uid": "1000", "gid": "1000", "bind": "instance"},
	}
}

func installSshAgent(fs workshop.WorkshopFs, dev workshop.Device, workshop string) error {
	env, err := fs.Create(filepath.Join("/etc/profile.d", dev.Name+".sh"))
	if err != nil {
		return fmt.Errorf("cannot set SSH_AUTH_SOCK for %q: %w", workshop, err)
	}

	_, err = env.Write([]byte("export SSH_AUTH_SOCK=" + strings.TrimPrefix(dev.Properties["listen"], "unix:")))
	if err != nil {
		return fmt.Errorf("cannot set SSH_AUTH_SOCK for %q: %w", workshop, err)
	}
	_ = env.Close()
	return nil
}

func removeSshAgent(fs workshop.WorkshopFs, name string) {
	_ = fs.Remove(filepath.Join("/etc/profile.d", name+".sh"))
}
