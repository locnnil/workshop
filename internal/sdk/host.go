package sdk

import (
	"fmt"
)

var hostSdkYaml = `name: host
base: %s
type: host
slots:
  content:
    interface: content
  gpu:
    interface: gpu
  ssh-agent:
    interface: ssh-agent
`

func HostSdkMeta(base string) string {
	return fmt.Sprintf(hostSdkYaml, base)
}
