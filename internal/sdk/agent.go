package sdk

import "fmt"

var agentSdkYamlTemplate = `name: agent
base: %s
type: agent
slots:
  content:
    interface: content
`

func AgentSdkMeta(base string) string {
	return fmt.Sprintf(agentSdkYamlTemplate, base)
}
