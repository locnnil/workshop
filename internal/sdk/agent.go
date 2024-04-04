package sdk

import "fmt"

var agentSdkYamlTemplate = `name: agent
base: %s
type: agent
slots:
  content:
    interface: content
  gpu:
    interface: gpu
`

func AgentSdkMeta(base string) string {
	return fmt.Sprintf(agentSdkYamlTemplate, base)
}
