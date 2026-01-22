package hcore

import (
	"github.com/reddts/edgegate-core/v2/service_manager"
	"github.com/sagernet/sing-box/adapter"
)

type coreMainServiceManager struct{}

var _ adapter.Service = (*coreMainServiceManager)(nil)

func (h *coreMainServiceManager) Start() error {
	return service_manager.OnMainServiceStart()
}

func (h *coreMainServiceManager) Close() error {
	return service_manager.OnMainServiceClose()
}
