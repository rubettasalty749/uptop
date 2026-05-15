package monitor

import (
	"go-upkeep/internal/store"
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

func InitHistoryFromStore() {
	s := store.Get()
	if s == nil {
		return
	}
	all := s.LoadAllHistory(maxHistoryLen)
	historyMu.Lock()
	defer historyMu.Unlock()
	for siteID, records := range all {
		h := &SiteHistory{}
		for _, r := range records {
			h.TotalChecks++
			if r.IsUp {
				h.UpChecks++
			}
			h.Latencies = append(h.Latencies, time.Duration(r.LatencyNs))
			h.Statuses = append(h.Statuses, r.IsUp)
		}
		histories[siteID] = h
	}
	if len(all) > 0 {
		AddLog("Loaded check history from database")
	}
}

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

	if s := store.Get(); s != nil {
		go s.SaveCheck(siteID, latency.Nanoseconds(), isUp)
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
