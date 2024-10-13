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
`

func SystemSdkMeta(base string) string {
	return fmt.Sprintf(systemSdkYaml, base)
}
