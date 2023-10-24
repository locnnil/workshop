package backend

import (
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/device"
)

func All() []interfaces.SecurityBackend {
	all := []interfaces.SecurityBackend{
		&device.Backend{},
	}
	return all
}
