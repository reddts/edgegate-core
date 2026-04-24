package hcore

import (
	"sync"

	"github.com/reddts/edgegate-core/v2/config"
	"github.com/sagernet/sing-box/experimental/libbox"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common/observable"
)

type CoreInstance struct {
	Box         *BoxService
	CoreOptions *config.CoreOptions
	// activeConfigPath string
	CoreLogFactory            log.Factory
	coreInfoObserver          *observable.Observer[*CoreInfoResponse]
	CoreState                 CoreStates
	logObserver               *observable.Observer[*LogMessage]
	systemInfoObserver        *observable.Observer[*SystemInfo]
	outboundsInfoObserver     *observable.Observer[*OutboundGroupList]
	mainOutboundsInfoObserver *observable.Observer[*OutboundGroupList]
	lock                      sync.Mutex
	lastStartRequestLock      sync.RWMutex
	lastStartRequestName      string
	globalPlatformInterface   libbox.PlatformInterface
	previousStartRequest      *StartRequest
}

var static = &CoreInstance{
	coreInfoObserver:          NewObserver[*CoreInfoResponse](1),
	CoreState:                 CoreStates_STOPPED,
	logObserver:               NewObserver[*LogMessage](1),
	systemInfoObserver:        NewObserver[*SystemInfo](1),
	outboundsInfoObserver:     NewObserver[*OutboundGroupList](1),
	mainOutboundsInfoObserver: NewObserver[*OutboundGroupList](1),
}

func (c *CoreInstance) SetLastStartRequestName(name string) {
	c.lastStartRequestLock.Lock()
	defer c.lastStartRequestLock.Unlock()
	c.lastStartRequestName = name
}

func (c *CoreInstance) LastStartRequestName() string {
	c.lastStartRequestLock.RLock()
	defer c.lastStartRequestLock.RUnlock()
	return c.lastStartRequestName
}
