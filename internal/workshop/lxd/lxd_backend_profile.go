package lxdbackend

import (
	"encoding/json"
	"fmt"
	"strings"

	lxd "github.com/canonical/lxd/client"
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
		return workshop.SdkProfile{}, err
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
			pr.Agent = &workshop.SshAgent{Name: name, Connect: dev["connect"], Listen: dev["listen"]}
		case "none":
			cfg, exist := config[DeviceConfigKey(profile, name)]
			if !exist {
				continue
			}

			devtype, exist := config[DeviceTypeConfigKey(profile, name)]
			if exist && devtype == "mount" {
				var mnt workshop.Mount
				if err := json.Unmarshal([]byte(cfg), &mnt); err != nil {
					return pr, err
				}
				pr.Mounts[name] = mnt
				continue
			}

			logger.Noticef("On reading %q SDK profile: unknown device: %s", profile, name)
		default:
			logger.Noticef("On reading %q SDK profile: unknown device type: %s", profile, dev["type"])
		}
	}
	return pr, nil
}
