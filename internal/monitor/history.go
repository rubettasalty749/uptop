package monitor

import (
	"sync"
	"time"
)

const maxHistoryLen = 30

type SiteHistory struct {
	Latencies   []time.Duration
	Statuses    []bool
	TotalChecks int
	UpChecks    int
}

var (
	histories = make(map[int]*SiteHistory)
	historyMu sync.RWMutex
)

func RecordCheck(siteID int, latency time.Duration, isUp bool) {
	historyMu.Lock()
	defer historyMu.Unlock()

	h, ok := histories[siteID]
	if !ok {
		h = &SiteHistory{}
		histories[siteID] = h
	}

	h.TotalChecks++
	if isUp {
		h.UpChecks++
	}

	h.Latencies = append(h.Latencies, latency)
	if len(h.Latencies) > maxHistoryLen {
		h.Latencies = h.Latencies[len(h.Latencies)-maxHistoryLen:]
	}

	h.Statuses = append(h.Statuses, isUp)
	if len(h.Statuses) > maxHistoryLen {
		h.Statuses = h.Statuses[len(h.Statuses)-maxHistoryLen:]
	}
}

func GetHistory(siteID int) (SiteHistory, bool) {
	historyMu.RLock()
	defer historyMu.RUnlock()
	h, ok := histories[siteID]
	if !ok {
		return SiteHistory{}, false
	}
	cp := SiteHistory{
		TotalChecks: h.TotalChecks,
		UpChecks:    h.UpChecks,
		Latencies:   make([]time.Duration, len(h.Latencies)),
		Statuses:    make([]bool, len(h.Statuses)),
	}
	copy(cp.Latencies, h.Latencies)
	copy(cp.Statuses, h.Statuses)
	return cp, true
}

func RemoveHistory(siteID int) {
	historyMu.Lock()
	defer historyMu.Unlock()
	delete(histories, siteID)
}
