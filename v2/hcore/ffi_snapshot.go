package hcore

import (
	"encoding/json"
	"sync"
	"time"
)

var (
	ffiStatusLock           sync.Mutex
	ffiStatusPrev           *SystemInfo
	ffiStatusJSON           string
	ffiStatusUpdaterOnce    sync.Once
	ffiOutboundsLock        sync.Mutex
	ffiOutboundsPrev        *OutboundGroupList
	ffiOutboundsJSON        string
	ffiMainOutboundsLock    sync.Mutex
	ffiMainOutboundsPrev    *OutboundGroupList
	ffiMainOutboundsJSON    string
	ffiOutboundsRefreshLock sync.Mutex
	ffiOutboundsMetaLock    sync.Mutex
	ffiOutboundsLastAccess  time.Time
	ffiOutboundsLastRefresh time.Time
	ffiOutboundsDirty       = true
	ffiOutboundsUpdaterOnce sync.Once
)

const (
	ffiStatusRefreshInterval    = 250 * time.Millisecond
	ffiOutboundsRefreshInterval = 500 * time.Millisecond
	ffiOutboundsWarmInterval    = 2 * time.Second
	ffiOutboundsIdleInterval    = 5 * time.Second
	ffiOutboundsWarmAfter       = 5 * time.Second
	ffiOutboundsIdleAfter       = 30 * time.Second
	ffiOutboundsHotFallback     = 5 * time.Second
	ffiOutboundsWarmFallback    = 15 * time.Second
	ffiOutboundsIdleFallback    = 30 * time.Second
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

// SnapshotSystemInfoJSON returns a cached JSON envelope for FFI callers.
func SnapshotSystemInfoJSON() string {
	ensureFFIStatusUpdater()
	ffiStatusLock.Lock()
	defer ffiStatusLock.Unlock()
	if ffiStatusJSON == "" {
		if ffiStatusPrev == nil {
			return marshalFFIEnvelope(false, "core service is not started", nil)
		}
		return marshalFFIEnvelope(true, "", ffiStatusPrev)
	}
	return ffiStatusJSON
}

// SnapshotOutbounds returns outbound groups; onlyMain mirrors MainOutboundsInfo behavior.
func SnapshotOutbounds(onlyMain bool) *OutboundGroupList {
	markFFIOutboundsAccess()
	ensureFFIOutboundsUpdater()
	if shouldRefreshFFIOutboundsOnAccess() {
		refreshAllFFIOutbounds()
	}
	if onlyMain {
		if cached := cloneCachedOutboundGroupList(&ffiMainOutboundsLock, ffiMainOutboundsPrev); cached != nil {
			return cached
		}
		refreshAllFFIOutbounds()
		return cloneCachedOutboundGroupList(&ffiMainOutboundsLock, ffiMainOutboundsPrev)
	}
	if cached := cloneCachedOutboundGroupList(&ffiOutboundsLock, ffiOutboundsPrev); cached != nil {
		return cached
	}
	refreshAllFFIOutbounds()
	return cloneCachedOutboundGroupList(&ffiOutboundsLock, ffiOutboundsPrev)
}

// SnapshotOutboundsJSON returns a cached JSON envelope for FFI callers.
func SnapshotOutboundsJSON(onlyMain bool) string {
	markFFIOutboundsAccess()
	ensureFFIOutboundsUpdater()
	if shouldRefreshFFIOutboundsOnAccess() {
		refreshAllFFIOutbounds()
	}
	if onlyMain {
		ffiMainOutboundsLock.Lock()
		defer ffiMainOutboundsLock.Unlock()
		if ffiMainOutboundsJSON == "" {
			return marshalFFIEnvelope(false, "core service is not started", nil)
		}
		return ffiMainOutboundsJSON
	}
	ffiOutboundsLock.Lock()
	defer ffiOutboundsLock.Unlock()
	if ffiOutboundsJSON == "" {
		return marshalFFIEnvelope(false, "core service is not started", nil)
	}
	return ffiOutboundsJSON
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
			for {
				timer := time.NewTimer(nextFFIOutboundsRefreshInterval())
				<-timer.C
				refreshAllFFIOutboundsIfNeeded()
			}
		}()
	})
}

func refreshAllFFIOutbounds() {
	ffiOutboundsRefreshLock.Lock()
	defer ffiOutboundsRefreshLock.Unlock()
	refreshAllFFIOutboundsLocked()
}

func refreshAllFFIOutboundsIfNeeded() {
	ffiOutboundsRefreshLock.Lock()
	defer ffiOutboundsRefreshLock.Unlock()

	if !shouldRefreshFFIOutboundsInBackground() {
		return
	}
	refreshAllFFIOutboundsLocked()
}

func refreshAllFFIOutboundsLocked() {
	if static.Box == nil {
		setFFIOutboundsCache(
			&ffiOutboundsLock,
			&ffiOutboundsPrev,
			&ffiOutboundsJSON,
			nil,
		)
		setFFIOutboundsCache(
			&ffiMainOutboundsLock,
			&ffiMainOutboundsPrev,
			&ffiMainOutboundsJSON,
			nil,
		)
		recordFFIOutboundsRefresh()
		return
	}

	allOutbounds, mainOutbounds := GetAllProxiesSnapshots()
	setFFIOutboundsCache(
		&ffiOutboundsLock,
		&ffiOutboundsPrev,
		&ffiOutboundsJSON,
		allOutbounds,
	)
	setFFIOutboundsCache(
		&ffiMainOutboundsLock,
		&ffiMainOutboundsPrev,
		&ffiMainOutboundsJSON,
		mainOutbounds,
	)
	recordFFIOutboundsRefresh()
}

func setFFIOutboundsCache(
	lock *sync.Mutex,
	cached **OutboundGroupList,
	cachedJSON *string,
	next *OutboundGroupList,
) {
	lock.Lock()
	defer lock.Unlock()
	*cached = next
	if next == nil {
		*cachedJSON = marshalFFIEnvelope(false, "core service is not started", nil)
		return
	}
	*cachedJSON = marshalFFIEnvelope(true, "", next)
}

func markFFIOutboundsAccess() {
	ffiOutboundsMetaLock.Lock()
	defer ffiOutboundsMetaLock.Unlock()
	ffiOutboundsLastAccess = time.Now()
}

func markFFIOutboundsDirty() {
	ffiOutboundsMetaLock.Lock()
	defer ffiOutboundsMetaLock.Unlock()
	ffiOutboundsDirty = true
}

func recordFFIOutboundsRefresh() {
	ffiOutboundsMetaLock.Lock()
	defer ffiOutboundsMetaLock.Unlock()
	ffiOutboundsLastRefresh = time.Now()
	ffiOutboundsDirty = false
}

func nextFFIOutboundsRefreshInterval() time.Duration {
	ffiOutboundsMetaLock.Lock()
	defer ffiOutboundsMetaLock.Unlock()

	if ffiOutboundsLastAccess.IsZero() {
		return ffiOutboundsIdleInterval
	}
	idleFor := time.Since(ffiOutboundsLastAccess)
	switch {
	case idleFor >= ffiOutboundsIdleAfter:
		return ffiOutboundsIdleInterval
	case idleFor >= ffiOutboundsWarmAfter:
		return ffiOutboundsWarmInterval
	default:
		return ffiOutboundsRefreshInterval
	}
}

func shouldRefreshFFIOutboundsOnAccess() bool {
	ffiOutboundsMetaLock.Lock()
	defer ffiOutboundsMetaLock.Unlock()
	return ffiOutboundsDirty || ffiOutboundsLastRefresh.IsZero()
}

func shouldRefreshFFIOutboundsInBackground() bool {
	ffiOutboundsMetaLock.Lock()
	defer ffiOutboundsMetaLock.Unlock()
	if ffiOutboundsDirty || ffiOutboundsLastRefresh.IsZero() {
		return true
	}
	return time.Since(ffiOutboundsLastRefresh) >= nextFFIOutboundsFallbackIntervalLocked()
}

func nextFFIOutboundsFallbackIntervalLocked() time.Duration {
	if ffiOutboundsLastAccess.IsZero() {
		return ffiOutboundsIdleFallback
	}
	idleFor := time.Since(ffiOutboundsLastAccess)
	switch {
	case idleFor >= ffiOutboundsIdleAfter:
		return ffiOutboundsIdleFallback
	case idleFor >= ffiOutboundsWarmAfter:
		return ffiOutboundsWarmFallback
	default:
		return ffiOutboundsHotFallback
	}
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
		ffiStatusJSON = marshalFFIEnvelope(false, "core service is not started", nil)
		return
	}
	ffiStatusPrev = readStatus(ffiStatusPrev)
	ffiStatusJSON = marshalFFIEnvelope(true, "", ffiStatusPrev)
}

func refreshFFISnapshotCaches() {
	refreshFFIStatus()
	markFFIOutboundsDirty()
	refreshAllFFIOutbounds()
}

func marshalFFIEnvelope(ok bool, errMsg string, data any) string {
	result := map[string]any{
		"ok": ok,
	}
	if errMsg != "" {
		result["error"] = errMsg
	}
	if data != nil {
		result["data"] = data
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return `{"ok":false,"error":"marshal result failed"}`
	}
	return string(raw)
}
