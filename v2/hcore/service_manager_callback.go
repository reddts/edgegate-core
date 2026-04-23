package hcore

import (
	"github.com/reddts/edgegate-core/v2/service_manager"
	"github.com/sagernet/sing-box/adapter"
)

type coreMainServiceManager struct{}

var _ adapter.Service = (*coreMainServiceManager)(nil)

func (h *coreMainServiceManager) Start(_ adapter.StartStage) error {
	return service_manager.OnMainServiceStart()
}

func (h *coreMainServiceManager) Close() error {
	return service_manager.OnMainServiceClose()
}

func (h *coreMainServiceManager) Type() string {
	return "edgegate_core_service"
}

func (h *coreMainServiceManager) Tag() string {
	return "core_main_service_manager"
}
