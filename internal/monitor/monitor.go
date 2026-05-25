package monitor

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"gitea.lerkolabs.com/lerko/uptop/internal/alert"
	"gitea.lerkolabs.com/lerko/uptop/internal/models"
	"gitea.lerkolabs.com/lerko/uptop/internal/store"
)

type Engine struct {
	mu        sync.RWMutex
	liveState map[int]models.Site

	logMu    sync.RWMutex
	logStore []string

	activeMu sync.RWMutex
	isActive bool

	histMu    sync.RWMutex
	histories map[int]*SiteHistory

	tokenIndex map[string]int // protected by mu

	probeResultsMu sync.RWMutex
	probeResults   map[int]map[string]NodeResult
	aggStrategy    AggregationStrategy

	db                  store.Store
	insecureSkipVerify  bool
	allowPrivateTargets bool
	strictClient        *http.Client
	insecureClient      *http.Client
}

func NewEngine(s store.Store) *Engine {
	return newEngine(s, false)
}

func NewEngineWithOpts(s store.Store, allowPrivateTargets bool) *Engine {
	return newEngine(s, allowPrivateTargets)
}

func newEngine(s store.Store, allowPrivateTargets bool) *Engine {
	dial := SafeDialContext(allowPrivateTargets)
	return &Engine{
		liveState:           make(map[int]models.Site),
		histories:           make(map[int]*SiteHistory),
		tokenIndex:          make(map[string]int),
		probeResults:        make(map[int]map[string]NodeResult),
		aggStrategy:         AggAnyDown,
		isActive:            true,
		allowPrivateTargets: allowPrivateTargets,
		db:                  s,
		strictClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
				DialContext:     dial,
			},
		},
		insecureClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // intentional for IgnoreTLS sites
				DialContext:     dial,
			},
		},
	}
}

func (e *Engine) SetInsecureSkipVerify(skip bool) {
	e.insecureSkipVerify = skip
}

func (e *Engine) AddLog(msg string) {
	e.logMu.Lock()
	defer e.logMu.Unlock()
	ts := time.Now().Format("15:04:05")
	entry := fmt.Sprintf("[%s] %s", ts, msg)
	e.logStore = append([]string{entry}, e.logStore...)
	if len(e.logStore) > 100 {
		e.logStore = e.logStore[:100]
	}
	go func() { _ = e.db.SaveLog(entry) }()
}

func (e *Engine) InitLogs() {
	logs, err := e.db.LoadLogs(100)
	if err != nil {
		return
	}
	if len(logs) == 0 {
		return
	}
	e.logMu.Lock()
	defer e.logMu.Unlock()
	e.logStore = logs
}

func (e *Engine) GetLogs() []string {
	e.logMu.RLock()
	defer e.logMu.RUnlock()
	logs := make([]string, len(e.logStore))
	copy(logs, e.logStore)
	return logs
}

func (e *Engine) SetActive(active bool) {
	e.activeMu.Lock()
	defer e.activeMu.Unlock()
	if e.isActive != active {
		e.isActive = active
		status := "RESUMED (Active)"
		if !active {
			status = "PAUSED (Passive)"
		}
		e.AddLog(fmt.Sprintf("Engine %s", status))
	}
}

func (e *Engine) IsActive() bool {
	e.activeMu.RLock()
	defer e.activeMu.RUnlock()
	return e.isActive
}

func (e *Engine) GetAllSites() []models.Site {
	e.mu.RLock()
	defer e.mu.RUnlock()
	sites := make([]models.Site, 0, len(e.liveState))
	for _, s := range e.liveState {
		sites = append(sites, s)
	}
	return sites
}

func (e *Engine) GetLiveState() map[int]models.Site {
	e.mu.RLock()
	defer e.mu.RUnlock()
	cp := make(map[int]models.Site, len(e.liveState))
	for k, v := range e.liveState {
		cp[k] = v
	}
	return cp
}

func (e *Engine) RecordHeartbeat(token string) bool {
	if !e.IsActive() {
		return false
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	targetID, ok := e.tokenIndex[token]
	if !ok {
		return false
	}

	site, exists := e.liveState[targetID]
	if !exists {
		return false
	}

	site.LastCheck = time.Now()
	wasDown := site.Status == "DOWN"
	site.Status = "UP"
	site.FailureCount = 0
	site.Latency = 0
	e.liveState[targetID] = site

	if wasDown {
		e.AddLog(fmt.Sprintf("Push Monitor '%s' recovered", site.Name))
		e.triggerAlert(site.AlertID, "✅ RECOVERY", fmt.Sprintf("Push Monitor '%s' is receiving heartbeats.", site.Name))
	}
	return true
}

func (e *Engine) addToTokenIndex(site models.Site) {
	if site.Type == "push" && site.Token != "" {
		e.tokenIndex[site.Token] = site.ID
	}
}

func (e *Engine) removeFromTokenIndex(id int) {
	for token, sid := range e.tokenIndex {
		if sid == id {
			delete(e.tokenIndex, token)
			return
		}
	}
}

func (e *Engine) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			sites, err := e.db.GetSites()
			if err != nil {
				e.AddLog(fmt.Sprintf("Failed to load sites: %v", err))
				select {
				case <-time.After(5 * time.Second):
				case <-ctx.Done():
					return
				}
				continue
			}
			for _, s := range sites {
				e.mu.RLock()
				_, exists := e.liveState[s.ID]
				e.mu.RUnlock()
				if !exists {
					e.mu.Lock()
					s.Status = "PENDING"
					if s.Type == "push" {
						s.LastCheck = time.Now()
					}
					if h, ok := e.GetHistory(s.ID); ok && len(h.Statuses) > 0 {
						if h.Statuses[len(h.Statuses)-1] {
							s.Status = "UP"
						} else {
							s.Status = "DOWN"
						}
						if len(h.Latencies) > 0 {
							s.Latency = h.Latencies[len(h.Latencies)-1]
						}
					}
					e.liveState[s.ID] = s
					e.addToTokenIndex(s)
					e.mu.Unlock()
					go e.monitorRoutine(ctx, s.ID)
				}
			}

			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (e *Engine) UpdateSiteConfig(site models.Site) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if existing, ok := e.liveState[site.ID]; ok {
		e.removeFromTokenIndex(site.ID)
		site.Status = existing.Status
		site.StatusCode = existing.StatusCode
		site.Latency = existing.Latency
		site.CertExpiry = existing.CertExpiry
		site.HasSSL = existing.HasSSL
		site.LastCheck = existing.LastCheck
		site.SentSSLWarning = existing.SentSSLWarning
		site.FailureCount = existing.FailureCount
		e.liveState[site.ID] = site
		e.addToTokenIndex(site)
	}
}

func (e *Engine) RemoveSite(id int) {
	e.mu.Lock()
	e.removeFromTokenIndex(id)
	delete(e.liveState, id)
	e.mu.Unlock()
	e.removeHistory(id)
}

func (e *Engine) ToggleSitePause(id int) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	site, ok := e.liveState[id]
	if !ok {
		return false
	}
	site.Paused = !site.Paused
	e.liveState[id] = site
	if site.Paused {
		e.AddLog(fmt.Sprintf("Monitor '%s' paused", site.Name))
	} else {
		e.AddLog(fmt.Sprintf("Monitor '%s' resumed", site.Name))
	}
	return site.Paused
}

func (e *Engine) monitorRoutine(ctx context.Context, id int) {
	// Stagger initial check to avoid thundering herd on startup
	stagger := time.Duration(rand.IntN(3000)) * time.Millisecond //nolint:gosec // non-security jitter
	select {
	case <-time.After(stagger):
	case <-ctx.Done():
		return
	}

	e.checkByID(id)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !e.IsActive() {
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}
			continue
		}

		e.mu.RLock()
		site, exists := e.liveState[id]
		e.mu.RUnlock()
		if !exists {
			return
		}

		if site.Paused {
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}
			continue
		}

		interval := site.Interval
		if interval < 5 {
			interval = 5
		}
		jitter := time.Duration(rand.IntN(interval*100)) * time.Millisecond //nolint:gosec // non-security jitter
		select {
		case <-time.After(time.Duration(interval)*time.Second + jitter):
		case <-ctx.Done():
			return
		}
		e.checkByID(id)
	}
}

func (e *Engine) checkByID(id int) {
	if !e.IsActive() {
		return
	}

	e.mu.RLock()
	site, exists := e.liveState[id]
	e.mu.RUnlock()
	if !exists || site.Paused {
		return
	}

	switch site.Type {
	case "push":
		e.checkPush(site)
	case "group":
		e.checkGroup(site)
	default:
		result := RunCheck(site, e.strictClient, e.insecureClient, e.insecureSkipVerify, e.allowPrivateTargets)
		updatedSite := site
		updatedSite.HasSSL = result.HasSSL
		updatedSite.CertExpiry = result.CertExpiry
		updatedSite.Latency = time.Duration(result.LatencyNs)
		updatedSite.LastCheck = time.Now()
		e.handleStatusChange(updatedSite, result.Status, result.StatusCode, time.Duration(result.LatencyNs))
	}
}

func (e *Engine) checkPush(site models.Site) {
	deadline := site.LastCheck.Add(time.Duration(site.Interval) * time.Second).Add(5 * time.Second)
	if time.Now().After(deadline) {
		e.handleStatusChange(site, "DOWN", 0, 0)
	} else if site.Status != "UP" {
		e.handleStatusChange(site, "UP", 200, 0)
	}
}

func (e *Engine) handleStatusChange(site models.Site, rawStatus string, code int, latency time.Duration) {
	if !e.IsActive() {
		return
	}

	newState := site
	newState.StatusCode = code

	if site.Status == "UP" && rawStatus != "UP" {
		newState.FailureCount++
		if newState.FailureCount > site.MaxRetries {
			newState.Status = rawStatus
			newState.FailureCount = site.MaxRetries + 1
			e.AddLog(fmt.Sprintf("Monitor '%s' confirmed DOWN", site.Name))
		} else {
			e.AddLog(fmt.Sprintf("Monitor '%s' failed check %d/%d", site.Name, newState.FailureCount, site.MaxRetries))
		}
	} else if rawStatus == "UP" {
		newState.FailureCount = 0
		newState.Status = "UP"
	} else {
		newState.Status = rawStatus
		newState.FailureCount = site.MaxRetries + 1
	}

	inMaint := e.isInMaintenance(site.ID)

	if site.Type == "http" && site.CheckSSL && site.HasSSL {
		daysLeft := int(time.Until(site.CertExpiry).Hours() / 24)
		if daysLeft <= site.ExpiryThreshold && !site.SentSSLWarning && rawStatus != "SSL EXP" {
			if !inMaint {
				e.triggerAlert(site.AlertID, "SSL WARNING", fmt.Sprintf("SSL for '%s' expires in %d days", site.Name, daysLeft))
			} else {
				e.AddLog(fmt.Sprintf("SSL warning for '%s' suppressed (maintenance)", site.Name))
			}
			newState.SentSSLWarning = true
		} else if daysLeft > site.ExpiryThreshold {
			newState.SentSSLWarning = false
		}
	}

	e.mu.Lock()
	if _, ok := e.liveState[site.ID]; ok {
		e.liveState[site.ID] = newState
	}
	e.mu.Unlock()

	e.recordCheck(site.ID, latency, rawStatus == "UP")

	isBroken := func(s string) bool { return s == "DOWN" || s == "SSL EXP" }
	if !isBroken(site.Status) && isBroken(newState.Status) && newState.Status != "PENDING" {
		if inMaint {
			e.AddLog(fmt.Sprintf("Monitor '%s' is DOWN (alerts suppressed — maintenance)", site.Name))
		} else {
			msg := fmt.Sprintf("Monitor '%s' is DOWN (%s)", site.Name, rawStatus)
			if site.Type == "push" {
				msg = fmt.Sprintf("Push Monitor '%s' missed heartbeat.", site.Name)
			}
			e.triggerAlert(site.AlertID, "🚨 ALERT", msg)
		}
	}
	if isBroken(site.Status) && newState.Status == "UP" {
		if !inMaint {
			e.triggerAlert(site.AlertID, "✅ RECOVERY", fmt.Sprintf("Monitor '%s' is UP", site.Name))
		} else {
			e.AddLog(fmt.Sprintf("Monitor '%s' recovered (maintenance active, alert suppressed)", site.Name))
		}
	}
}

func (e *Engine) triggerAlert(alertID int, title, message string) {
	cfg, err := e.db.GetAlert(alertID)
	if err != nil {
		e.AddLog(fmt.Sprintf("Failed to load alert config %d: %v", alertID, err))
		return
	}
	provider := alert.GetProvider(cfg)
	if provider != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := provider.Send(ctx, title, message); err != nil {
				e.AddLog(fmt.Sprintf("Alert send failed (%s): %v", cfg.Name, err))
			}
		}()
	}
}

func (e *Engine) isInMaintenance(monitorID int) bool {
	inMaint, err := e.db.IsMonitorInMaintenance(monitorID)
	if err != nil {
		return false
	}
	return inMaint
}

func (e *Engine) GetDisplayStatus(site models.Site) string {
	if site.Paused {
		return "PAUSED"
	}
	if e.isInMaintenance(site.ID) {
		return "MAINT"
	}
	return site.Status
}

func (e *Engine) checkGroup(site models.Site) {
	e.mu.RLock()
	status := "UP"
	hasChildren := false
	allPaused := true
	for _, child := range e.liveState {
		if child.ParentID != site.ID || child.Type == "group" {
			continue
		}
		hasChildren = true
		if !child.Paused {
			allPaused = false
		}
		if child.Paused || e.isInMaintenance(child.ID) {
			continue
		}
		if child.Status == "DOWN" || child.Status == "SSL EXP" {
			status = "DOWN"
		} else if child.Status == "PENDING" && status != "DOWN" {
			status = "PENDING"
		}
	}
	e.mu.RUnlock()

	if !hasChildren {
		status = "PENDING"
	}

	e.mu.Lock()
	s := e.liveState[site.ID]
	s.Status = status
	if hasChildren && allPaused {
		s.Paused = true
	}
	e.liveState[site.ID] = s
	e.mu.Unlock()
}

func (e *Engine) SetAggStrategy(strategy AggregationStrategy) {
	e.aggStrategy = strategy
}

func (e *Engine) IngestProbeResult(nodeID string, siteID int, latencyNs int64, isUp bool) {
	e.probeResultsMu.Lock()
	if e.probeResults[siteID] == nil {
		e.probeResults[siteID] = make(map[string]NodeResult)
	}
	e.probeResults[siteID][nodeID] = NodeResult{
		NodeID:    nodeID,
		IsUp:      isUp,
		LatencyNs: latencyNs,
		CheckedAt: time.Now(),
	}
	results := make([]NodeResult, 0, len(e.probeResults[siteID]))
	for _, r := range e.probeResults[siteID] {
		results = append(results, r)
	}
	e.probeResultsMu.Unlock()

	aggUp, avgLatency := AggregateStatus(results, e.aggStrategy)

	e.mu.RLock()
	site, exists := e.liveState[siteID]
	e.mu.RUnlock()
	if !exists {
		return
	}

	rawStatus := "UP"
	if !aggUp {
		rawStatus = "DOWN"
	}

	updatedSite := site
	updatedSite.Latency = time.Duration(avgLatency)
	updatedSite.LastCheck = time.Now()
	e.handleStatusChange(updatedSite, rawStatus, 0, time.Duration(avgLatency))
}

func (e *Engine) GetProbeResults(siteID int) map[string]NodeResult {
	e.probeResultsMu.RLock()
	defer e.probeResultsMu.RUnlock()
	src := e.probeResults[siteID]
	cp := make(map[string]NodeResult, len(src))
	for k, v := range src {
		cp[k] = v
	}
	return cp
}
