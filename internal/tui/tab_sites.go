package tui

import (
	"fmt"
	"go-upkeep/internal/models"
	"net/url"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

var sparkChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

func typeIcon(siteType string, collapsed bool) string {
	switch siteType {
	case "http":
		return "→"
	case "push":
		return "↓"
	case "ping":
		return "↔"
	case "port":
		return "⊡"
	case "dns":
		return "◆"
	case "group":
		if collapsed {
			return ""
		}
		return ""
	default:
		return "·"
	}
}

var siteGroupStyle = lipgloss.NewStyle().
	Padding(0, 1).
	Bold(true).
	Foreground(lipgloss.Color("#7D56F4"))

type siteFormData struct {
	Name          string
	SiteType      string
	URL           string
	Method        string
	AcceptedCodes string
	Interval      string
	AlertID       string
	CheckSSL      bool
	Threshold     string
	Retries       string
	Hostname      string
	Port          string
	Timeout       string
	Description   string
	IgnoreTLS     bool
	GroupID       string
	Regions       string
}

func latencySparkline(latencies []time.Duration, width int) string {
	if len(latencies) == 0 {
		return subtleStyle.Render(strings.Repeat("·", width))
	}

	samples := latencies
	if len(samples) > width {
		samples = samples[len(samples)-width:]
	}

	minL, maxL := samples[0], samples[0]
	for _, l := range samples {
		if l < minL {
			minL = l
		}
		if l > maxL {
			maxL = l
		}
	}

	var sb strings.Builder
	if remaining := width - len(samples); remaining > 0 {
		sb.WriteString(subtleStyle.Render(strings.Repeat("·", remaining)))
	}
	spread := maxL - minL
	for _, l := range samples {
		idx := 0
		if spread > 0 {
			idx = int(float64(l-minL) / float64(spread) * 7)
			if idx > 7 {
				idx = 7
			}
		}
		ch := string(sparkChars[idx])
		ms := l.Milliseconds()
		if ms < 200 {
			sb.WriteString(specialStyle.Render(ch))
		} else if ms < 500 {
			sb.WriteString(warnStyle.Render(ch))
		} else {
			sb.WriteString(dangerStyle.Render(ch))
		}
	}
	return sb.String()
}

func heartbeatSparkline(statuses []bool, width int) string {
	if len(statuses) == 0 {
		return subtleStyle.Render(strings.Repeat("·", width))
	}

	samples := statuses
	if len(samples) > width {
		samples = samples[len(samples)-width:]
	}

	var sb strings.Builder
	if remaining := width - len(samples); remaining > 0 {
		sb.WriteString(subtleStyle.Render(strings.Repeat("·", remaining)))
	}
	for _, up := range samples {
		if up {
			sb.WriteString(specialStyle.Render("▁"))
		} else {
			sb.WriteString(dangerStyle.Render("█"))
		}
	}
	return sb.String()
}

func (m Model) groupSparkline(groupID int, width int) string {
	allSites := m.engine.GetAllSites()
	var childStatuses [][]bool
	for _, s := range allSites {
		if s.ParentID == groupID && !s.Paused && !m.isMonitorInMaintenance(s.ID) {
			hist, _ := m.engine.GetHistory(s.ID)
			if len(hist.Statuses) > 0 {
				childStatuses = append(childStatuses, hist.Statuses)
			}
		}
	}

	if len(childStatuses) == 0 {
		return subtleStyle.Render(strings.Repeat("·", width))
	}

	maxLen := 0
	for _, s := range childStatuses {
		if len(s) > maxLen {
			maxLen = len(s)
		}
	}
	if maxLen > width {
		maxLen = width
	}

	aggregated := make([]bool, maxLen)
	for i := 0; i < maxLen; i++ {
		allUp := true
		for _, statuses := range childStatuses {
			idx := len(statuses) - maxLen + i
			if idx >= 0 && !statuses[idx] {
				allUp = false
				break
			}
		}
		aggregated[i] = allUp
	}

	var sb strings.Builder
	if remaining := width - len(aggregated); remaining > 0 {
		sb.WriteString(subtleStyle.Render(strings.Repeat("·", remaining)))
	}
	for _, up := range aggregated {
		if up {
			sb.WriteString(specialStyle.Render("●"))
		} else {
			sb.WriteString(dangerStyle.Render("●"))
		}
	}
	return sb.String()
}

func (m Model) groupUptime(groupID int) string {
	allSites := m.engine.GetAllSites()
	var allStatuses [][]bool
	for _, s := range allSites {
		if s.ParentID == groupID && !s.Paused && !m.isMonitorInMaintenance(s.ID) {
			hist, _ := m.engine.GetHistory(s.ID)
			if len(hist.Statuses) > 0 {
				allStatuses = append(allStatuses, hist.Statuses)
			}
		}
	}
	if len(allStatuses) == 0 {
		return subtleStyle.Render("—")
	}
	total, up := 0, 0
	for _, statuses := range allStatuses {
		for _, s := range statuses {
			total++
			if s {
				up++
			}
		}
	}
	return fmtUptime(func() []bool {
		out := make([]bool, total)
		idx := 0
		for _, statuses := range allStatuses {
			copy(out[idx:], statuses)
			idx += len(statuses)
		}
		return out
	}())
}

func fmtLatency(d time.Duration) string {
	ms := d.Milliseconds()
	if ms == 0 {
		return subtleStyle.Render("—")
	}
	var s string
	if ms < 1000 {
		s = fmt.Sprintf("%dms", ms)
	} else {
		s = fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	if ms < 200 {
		return specialStyle.Render(s)
	}
	if ms < 500 {
		return warnStyle.Render(s)
	}
	return dangerStyle.Render(s)
}

func fmtUptime(statuses []bool) string {
	if len(statuses) == 0 {
		return subtleStyle.Render("—")
	}
	up := 0
	for _, s := range statuses {
		if s {
			up++
		}
	}
	pct := float64(up) / float64(len(statuses)) * 100
	s := fmt.Sprintf("%.1f%%", pct)
	if pct >= 99 {
		return specialStyle.Render(s)
	}
	if pct >= 95 {
		return warnStyle.Render(s)
	}
	return dangerStyle.Render(s)
}

func fmtSSL(site models.Site) string {
	if site.Type != "http" || !site.CheckSSL || !site.HasSSL {
		return subtleStyle.Render("-")
	}
	days := int(time.Until(site.CertExpiry).Hours() / 24)
	s := fmt.Sprintf("%dd", days)
	if days <= 0 {
		return dangerStyle.Render("EXPIRED")
	}
	if days <= site.ExpiryThreshold {
		return warnStyle.Render(s)
	}
	return specialStyle.Render(s)
}

func fmtRetries(site models.Site) string {
	retriesDone := site.FailureCount - 1
	if retriesDone < 0 {
		retriesDone = 0
	}
	dispCount := retriesDone
	if dispCount > site.MaxRetries {
		dispCount = site.MaxRetries
	}
	s := fmt.Sprintf("%d/%d", dispCount, site.MaxRetries)
	if site.Status == "DOWN" {
		return dangerStyle.Render(s)
	}
	if site.Status == "UP" && site.FailureCount > 0 {
		return warnStyle.Render(s)
	}
	return s
}

func fmtStatus(status string, paused bool, inMaint bool) string {
	if paused {
		return warnStyle.Render("PAUSED")
	}
	if inMaint {
		return maintStyle.Render("MAINT")
	}
	switch {
	case status == "DOWN" || status == "SSL EXP":
		return dangerStyle.Render(status)
	case status == "PENDING":
		return subtleStyle.Render(status)
	default:
		return specialStyle.Render(status)
	}
}

func (m Model) dynamicWidths() (nameW, sparkW int) {
	fixed := 6 + 10 + 10 + 8 + 8 + 7 + 9 // #, TYPE, STATUS, LATENCY, UPTIME, SSL, RETRY
	overhead := 30                       // cell padding + borders
	avail := m.termWidth - chromePadH - 2 - fixed - overhead
	if avail < 30 {
		avail = 30
	}
	nameW = avail / 2
	sparkW = avail - nameW - 2 // -2 for spark column padding
	if nameW < 13 {
		nameW = 13
	}
	if nameW > 40 {
		nameW = 40
	}
	if sparkW < 10 {
		sparkW = 10
	}
	if sparkW > 60 {
		sparkW = 60
	}
	return
}

func (m Model) viewSitesTab() string {

	if len(m.sites) == 0 {
		welcome := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(1, 3).
			Render(
				titleStyle.Render("Go-Upkeep") + "\n\n" +
					"No monitors configured yet.\n\n" +
					subtleStyle.Render("[n] Add your first monitor"),
			)
		return "\n" + welcome
	}

	nameW, sparkWidth := m.dynamicWidths()
	colWidths := []int{6, 0, 10, 10, 8, 8, sparkWidth + 2, 7, 9}

	var groupRows map[int]bool
	return m.renderTable(
		[]string{"#", "NAME", "TYPE", "STATUS", "LATENCY", "UPTIME", "HISTORY", "SSL", "RETRY"},
		len(m.sites),
		func(start, end int) [][]string {
			groupRows = make(map[int]bool)
			var rows [][]string
			for i := start; i < end; i++ {
				site := m.sites[i]

				if site.Type == "group" {
					groupRows[i-start] = true
					icon := typeIcon("group", m.collapsed[site.ID])
					rows = append(rows, []string{
						strconv.Itoa(i + 1),
						m.zones.Mark(fmt.Sprintf("site-%d", i), icon+" "+limitStr(site.Name, nameW-2)),
						"group",
						fmtStatus(site.Status, site.Paused, m.isMonitorInMaintenance(site.ID)),
						subtleStyle.Render("—"),
						m.groupUptime(site.ID),
						m.groupSparkline(site.ID, sparkWidth),
						subtleStyle.Render("-"),
						subtleStyle.Render("—"),
					})
					continue
				}

				name := site.Name
				if site.ParentID > 0 {
					prefix := "├"
					if i+1 >= len(m.sites) || m.sites[i+1].ParentID != site.ParentID {
						prefix = "└"
					}
					name = prefix + " " + limitStr(name, nameW-2)
				} else {
					name = limitStr(name, nameW)
				}

				hist, _ := m.engine.GetHistory(site.ID)
				var spark string
				if site.Type == "push" {
					spark = heartbeatSparkline(hist.Statuses, sparkWidth)
				} else {
					spark = latencySparkline(hist.Latencies, sparkWidth)
				}

				rows = append(rows, []string{
					strconv.Itoa(i + 1),
					m.zones.Mark(fmt.Sprintf("site-%d", i), name),
					typeIcon(site.Type, false) + " " + site.Type,
					fmtStatus(site.Status, site.Paused, m.isMonitorInMaintenance(site.ID)),
					fmtLatency(site.Latency),
					fmtUptime(hist.Statuses),
					spark,
					fmtSSL(site),
					fmtRetries(site),
				})
			}
			return rows
		},
		colWidths,
		func(row, col int) *lipgloss.Style {
			if groupRows[row] {
				s := siteGroupStyle
				return &s
			}
			return nil
		},
	)
}

func (m *Model) initSiteHuhForm() tea.Cmd {
	m.siteFormData = &siteFormData{
		SiteType:      "http",
		Method:        "GET",
		AcceptedCodes: "200-299",
		Interval:      "60",
		Threshold:     "7",
		Retries:       "0",
		Timeout:       "5",
		Port:          "0",
		GroupID:       "0",
	}

	if m.editID > 0 {
		for _, site := range m.sites {
			if site.ID == m.editID {
				m.siteFormData.Name = site.Name
				m.siteFormData.SiteType = site.Type
				m.siteFormData.URL = site.URL
				m.siteFormData.Interval = strconv.Itoa(site.Interval)
				m.siteFormData.AlertID = strconv.Itoa(site.AlertID)
				m.siteFormData.CheckSSL = site.CheckSSL
				m.siteFormData.Threshold = strconv.Itoa(site.ExpiryThreshold)
				m.siteFormData.Retries = strconv.Itoa(site.MaxRetries)
				m.siteFormData.Hostname = site.Hostname
				m.siteFormData.Port = strconv.Itoa(site.Port)
				m.siteFormData.Timeout = strconv.Itoa(site.Timeout)
				m.siteFormData.Description = site.Description
				m.siteFormData.IgnoreTLS = site.IgnoreTLS
				m.siteFormData.GroupID = strconv.Itoa(site.ParentID)
				m.siteFormData.Method = site.Method
				m.siteFormData.AcceptedCodes = site.AcceptedCodes
				m.siteFormData.Regions = site.Regions
				break
			}
		}
	}

	alertOpts := []huh.Option[string]{huh.NewOption("None", "0")}
	if alerts, err := m.store.GetAllAlerts(); err == nil {
		for _, a := range alerts {
			alertOpts = append(alertOpts, huh.NewOption(
				fmt.Sprintf("%s (%s)", a.Name, a.Type),
				strconv.Itoa(a.ID),
			))
		}
	}

	groupOpts := []huh.Option[string]{huh.NewOption("None", "0")}
	for _, s := range m.sites {
		if s.Type == "group" && s.ID != m.editID {
			groupOpts = append(groupOpts, huh.NewOption(s.Name, strconv.Itoa(s.ID)))
		}
	}

	m.huhForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Monitor Name").
				Placeholder("My Service").
				Value(&m.siteFormData.Name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					return nil
				}),
			huh.NewSelect[string]().Title("Monitor Type").
				Options(
					huh.NewOption("HTTP/HTTPS", "http"),
					huh.NewOption("Push / Heartbeat", "push"),
					huh.NewOption("Ping (ICMP)", "ping"),
					huh.NewOption("TCP Port", "port"),
					huh.NewOption("DNS", "dns"),
					huh.NewOption("Group", "group"),
				).Value(&m.siteFormData.SiteType),
			huh.NewSelect[string]().Title("Alert Channel").
				Options(alertOpts...).
				Value(&m.siteFormData.AlertID),
		).Title("Monitor Settings"),
		huh.NewGroup(
			huh.NewInput().Title("URL").
				Placeholder("https://example.com").
				Description("Required for HTTP monitors").
				Value(&m.siteFormData.URL).
				Validate(func(s string) error {
					if m.siteFormData.SiteType == "push" || m.siteFormData.SiteType == "group" {
						return nil
					}
					if s == "" {
						return fmt.Errorf("URL is required for HTTP monitors")
					}
					u, err := url.Parse(s)
					if err != nil {
						return fmt.Errorf("invalid URL")
					}
					if u.Scheme != "http" && u.Scheme != "https" {
						return fmt.Errorf("URL must start with http:// or https://")
					}
					if u.Host == "" {
						return fmt.Errorf("URL must include a host")
					}
					return nil
				}),
			huh.NewInput().Title("Check Interval (seconds)").
				Placeholder("60").
				Value(&m.siteFormData.Interval).
				Validate(func(s string) error {
					if m.siteFormData.SiteType == "group" {
						return nil
					}
					v, err := strconv.Atoi(s)
					if err != nil {
						return fmt.Errorf("must be a number")
					}
					if v < 5 {
						return fmt.Errorf("minimum interval is 5 seconds")
					}
					return nil
				}),
			huh.NewSelect[string]().Title("Parent Group").
				Options(groupOpts...).
				Value(&m.siteFormData.GroupID),
			huh.NewInput().Title("Hostname / IP").
				Placeholder("10.0.0.1").
				Description("Target for ping/port/DNS monitors").
				Value(&m.siteFormData.Hostname),
			huh.NewInput().Title("Port").
				Placeholder("0").
				Description("Target port for TCP port monitors").
				Value(&m.siteFormData.Port).
				Validate(func(s string) error {
					v, err := strconv.Atoi(s)
					if err != nil {
						return fmt.Errorf("must be a number")
					}
					if v < 0 || v > 65535 {
						return fmt.Errorf("port must be 0-65535")
					}
					return nil
				}),
			huh.NewInput().Title("Timeout (seconds)").
				Placeholder("5").
				Value(&m.siteFormData.Timeout).
				Validate(func(s string) error {
					if m.siteFormData.SiteType == "group" {
						return nil
					}
					v, err := strconv.Atoi(s)
					if err != nil {
						return fmt.Errorf("must be a number")
					}
					if v < 1 || v > 300 {
						return fmt.Errorf("timeout must be 1-300 seconds")
					}
					return nil
				}),
			huh.NewInput().Title("Description").
				Placeholder("Optional description").
				Value(&m.siteFormData.Description),
			huh.NewInput().Title("Probe Regions").
				Placeholder("us-east, eu-west (empty = all)").
				Description("Comma-separated regions for distributed probing").
				Value(&m.siteFormData.Regions),
		).Title("Connection").WithHideFunc(func() bool {
			return m.siteFormData.SiteType == "group"
		}),
		huh.NewGroup(
			huh.NewSelect[string]().Title("HTTP Method").
				Options(
					huh.NewOption("GET", "GET"),
					huh.NewOption("POST", "POST"),
					huh.NewOption("PUT", "PUT"),
					huh.NewOption("PATCH", "PATCH"),
					huh.NewOption("DELETE", "DELETE"),
					huh.NewOption("HEAD", "HEAD"),
					huh.NewOption("OPTIONS", "OPTIONS"),
				).Value(&m.siteFormData.Method),
			huh.NewInput().Title("Accepted Status Codes").
				Placeholder("200-299").
				Description("Ranges (200-299) and singles (301) separated by commas").
				Value(&m.siteFormData.AcceptedCodes),
		).Title("HTTP Settings").WithHideFunc(func() bool {
			return m.siteFormData.SiteType != "http"
		}),
		huh.NewGroup(
			huh.NewConfirm().Title("Monitor SSL Certificate?").
				Value(&m.siteFormData.CheckSSL),
			huh.NewInput().Title("SSL Warning Threshold (days)").
				Placeholder("7").
				Value(&m.siteFormData.Threshold).
				Validate(func(s string) error {
					v, err := strconv.Atoi(s)
					if err != nil {
						return fmt.Errorf("must be a number")
					}
					if v < 1 {
						return fmt.Errorf("threshold must be at least 1 day")
					}
					return nil
				}),
			huh.NewInput().Title("Max Retries Before Alert").
				Placeholder("0").
				Value(&m.siteFormData.Retries).
				Validate(func(s string) error {
					v, err := strconv.Atoi(s)
					if err != nil {
						return fmt.Errorf("must be a number")
					}
					if v < 0 {
						return fmt.Errorf("retries cannot be negative")
					}
					return nil
				}),
			huh.NewConfirm().Title("Ignore TLS Errors?").
				Value(&m.siteFormData.IgnoreTLS),
		).Title("Advanced").WithHideFunc(func() bool {
			return m.siteFormData.SiteType == "group"
		}),
	).WithTheme(huh.ThemeDracula())

	return m.huhForm.Init()
}

func (m *Model) submitSiteForm() {
	d := m.siteFormData
	interval, _ := strconv.Atoi(d.Interval)
	alertID, _ := strconv.Atoi(d.AlertID)
	threshold, _ := strconv.Atoi(d.Threshold)
	retries, _ := strconv.Atoi(d.Retries)
	port, _ := strconv.Atoi(d.Port)
	timeout, _ := strconv.Atoi(d.Timeout)
	groupID, _ := strconv.Atoi(d.GroupID)
	if interval < 1 {
		interval = 60
	}
	if threshold < 1 {
		threshold = 7
	}

	site := models.Site{
		ID:              m.editID,
		Name:            d.Name,
		URL:             d.URL,
		Type:            d.SiteType,
		Interval:        interval,
		AlertID:         alertID,
		CheckSSL:        d.CheckSSL,
		ExpiryThreshold: threshold,
		MaxRetries:      retries,
		Hostname:        d.Hostname,
		Port:            port,
		Timeout:         timeout,
		Description:     d.Description,
		IgnoreTLS:       d.IgnoreTLS,
		ParentID:        groupID,
		Method:          d.Method,
		AcceptedCodes:   d.AcceptedCodes,
		Regions:         d.Regions,
	}

	if m.editID > 0 {
		if err := m.store.UpdateSite(site); err != nil {
			m.engine.AddLog("Update site failed: " + err.Error())
		}
		m.engine.UpdateSiteConfig(site)
	} else {
		if err := m.store.AddSite(site); err != nil {
			m.engine.AddLog("Add site failed: " + err.Error())
		}
	}
	m.state = stateDashboard
}

func (m Model) viewDetailPanel() string {
	if m.cursor >= len(m.sites) {
		return ""
	}
	site := m.sites[m.cursor]
	hist, _ := m.engine.GetHistory(site.ID)

	var b strings.Builder

	var breadcrumb string
	if site.ParentID > 0 {
		for _, s := range m.sites {
			if s.ID == site.ParentID {
				breadcrumb = subtleStyle.Render("  Sites > "+s.Name+" > ") + titleStyle.Render(site.Name)
				break
			}
		}
	}
	if breadcrumb == "" {
		breadcrumb = subtleStyle.Render("  Sites > ") + titleStyle.Render(site.Name)
	}
	b.WriteString(breadcrumb + "\n\n")

	row := func(label, value string) {
		b.WriteString(fmt.Sprintf("  %-16s %s\n", subtleStyle.Render(label), value))
	}

	row("Status", fmtStatus(site.Status, site.Paused, m.isMonitorInMaintenance(site.ID)))
	if m.isMonitorInMaintenance(site.ID) {
		for _, mw := range m.maintenanceWindows {
			if mw.Type == "maintenance" && (mw.MonitorID == 0 || mw.MonitorID == site.ID || mw.MonitorID == site.ParentID) {
				row("Maintenance", maintStyle.Render(mw.Title))
				break
			}
		}
	}
	row("Type", site.Type)
	if site.URL != "" {
		row("URL", site.URL)
	}
	if site.Hostname != "" {
		row("Host", site.Hostname)
	}
	if site.Port > 0 {
		row("Port", strconv.Itoa(site.Port))
	}
	row("Interval", fmt.Sprintf("%ds", site.Interval))
	row("Timeout", fmt.Sprintf("%ds", site.Timeout))
	row("Latency", fmtLatency(site.Latency))
	row("Uptime", fmtUptime(hist.Statuses))

	if site.Type == "http" {
		row("Method", site.Method)
		row("Codes", site.AcceptedCodes)
		row("SSL", fmtSSL(site))
		if site.IgnoreTLS {
			row("TLS Verify", dangerStyle.Render("disabled"))
		}
	}

	if site.MaxRetries > 0 {
		row("Retries", fmtRetries(site))
	}
	if site.Regions != "" {
		row("Regions", site.Regions)
	}
	if site.Description != "" {
		row("Description", site.Description)
	}
	if !site.LastCheck.IsZero() {
		row("Last Check", site.LastCheck.Format("15:04:05"))
	}

	probeResults := m.engine.GetProbeResults(site.ID)
	if len(probeResults) > 0 {
		b.WriteString("\n" + subtleStyle.Render("  PROBE RESULTS") + "\n")
		for nodeID, result := range probeResults {
			status := specialStyle.Render("UP")
			if !result.IsUp {
				status = dangerStyle.Render("DN")
			}
			latency := time.Duration(result.LatencyNs).Milliseconds()
			ago := time.Since(result.CheckedAt).Truncate(time.Second)
			b.WriteString(fmt.Sprintf("  %-14s %s  %dms  %s ago\n", nodeID, status, latency, ago))
		}
	}

	b.WriteString("\n")
	const sparkWidth = 40
	if site.Type == "push" {
		b.WriteString("  " + heartbeatSparkline(hist.Statuses, sparkWidth))
		if len(hist.Statuses) > 0 {
			up := 0
			for _, s := range hist.Statuses {
				if s {
					up++
				}
			}
			b.WriteString(fmt.Sprintf("\n  %s %d/%d checks up",
				subtleStyle.Render("Heartbeats"),
				up, len(hist.Statuses)))
		}
	} else {
		b.WriteString("  " + latencySparkline(hist.Latencies, sparkWidth))
		if len(hist.Latencies) > 0 {
			minL, maxL := hist.Latencies[0], hist.Latencies[0]
			var total time.Duration
			for _, l := range hist.Latencies {
				total += l
				if l < minL {
					minL = l
				}
				if l > maxL {
					maxL = l
				}
			}
			avg := total / time.Duration(len(hist.Latencies))
			b.WriteString(fmt.Sprintf("\n  %s %dms  %s %dms  %s %dms",
				subtleStyle.Render("Min"), minL.Milliseconds(),
				subtleStyle.Render("Avg"), avg.Milliseconds(),
				subtleStyle.Render("Max"), maxL.Milliseconds()))
		}
	}

	b.WriteString("\n\n")
	b.WriteString(subtleStyle.Render("  [i/Esc] Back  [e] Edit  [q] Quit"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}
