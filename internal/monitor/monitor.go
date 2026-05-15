package monitor

import (
	"context"
	"crypto/tls"
	"fmt"
	"go-upkeep/internal/alert"
	"go-upkeep/internal/models"
	"go-upkeep/internal/store"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	probing "github.com/prometheus-community/pro-bing"
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

	tokenIndex map[string]int

	db                 store.Store
	insecureSkipVerify bool
	strictClient       *http.Client
	insecureClient     *http.Client
}

func NewEngine(s store.Store) *Engine {
	return &Engine{
		liveState:  make(map[int]models.Site),
		histories:  make(map[int]*SiteHistory),
		tokenIndex: make(map[string]int),
		isActive:   true,
		db:         s,
		strictClient: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: false}},
		},
		insecureClient: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
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
		select {
		case <-time.After(time.Duration(interval) * time.Second):
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
	case "http":
		e.checkHTTP(site)
	case "push":
		e.checkPush(site)
	case "ping":
		e.checkPing(site)
	case "port":
		e.checkPort(site)
	case "dns":
		e.checkDNS(site)
	case "group":
		e.checkGroup(site)
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

func (e *Engine) checkHTTP(site models.Site) {
	method := site.Method
	if method == "" {
		method = "GET"
	}

	timeout := siteTimeout(site)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, site.URL, nil)
	if err != nil {
		e.handleStatusChange(site, "DOWN", 0, 0)
		return
	}

	client := e.strictClient
	if e.insecureSkipVerify || site.IgnoreTLS {
		client = e.insecureClient
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	rawStatus := "UP"
	rawCode := 0
	var certExpiry time.Time
	hasSSL := false

	if err != nil {
		rawStatus = "DOWN"
	} else {
		defer resp.Body.Close()
		rawCode = resp.StatusCode
		if !isCodeAccepted(rawCode, site.AcceptedCodes) {
			rawStatus = "DOWN"
		}
		if site.CheckSSL && resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
			hasSSL = true
			cert := resp.TLS.PeerCertificates[0]
			certExpiry = cert.NotAfter
			if time.Now().After(cert.NotAfter) {
				rawStatus = "SSL EXP"
			}
		}
	}
	updatedSite := site
	updatedSite.HasSSL = hasSSL
	updatedSite.CertExpiry = certExpiry
	updatedSite.Latency = latency
	updatedSite.LastCheck = time.Now()
	e.handleStatusChange(updatedSite, rawStatus, rawCode, latency)
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

	if site.Type == "http" && site.CheckSSL && site.HasSSL {
		daysLeft := int(time.Until(site.CertExpiry).Hours() / 24)
		if daysLeft <= site.ExpiryThreshold && !site.SentSSLWarning && rawStatus != "SSL EXP" {
			e.triggerAlert(site.AlertID, "SSL WARNING", fmt.Sprintf("SSL for '%s' expires in %d days", site.Name, daysLeft))
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
		msg := fmt.Sprintf("Monitor '%s' is DOWN (%s)", site.Name, rawStatus)
		if site.Type == "push" {
			msg = fmt.Sprintf("Push Monitor '%s' missed heartbeat.", site.Name)
		}
		e.triggerAlert(site.AlertID, "🚨 ALERT", msg)
	}
	if isBroken(site.Status) && newState.Status == "UP" {
		e.triggerAlert(site.AlertID, "✅ RECOVERY", fmt.Sprintf("Monitor '%s' is UP", site.Name))
	}
}

func (e *Engine) triggerAlert(alertID int, title, message string) {
	cfg, err := e.db.GetAlert(alertID)
	if err != nil {
		return
	}
	provider := alert.GetProvider(cfg)
	if provider != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = ctx
			_ = provider.Send(title, message)
		}()
	}
}

func siteTimeout(site models.Site) time.Duration {
	if site.Timeout > 0 {
		return time.Duration(site.Timeout) * time.Second
	}
	return 5 * time.Second
}

func isCodeAccepted(code int, accepted string) bool {
	if accepted == "" {
		return code >= 200 && code < 300
	}
	for _, part := range strings.Split(accepted, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
			hi, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err1 == nil && err2 == nil && code >= lo && code <= hi {
				return true
			}
		} else {
			if v, err := strconv.Atoi(part); err == nil && code == v {
				return true
			}
		}
	}
	return false
}

func (e *Engine) checkPing(site models.Site) {
	host := site.Hostname
	if host == "" {
		host = site.URL
	}

	pinger, err := probing.NewPinger(host)
	if err != nil {
		e.handleStatusChange(site, "DOWN", 0, 0)
		e.AddLog(fmt.Sprintf("Ping '%s' resolve failed: %v", site.Name, err))
		return
	}
	pinger.Count = 1
	pinger.Timeout = siteTimeout(site)
	pinger.SetPrivileged(false)

	start := time.Now()
	err = pinger.Run()
	latency := time.Since(start)

	if err != nil || pinger.Statistics().PacketsRecv == 0 {
		updatedSite := site
		updatedSite.Latency = latency
		updatedSite.LastCheck = time.Now()
		e.handleStatusChange(updatedSite, "DOWN", 0, latency)
		return
	}

	stats := pinger.Statistics()
	updatedSite := site
	updatedSite.Latency = stats.AvgRtt
	updatedSite.LastCheck = time.Now()
	e.handleStatusChange(updatedSite, "UP", 0, stats.AvgRtt)
}

func (e *Engine) checkPort(site models.Site) {
	host := site.Hostname
	if host == "" {
		host = site.URL
	}
	addr := net.JoinHostPort(host, strconv.Itoa(site.Port))
	timeout := siteTimeout(site)

	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	latency := time.Since(start)

	updatedSite := site
	updatedSite.Latency = latency
	updatedSite.LastCheck = time.Now()

	if err != nil {
		e.handleStatusChange(updatedSite, "DOWN", 0, latency)
		return
	}
	conn.Close()
	e.handleStatusChange(updatedSite, "UP", 0, latency)
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
		if child.Paused {
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

func (e *Engine) checkDNS(site models.Site) {
	host := site.Hostname
	if host == "" {
		host = site.URL
	}

	server := site.DNSServer
	if server == "" {
		server = "1.1.1.1"
	}
	if _, _, err := net.SplitHostPort(server); err != nil {
		server = net.JoinHostPort(server, "53")
	}

	qtype := dns.TypeA
	switch site.DNSResolveType {
	case "AAAA":
		qtype = dns.TypeAAAA
	case "MX":
		qtype = dns.TypeMX
	case "CNAME":
		qtype = dns.TypeCNAME
	case "TXT":
		qtype = dns.TypeTXT
	case "NS":
		qtype = dns.TypeNS
	case "SOA":
		qtype = dns.TypeSOA
	case "SRV":
		qtype = dns.TypeSRV
	case "PTR":
		qtype = dns.TypePTR
	}

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(host), qtype)

	c := new(dns.Client)
	c.Timeout = siteTimeout(site)

	start := time.Now()
	r, _, err := c.Exchange(m, server)
	latency := time.Since(start)

	updatedSite := site
	updatedSite.Latency = latency
	updatedSite.LastCheck = time.Now()

	if err != nil {
		e.handleStatusChange(updatedSite, "DOWN", 0, latency)
		return
	}

	if r.Rcode != dns.RcodeSuccess {
		e.handleStatusChange(updatedSite, "DOWN", r.Rcode, latency)
		return
	}

	e.handleStatusChange(updatedSite, "UP", 0, latency)
}
