package lxdbackend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"

	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/workshop"
)

func ProfileName(pid, workshop, sdk string) string {
	return strings.Join([]string{InstanceName(workshop, pid), sdk}, "-")
}

func DeviceConfigKey(sdk, dev string) string {
	return fmt.Sprintf("user.workshop.%s.%s", sdk, dev)
}

func DeviceTypeConfigKey(sdk, dev string) string {
	return fmt.Sprintf("user.workshop.%s.%s.type", sdk, dev)
}

func Profile(conn lxd.InstanceServer, pid, wp, profile string) (workshop.SdkProfile, error) {
	name := ProfileName(pid, wp, profile)
	lxdp, _, err := conn.GetProfile(name)
	if err != nil {
		if api.StatusErrorCheck(err, http.StatusNotFound) {
			return workshop.SdkProfile{}, workshop.ErrSdkProfileNotFound
		}
		return workshop.SdkProfile{}, fmt.Errorf("cannot load %q profile (%w)", profile, err)
	}

	return lxdToSdkProfile(profile, lxdp.Devices, lxdp.Config)
}

func lxdToSdkProfile(profile string, devs map[string]map[string]string, config map[string]string) (workshop.SdkProfile, error) {
	var pr = workshop.NewSdkProfile(profile)
	for name, dev := range devs {
		switch dev["type"] {
		case "disk":
			pr.Mounts[name] = workshop.Mount{Name: name, What: dev["source"], Where: dev["path"], Type: workshop.HostWorkshop}
		case "gpu":
			pr.Gpu = &workshop.Gpu{Name: name}
		case "proxy":
			devtype := config[DeviceTypeConfigKey(profile, name)]
			switch devtype {
			case "ssh-agent":
				proxyEntry, err := proxyEntryFromLxdDevice(name, dev)
				if err != nil {
					return pr, err
				}
				pr.Agent = &workshop.SshAgent{ProxyEntry: *proxyEntry}
			case "desktop-wayland":
				if pr.Desktop == nil {
					pr.Desktop = &workshop.Desktop{}
				}
				proxyEntry, err := proxyEntryFromLxdDevice(name, dev)
				if err != nil {
					return pr, err
				}
				pr.Desktop.Wayland = proxyEntry
			case "desktop-x11":
				if pr.Desktop == nil {
					pr.Desktop = &workshop.Desktop{}
				}
				proxyEntry, err := proxyEntryFromLxdDevice(name, dev)
				if err != nil {
					return pr, err
				}
				pr.Desktop.X11 = proxyEntry
			default:
				logger.Noticef("On reading %q SDK profile: unknown device type: %q", profile, devtype)
			}
		case "unix-char":
			devtype := config[DeviceTypeConfigKey(profile, name)]
			if devtype == "camera" {
				continue
			}

			logger.Noticef("On reading %q SDK profile: unknown device type %q", profile, devtype)
		case "none":
			cfg, exist := config[DeviceConfigKey(profile, name)]
			if !exist {
				logger.Noticef("On reading %q SDK profile: unknown device %q", profile, name)
				continue
			}

			devtype := config[DeviceTypeConfigKey(profile, name)]
			switch devtype {
			case "camera":
				var camera workshop.Camera
				if err := json.Unmarshal([]byte(cfg), &camera); err != nil {
					return pr, err
				}
				pr.Camera = &camera
			case "mount":
				var mnt workshop.Mount
				if err := json.Unmarshal([]byte(cfg), &mnt); err != nil {
					return pr, err
				}
				pr.Mounts[name] = mnt
			default:
				logger.Noticef("On reading %q SDK profile: unknown device type %q", profile, devtype)
			}
		default:
			logger.Noticef("On reading %q SDK profile: unknown device type %q", profile, dev["type"])
		}
	}
	return pr, nil
}

// Constructs a ProxyEntry from an LXD device entry
func proxyEntryFromLxdDevice(name string, dev map[string]string) (*workshop.ProxyEntry, error) {
	connect := strings.SplitN(dev["connect"], ":", 2)
	listen := strings.SplitN(dev["listen"], ":", 2)
	if len(connect) != 2 {
		return nil, fmt.Errorf("internal error: cannot deserialise proxy device in lxd profile: connect entry %q invalid", connect)
	}
	if len(listen) != 2 {
		return nil, fmt.Errorf("internal error: cannot deserialise proxy device in lxd profile: listen entry %q invalid", listen)
	}

	return &workshop.ProxyEntry{
		Name: name,
		Connect: workshop.ProxyTarget{
			Address:  connect[1],
			Protocol: connect[0],
		},
		Listen: workshop.ProxyTarget{
			Address:  listen[1],
			Protocol: listen[0],
		},
	}, nil
}
