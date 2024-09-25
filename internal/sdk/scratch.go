package sdk

import (
	"fmt"
)

const Scratch = "scratch"

var scratchSdkYaml = `name: scratch
base: %s
type: regular
`

func ScratchSdkMeta(base string) string {
	return fmt.Sprintf(scratchSdkYaml, base)
}
