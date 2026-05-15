package tui

import (
	"fmt"
	"go-upkeep/internal/models"
	"go-upkeep/internal/monitor"
	"go-upkeep/internal/store"
	"net/url"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var sparkChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

var (
	siteHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true).
			Padding(0, 1)

	siteCellStyle = lipgloss.NewStyle().Padding(0, 1)

	siteSelectedStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Bold(true).
				Foreground(lipgloss.Color("#ffffff")).
				Background(lipgloss.Color("#3b3b5c"))

	siteBorderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444"))

	siteGroupStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4"))
)

type siteFormData struct {
	Name        string
	SiteType    string
	URL         string
	Interval    string
	AlertID     string
	CheckSSL    bool
	Threshold   string
	Retries     string
	Hostname    string
	Port        string
	Timeout     string
	Description string
	IgnoreTLS   bool
	GroupID     string
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

	if remaining := width - len(samples); remaining > 0 {
		sb.WriteString(subtleStyle.Render(strings.Repeat("·", remaining)))
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
	for _, up := range samples {
		if up {
			sb.WriteString(specialStyle.Render("▁"))
		} else {
			sb.WriteString(dangerStyle.Render("█"))
		}
	}

	if remaining := width - len(samples); remaining > 0 {
		sb.WriteString(subtleStyle.Render(strings.Repeat("·", remaining)))
	}
	return sb.String()
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

func fmtUptime(total, up int) string {
	if total == 0 {
		return subtleStyle.Render("—")
	}
	pct := float64(up) / float64(total) * 100
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

func fmtStatus(status string, paused bool) string {
	if paused {
		return warnStyle.Render("PAUSED")
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

func (m Model) viewSitesTab() string {
	const sparkWidth = 20

	if len(m.sites) == 0 {
		return "\n  No sites configured. Press [n] to add one."
	}

	end := m.tableOffset + m.maxTableRows
	if end > len(m.sites) {
		end = len(m.sites)
	}

	selectedVisual := m.cursor - m.tableOffset

	var rows [][]string
	var groupRows []int
	for i := m.tableOffset; i < end; i++ {
		site := m.sites[i]

		if site.Type == "group" {
			groupRows = append(groupRows, i-m.tableOffset)
			arrow := "▾"
			if m.collapsed[site.ID] {
				arrow = "▸"
			}
			rows = append(rows, []string{
				strconv.Itoa(i + 1),
				m.zones.Mark(fmt.Sprintf("site-%d", i), arrow+" "+limitStr(site.Name, 11)),
				"group",
				fmtStatus(site.Status, site.Paused),
				subtleStyle.Render("—"),
				subtleStyle.Render("—"),
				subtleStyle.Render(strings.Repeat("·", sparkWidth)),
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
			name = prefix + " " + limitStr(name, 11)
		} else {
			name = limitStr(name, 13)
		}

		hist, _ := monitor.GetHistory(site.ID)
		var spark string
		if site.Type == "push" {
			spark = heartbeatSparkline(hist.Statuses, sparkWidth)
		} else {
			spark = latencySparkline(hist.Latencies, sparkWidth)
		}

		rows = append(rows, []string{
			strconv.Itoa(i + 1),
			m.zones.Mark(fmt.Sprintf("site-%d", i), name),
			site.Type,
			fmtStatus(site.Status, site.Paused),
			fmtLatency(site.Latency),
			fmtUptime(hist.TotalChecks, hist.UpChecks),
			spark,
			fmtSSL(site),
			fmtRetries(site),
		})
	}

	isGroupRow := func(row int) bool {
		for _, g := range groupRows {
			if g == row {
				return true
			}
		}
		return false
	}

	tableWidth := m.termWidth - 6
	if tableWidth < 40 {
		tableWidth = 40
	}

	// column widths: #=6, name=flex, type=10, status=10, latency=8, uptime=8, history=sparkWidth+4, ssl=7, retry=9
	colWidths := []int{6, 0, 10, 10, 8, 8, sparkWidth + 4, 7, 9}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(siteBorderStyle).
		Width(tableWidth).
		Headers("#", "NAME", "TYPE", "STATUS", "LATENCY", "UPTIME", "HISTORY", "SSL", "RETRY").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			var base lipgloss.Style
			if row == table.HeaderRow {
				base = siteHeaderStyle
			} else if row == selectedVisual {
				base = siteSelectedStyle
			} else if isGroupRow(row) {
				base = siteGroupStyle
			} else {
				base = siteCellStyle
			}
			if col < len(colWidths) && colWidths[col] > 0 {
				base = base.Width(colWidths[col])
			}
			return base
		})

	return "\n" + t.Render()
}

func (m *Model) initSiteHuhForm() tea.Cmd {
	m.siteFormData = &siteFormData{
		SiteType:  "http",
		Interval:  "60",
		Threshold: "7",
		Retries:   "0",
		Timeout:   "5",
		Port:      "0",
		GroupID:   "0",
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
				break
			}
		}
	}

	alertOpts := []huh.Option[string]{huh.NewOption("None", "0")}
	if store.Get() != nil {
		for _, a := range store.Get().GetAllAlerts() {
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
		).Title("Connection").WithHideFunc(func() bool {
			return m.siteFormData.SiteType == "group"
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
	}

	if m.editID > 0 {
		store.Get().UpdateSite(site)
		monitor.UpdateSiteConfig(site)
	} else {
		store.Get().AddSite(site)
	}
	m.state = stateDashboard
}
