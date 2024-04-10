package workshopbackend


func Mount(name, source, target string) Device {
	return Device{name: name,
		deviceType: BindMount,
		properties: map[string]string{"type": "disk", "source": source,
			"path": target},
	}
}

func Volume(name, mountTo, volume string) Device {
	return Device{
		name:       name,
		deviceType: DiskVolume,
		properties: map[string]string{"type": "disk",
			"pool":   "default",
			"path":   mountTo,
			"source": volume},
	}
}

func Gpu(name string) Device {
	return Device{
		name:       name,
		deviceType: GPU,

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
		properties: map[string]string{"type": "gpu", "gputype": "physical", "uid": "1000", "gid": "1000"},
	}
}

// A network protocol proxy device, opens a port on the host or in a workhop.
// from, to are the source and destination addresses (paths in the case of unix sockets),
// see https://documentation.ubuntu.com/lxd/en/latest/reference/devices_proxy/#device-proxy-device-conf:bind
// bind denotes where the port is open (can be: instance, host)
func SshAgent(name string, from, to string) Device {
	return Device{
		name:       name,
		deviceType: SshAgentProxy,
		properties: map[string]string{"type": "proxy", "connect": "unix:" + from, "listen": "unix:" + to, "uid": "1000", "gid": "1000", "bind": "instance"},
	}
}

func (w Device) lxdProperties() map[string]string {
	return w.properties
}
