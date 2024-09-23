package backend

import (
	"github.com/canonical/workshop/internal/interfaces"
	"github.com/canonical/workshop/internal/interfaces/lxd_device"
)

func All() []interfaces.SecurityBackend {
	all := []interfaces.SecurityBackend{
		&lxd_device.Backend{},
	}
	return all
}
