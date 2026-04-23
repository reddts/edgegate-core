package hcore

import (
	"sync"
	"time"
)

var (
	ffiStatusLock           sync.Mutex
	ffiStatusPrev           *SystemInfo
	ffiStatusUpdaterOnce    sync.Once
	ffiOutboundsLock        sync.Mutex
	ffiOutboundsPrev        *OutboundGroupList
	ffiMainOutboundsLock    sync.Mutex
	ffiMainOutboundsPrev    *OutboundGroupList
	ffiOutboundsUpdaterOnce sync.Once
)

const (
	ffiStatusRefreshInterval    = 250 * time.Millisecond
	ffiOutboundsRefreshInterval = 500 * time.Millisecond
)

// SnapshotCoreInfo returns a lightweight core status snapshot for FFI callers.
func SnapshotCoreInfo() *CoreInfoResponse {
	return &CoreInfoResponse{
		CoreState:   static.CoreState,
		MessageType: MessageType_EMPTY,
		Message:     "",
	}
}

// SnapshotSystemInfo returns a point-in-time system info object.
func SnapshotSystemInfo() *SystemInfo {
	ensureFFIStatusUpdater()
	ffiStatusLock.Lock()
	defer ffiStatusLock.Unlock()
	return cloneSystemInfoSnapshot(ffiStatusPrev)
}

// SnapshotOutbounds returns outbound groups; onlyMain mirrors MainOutboundsInfo behavior.
func SnapshotOutbounds(onlyMain bool) *OutboundGroupList {
	ensureFFIOutboundsUpdater()
	if onlyMain {
		if cached := cloneCachedOutboundGroupList(&ffiMainOutboundsLock, ffiMainOutboundsPrev); cached != nil {
			return cached
		}
		refreshFFIOutbounds(true)
		return cloneCachedOutboundGroupList(&ffiMainOutboundsLock, ffiMainOutboundsPrev)
	}
	if cached := cloneCachedOutboundGroupList(&ffiOutboundsLock, ffiOutboundsPrev); cached != nil {
		return cached
	}
	refreshFFIOutbounds(false)
	return cloneCachedOutboundGroupList(&ffiOutboundsLock, ffiOutboundsPrev)
}

func cloneCachedOutboundGroupList(lock *sync.Mutex, cached *OutboundGroupList) *OutboundGroupList {
	lock.Lock()
	defer lock.Unlock()
	return cloneOutboundGroupList(cached)
}

func ensureFFIOutboundsUpdater() {
	ffiOutboundsUpdaterOnce.Do(func() {
		go func() {
			refreshAllFFIOutbounds()
			ticker := time.NewTicker(ffiOutboundsRefreshInterval)
			defer ticker.Stop()

			for range ticker.C {
				refreshAllFFIOutbounds()
			}
		}()
	})
}

func refreshAllFFIOutbounds() {
	if static.Box == nil {
		setFFIOutboundsCache(&ffiOutboundsLock, &ffiOutboundsPrev, nil)
		setFFIOutboundsCache(&ffiMainOutboundsLock, &ffiMainOutboundsPrev, nil)
		return
	}

	refreshFFIOutbounds(false)
	refreshFFIOutbounds(true)
}

func refreshFFIOutbounds(onlyMain bool) {
	if static.Box == nil {
		if onlyMain {
			setFFIOutboundsCache(&ffiMainOutboundsLock, &ffiMainOutboundsPrev, nil)
			return
		}
		setFFIOutboundsCache(&ffiOutboundsLock, &ffiOutboundsPrev, nil)
		return
	}

	next := GetAllProxiesInfo(onlyMain)
	if onlyMain {
		setFFIOutboundsCache(&ffiMainOutboundsLock, &ffiMainOutboundsPrev, next)
		return
	}
	setFFIOutboundsCache(&ffiOutboundsLock, &ffiOutboundsPrev, next)
}

func setFFIOutboundsCache(
	lock *sync.Mutex,
	cached **OutboundGroupList,
	next *OutboundGroupList,
) {
	lock.Lock()
	defer lock.Unlock()
	*cached = next
}

func cloneSystemInfoSnapshot(in *SystemInfo) *SystemInfo {
	if in == nil {
		return nil
	}
	cloned := *in
	return &cloned
}

func cloneOutboundGroupList(in *OutboundGroupList) *OutboundGroupList {
	if in == nil {
		return nil
	}
	cloned := &OutboundGroupList{
		Items: make([]*OutboundGroup, 0, len(in.Items)),
	}
	for _, group := range in.Items {
		if group == nil {
			continue
		}
		groupClone := *group
		groupClone.Selected = cloneOutboundInfo(group.Selected)
		groupClone.Items = make([]*OutboundInfo, 0, len(group.Items))
		for _, item := range group.Items {
			groupClone.Items = append(groupClone.Items, cloneOutboundInfo(item))
		}
		cloned.Items = append(cloned.Items, &groupClone)
	}
	return cloned
}

func cloneOutboundInfo(in *OutboundInfo) *OutboundInfo {
	if in == nil {
		return nil
	}
	cloned := *in
	return &cloned
}

func ensureFFIStatusUpdater() {
	ffiStatusUpdaterOnce.Do(func() {
		go func() {
			refreshFFIStatus()
			ticker := time.NewTicker(ffiStatusRefreshInterval)
			defer ticker.Stop()

			for range ticker.C {
				refreshFFIStatus()
			}
		}()
	})
}

func refreshFFIStatus() {
	ffiStatusLock.Lock()
	defer ffiStatusLock.Unlock()

	if static.Box == nil {
		ffiStatusPrev = nil
		return
	}
	ffiStatusPrev = readStatus(ffiStatusPrev)
}

func refreshFFISnapshotCaches() {
	refreshFFIStatus()
	refreshAllFFIOutbounds()
}
