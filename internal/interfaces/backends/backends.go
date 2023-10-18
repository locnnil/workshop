package backend

import (
	"github.com/canonical/workspace/internal/interfaces"
	"github.com/canonical/workspace/internal/interfaces/device"
)

func All() []interfaces.SecurityBackend {
	all := []interfaces.SecurityBackend{
		&device.Backend{},
	}
	return all
}
