package doctemplates

import "embed"

//go:embed *.rst
var templates embed.FS

func ReadFile(name string) ([]byte, error) {
	return templates.ReadFile(name)
}
