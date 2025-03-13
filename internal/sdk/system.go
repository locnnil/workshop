package sdk

import (
	"fmt"
)

var systemSdkYaml = `name: system
base: %s
type: system
slots:
  camera:
    interface: camera
  mount:
    interface: mount
  gpu:
    interface: gpu
  ssh-agent:
    interface: ssh-agent
  desktop:
    interface: desktop
`

func SystemSdkMeta(base string) string {
	return fmt.Sprintf(systemSdkYaml, base)
}

func IsSystem(name string) bool {
	return name == System.String()
}
