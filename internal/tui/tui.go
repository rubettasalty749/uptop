package tui

import (
	"fmt"
	"go-upkeep/internal/models"
	"go-upkeep/internal/monitor"
	"go-upkeep/internal/store"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

var (
	subtleStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#565f89"})
	specialStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"})
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#F0E442", Dark: "#F0E442"})
	dangerStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#F25D94", Dark: "#F25D94"})
	titleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true)

	activeTab   = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(lipgloss.Color("#7D56F4")).Foreground(lipgloss.Color("#7D56F4")).Bold(true).Padding(0, 1)
	inactiveTab = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.AdaptiveColor{Light: "#AAA", Dark: "#555"})
)

var pulseFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const (
	chromePadV   = 2 // outer Padding(1,2): 1 top + 1 bottom
	chromePadH   = 4 // outer Padding(1,2): 2 left + 2 right
	chromeHeader = 1 // tab bar line
	chromeGaps   = 2 // "\n" separators: before content + before footer
	chromeFooter = 2 // footer: "\n" prefix + text line
	chromeTable  = 3 // renderTable "\n" prefix + top border + header + bottom border (lipgloss collapses two into three rendered lines)
	chromeBase   = chromePadV + chromeHeader + chromeGaps + chromeFooter + chromeTable
)

type sessionState int

const (
	stateDashboard sessionState = iota
	stateLogs
	stateUsers
	stateDetail
	stateFormSite
	stateFormAlert
	stateFormUser
	stateConfirmDelete
	stateFormMaint
)

type Model struct {
	state        sessionState
	currentTab   int
	cursor       int
	tableOffset  int
	maxTableRows int
	termWidth    int
	termHeight   int
	editID       int
	editToken    string

	huhForm       *huh.Form
	siteFormData  *siteFormData
	alertFormData *alertFormData
	userFormData  *userFormData
	maintFormData *maintFormData

	logViewport viewport.Model
	isAdmin     bool
	zones       *zone.Manager

	deleteID   int
	deleteName string
	deleteTab  int

	collapsed map[int]bool
	store     store.Store
	engine    *monitor.Engine

	// harmonica animation state
	pulseSpring harmonica.Spring
	pulsePos    float64
	pulseVel    float64
	tickCount   int

	sites              []models.Site
	alerts             []models.AlertConfig
	users              []models.User
	nodes              []models.ProbeNode
	maintenanceWindows []models.MaintenanceWindow

	filterMode bool
	filterText string
}

func InitialModel(isAdmin bool, s store.Store, eng *monitor.Engine) Model {
	vpLogs := viewport.New(100, 20)
	vpLogs.SetContent("Waiting for logs...")
	z := zone.New()
	spring := harmonica.NewSpring(harmonica.FPS(10), 6.0, 0.4)
	return Model{
		state:        stateDashboard,
		logViewport:  vpLogs,
		maxTableRows: 5,
		isAdmin:      isAdmin,
		store:        s,
		engine:       eng,
		zones:        z,
		pulseSpring:  spring,
		collapsed:    make(map[int]bool),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.ClearScreen, tea.Tick(time.Second, func(t time.Time) tea.Msg { return t }))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if m.state == stateConfirmDelete {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "y", "Y":
				switch m.deleteTab {
				case 0:
					if err := m.store.DeleteSite(m.deleteID); err != nil {
						m.engine.AddLog("Delete site failed: " + err.Error())
					}
					m.engine.RemoveSite(m.deleteID)
					m.adjustCursor(len(m.sites) - 1)
				case 1:
					if err := m.store.DeleteAlert(m.deleteID); err != nil {
						m.engine.AddLog("Delete alert failed: " + err.Error())
					}
					m.adjustCursor(len(m.alerts) - 1)
				case 4:
					if err := m.store.DeleteMaintenanceWindow(m.deleteID); err != nil {
						m.engine.AddLog("Delete maintenance window failed: " + err.Error())
					}
					m.adjustCursor(len(m.maintenanceWindows) - 1)
				case 5:
					if err := m.store.DeleteUser(m.deleteID); err != nil {
						m.engine.AddLog("Delete user failed: " + err.Error())
					}
					m.adjustCursor(len(m.users) - 1)
				}
				m.refreshData()
				m.state = stateDashboard
				if m.deleteTab == 5 {
					m.state = stateUsers
				}
			case "n", "N", "esc":
				m.state = stateDashboard
				if m.deleteTab == 5 {
					m.state = stateUsers
				}
			case "ctrl+c":
				return m, tea.Quit
			}
		}
		return m, nil
	}

	// Form state: forward ALL messages to huh (keys, timers, resize, etc.)
	if m.state == stateFormSite || m.state == stateFormAlert || m.state == stateFormUser || m.state == stateFormMaint {
		if wsm, ok := msg.(tea.WindowSizeMsg); ok {
			m.termWidth = wsm.Width
			m.termHeight = wsm.Height
		}
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			if keyMsg.String() == "esc" {
				m.huhForm = nil
				m.state = stateDashboard
				if m.currentTab == 5 {
					m.state = stateUsers
				}
				return m, nil
			}
		}
		if m.huhForm != nil {
			form, formCmd := m.huhForm.Update(msg)
			if f, ok := form.(*huh.Form); ok {
				m.huhForm = f
			}
			if m.huhForm.State == huh.StateCompleted {
				m.submitForm()
				m.refreshData()
				m.huhForm = nil
				return m, nil
			}
			return m, formCmd
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		chrome := chromeBase
		if m.filterMode || m.filterText != "" {
			chrome++
		}
		m.maxTableRows = msg.Height - chrome
		if m.maxTableRows < 1 {
			m.maxTableRows = 1
		}
		m.logViewport.Width = msg.Width - chromePadH
		m.logViewport.Height = msg.Height - (chromePadV + chromeHeader + chromeGaps + chromeFooter)
		return m, tea.ClearScreen

	case time.Time:
		m.refreshData()
		m.tickCount++
		target := math.Sin(float64(m.tickCount)*0.3)*0.5 + 0.5
		m.pulsePos, m.pulseVel = m.pulseSpring.Update(m.pulsePos, m.pulseVel, target)
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg { return t })

	case tea.MouseMsg:
		if m.state == stateDashboard || m.state == stateLogs || m.state == stateUsers {
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				return m.handleClick(msg)
			}
			if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
				if m.state == stateLogs {
					if msg.Button == tea.MouseButtonWheelUp {
						m.logViewport.LineUp(3)
					} else {
						m.logViewport.LineDown(3)
					}
					return m, nil
				}
				listLen := len(m.sites)
				if m.currentTab == 1 {
					listLen = len(m.alerts)
				} else if m.currentTab == 3 {
					listLen = len(m.nodes)
				} else if m.currentTab == 4 {
					listLen = len(m.maintenanceWindows)
				} else if m.currentTab == 5 {
					listLen = len(m.users)
				}
				if msg.Button == tea.MouseButtonWheelUp {
					if m.cursor > 0 {
						m.cursor--
						if m.cursor < m.tableOffset {
							m.tableOffset = m.cursor
						}
					}
				} else {
					if m.cursor < listLen-1 {
						m.cursor++
						if m.cursor >= m.tableOffset+m.maxTableRows {
							m.tableOffset++
						}
					}
				}
				return m, nil
			}
		}

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "ctrl+l" {
			return m, tea.ClearScreen
		}

		if m.filterMode {
			switch msg.String() {
			case "esc":
				m.filterMode = false
				m.filterText = ""
				m.cursor = 0
				m.tableOffset = 0
				m.refreshData()
			case "enter":
				m.filterMode = false
			case "backspace":
				if len(m.filterText) > 0 {
					m.filterText = m.filterText[:len(m.filterText)-1]
					m.cursor = 0
					m.tableOffset = 0
					m.refreshData()
				}
			case "ctrl+c":
				return m, tea.Quit
			default:
				if len(msg.String()) == 1 {
					m.filterText += msg.String()
					m.cursor = 0
					m.tableOffset = 0
					m.refreshData()
				}
			}
			return m, nil
		}

		switch m.state {
		case stateDetail:
			switch msg.String() {
			case "i", "esc":
				m.state = stateDashboard
			case "q":
				return m, tea.Quit
			}
			return m, nil
		case stateDashboard, stateLogs, stateUsers:
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "/":
				if m.currentTab == 0 {
					m.filterMode = true
					return m, nil
				}
			case "tab":
				m.switchTab(m.currentTab + 1)
			case "pgup", "pgdown":
				if m.state == stateLogs {
					m.logViewport, cmd = m.logViewport.Update(msg)
					return m, cmd
				}
			case "up", "k":
				if m.state == stateLogs {
					m.logViewport.LineUp(1)
				} else if m.cursor > 0 {
					m.cursor--
					if m.cursor < m.tableOffset {
						m.tableOffset = m.cursor
					}
				}
			case "down", "j":
				if m.state == stateLogs {
					m.logViewport.LineDown(1)
				} else {
					max := len(m.sites) - 1
					if m.currentTab == 1 {
						max = len(m.alerts) - 1
					}
					if m.currentTab == 3 {
						max = len(m.nodes) - 1
					}
					if m.currentTab == 4 {
						max = len(m.maintenanceWindows) - 1
					}
					if m.currentTab == 5 {
						max = len(m.users) - 1
					}
					if m.cursor < max {
						m.cursor++
						if m.cursor >= m.tableOffset+m.maxTableRows {
							m.tableOffset++
						}
					}
				}
			case "n":
				m.editID = 0
				m.editToken = ""
				if m.currentTab == 0 {
					m.state = stateFormSite
					return m, m.initSiteHuhForm()
				} else if m.currentTab == 1 {
					m.state = stateFormAlert
					return m, m.initAlertHuhForm()
				} else if m.currentTab == 4 {
					m.state = stateFormMaint
					return m, m.initMaintHuhForm()
				} else if m.currentTab == 5 && m.isAdmin {
					m.state = stateFormUser
					return m, m.initUserHuhForm()
				}
			case "e", "enter":
				if m.currentTab == 0 && len(m.sites) > 0 {
					m.editID = m.sites[m.cursor].ID
					m.editToken = m.sites[m.cursor].Token
					m.state = stateFormSite
					return m, m.initSiteHuhForm()
				} else if m.currentTab == 1 && len(m.alerts) > 0 {
					m.editID = m.alerts[m.cursor].ID
					m.state = stateFormAlert
					return m, m.initAlertHuhForm()
				} else if m.currentTab == 5 && m.isAdmin && len(m.users) > 0 {
					m.editID = m.users[m.cursor].ID
					m.state = stateFormUser
					return m, m.initUserHuhForm()
				}
			case " ":
				if m.currentTab == 0 && len(m.sites) > 0 && m.sites[m.cursor].Type == "group" {
					gid := m.sites[m.cursor].ID
					m.collapsed[gid] = !m.collapsed[gid]
					m.refreshData()
				}
			case "p":
				if m.currentTab == 0 && len(m.sites) > 0 {
					site := m.sites[m.cursor]
					m.engine.ToggleSitePause(site.ID)
					site.Paused = !site.Paused
					_ = m.store.UpdateSitePaused(site.ID, site.Paused)
					m.refreshData()
				}
			case "i":
				if m.currentTab == 0 && len(m.sites) > 0 {
					m.state = stateDetail
				}
			case "x":
				if m.currentTab == 4 && len(m.maintenanceWindows) > 0 {
					mw := m.maintenanceWindows[m.cursor]
					now := time.Now()
					isActive := !mw.StartTime.After(now) && (mw.EndTime.IsZero() || mw.EndTime.After(now))
					if isActive {
						if err := m.store.EndMaintenanceWindow(mw.ID); err != nil {
							m.engine.AddLog("End maintenance failed: " + err.Error())
						}
						m.refreshData()
					}
				}
			case "d", "backspace":
				if m.currentTab == 0 && len(m.sites) > 0 {
					m.deleteID = m.sites[m.cursor].ID
					m.deleteName = m.sites[m.cursor].Name
					m.deleteTab = 0
					m.state = stateConfirmDelete
				} else if m.currentTab == 1 && len(m.alerts) > 0 {
					m.deleteID = m.alerts[m.cursor].ID
					m.deleteName = m.alerts[m.cursor].Name
					m.deleteTab = 1
					m.state = stateConfirmDelete
				} else if m.currentTab == 4 && len(m.maintenanceWindows) > 0 {
					m.deleteID = m.maintenanceWindows[m.cursor].ID
					m.deleteName = m.maintenanceWindows[m.cursor].Title
					m.deleteTab = 4
					m.state = stateConfirmDelete
				} else if m.currentTab == 5 && m.isAdmin && len(m.users) > 0 {
					m.deleteID = m.users[m.cursor].ID
					m.deleteName = m.users[m.cursor].Username
					m.deleteTab = 5
					m.state = stateConfirmDelete
				}
			}
		}
	}
	return m, nil
}

func (m *Model) handleClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	tabCount := 5
	if m.isAdmin {
		tabCount = 6
	}
	for i := 0; i < tabCount; i++ {
		if m.zones.Get(fmt.Sprintf("tab-%d", i)).InBounds(msg) {
			m.switchTab(i)
			return m, nil
		}
	}

	if m.currentTab == 0 {
		end := m.tableOffset + m.maxTableRows
		if end > len(m.sites) {
			end = len(m.sites)
		}
		for i := m.tableOffset; i < end; i++ {
			if m.zones.Get(fmt.Sprintf("site-%d", i)).InBounds(msg) {
				m.cursor = i
				return m, nil
			}
		}
	}

	if m.currentTab == 1 {
		end := m.tableOffset + m.maxTableRows
		if end > len(m.alerts) {
			end = len(m.alerts)
		}
		for i := m.tableOffset; i < end; i++ {
			if m.zones.Get(fmt.Sprintf("alert-%d", i)).InBounds(msg) {
				m.cursor = i
				return m, nil
			}
		}
	}

	if m.currentTab == 4 {
		end := m.tableOffset + m.maxTableRows
		if end > len(m.maintenanceWindows) {
			end = len(m.maintenanceWindows)
		}
		for i := m.tableOffset; i < end; i++ {
			if m.zones.Get(fmt.Sprintf("maint-%d", i)).InBounds(msg) {
				m.cursor = i
				return m, nil
			}
		}
	}

	if m.currentTab == 5 {
		end := m.tableOffset + m.maxTableRows
		if end > len(m.users) {
			end = len(m.users)
		}
		for i := m.tableOffset; i < end; i++ {
			if m.zones.Get(fmt.Sprintf("user-%d", i)).InBounds(msg) {
				m.cursor = i
				return m, nil
			}
		}
	}

	return m, nil
}

func (m *Model) switchTab(idx int) {
	maxTabs := 4
	if m.isAdmin {
		maxTabs = 5
	}
	if idx > maxTabs {
		idx = 0
	}
	m.currentTab = idx
	m.cursor = 0
	m.tableOffset = 0
	switch idx {
	case 2:
		m.state = stateLogs
	case 5:
		m.state = stateUsers
	default:
		m.state = stateDashboard
	}
}

func (m *Model) adjustCursor(newLen int) {
	if m.cursor >= newLen && m.cursor > 0 {
		m.cursor--
	}
	if m.cursor < m.tableOffset {
		m.tableOffset = m.cursor
		if m.tableOffset < 0 {
			m.tableOffset = 0
		}
	}
}

func (m *Model) refreshData() {
	allSites := m.engine.GetAllSites()

	var groups, ungrouped []models.Site
	children := make(map[int][]models.Site)
	for _, s := range allSites {
		if s.Type == "group" {
			groups = append(groups, s)
		} else if s.ParentID > 0 {
			children[s.ParentID] = append(children[s.ParentID], s)
		} else {
			ungrouped = append(ungrouped, s)
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].ID < groups[j].ID })
	for pid := range children {
		c := children[pid]
		sort.Slice(c, func(i, j int) bool { return c[i].ID < c[j].ID })
		sort.SliceStable(c, func(i, j int) bool { return siteOrder(c[i]) < siteOrder(c[j]) })
		children[pid] = c
	}
	sort.Slice(ungrouped, func(i, j int) bool { return ungrouped[i].ID < ungrouped[j].ID })
	sort.SliceStable(ungrouped, func(i, j int) bool { return siteOrder(ungrouped[i]) < siteOrder(ungrouped[j]) })

	var ordered []models.Site
	for _, g := range groups {
		ordered = append(ordered, g)
		if !m.collapsed[g.ID] {
			ordered = append(ordered, children[g.ID]...)
		}
	}
	ordered = append(ordered, ungrouped...)
	if m.filterText != "" {
		var filtered []models.Site
		needle := strings.ToLower(m.filterText)
		for _, s := range ordered {
			if strings.Contains(strings.ToLower(s.Name), needle) {
				filtered = append(filtered, s)
			}
		}
		ordered = filtered
	}
	m.sites = ordered
	if alerts, err := m.store.GetAllAlerts(); err == nil {
		m.alerts = alerts
	}
	if m.isAdmin {
		if users, err := m.store.GetAllUsers(); err == nil {
			m.users = users
		}
	}
	if nodes, err := m.store.GetAllNodes(); err == nil {
		m.nodes = nodes
	}
	if windows, err := m.store.GetAllMaintenanceWindows(100); err == nil {
		m.maintenanceWindows = windows
	}
	m.logViewport.SetContent(strings.Join(m.engine.GetLogs(), "\n"))

	listLen := len(m.sites)
	if m.currentTab == 1 {
		listLen = len(m.alerts)
	} else if m.currentTab == 3 {
		listLen = len(m.nodes)
	} else if m.currentTab == 4 {
		listLen = len(m.maintenanceWindows)
	} else if m.currentTab == 5 {
		listLen = len(m.users)
	}
	if listLen > 0 && m.cursor >= listLen {
		m.cursor = listLen - 1
	}
	if m.cursor < m.tableOffset {
		m.tableOffset = m.cursor
	}
}

func (m *Model) submitForm() {
	switch m.state {
	case stateFormSite:
		if m.siteFormData != nil {
			m.submitSiteForm()
		}
	case stateFormAlert:
		if m.alertFormData != nil {
			m.submitAlertForm()
		}
	case stateFormUser:
		if m.userFormData != nil {
			m.submitUserForm()
		}
	case stateFormMaint:
		if m.maintFormData != nil {
			m.submitMaintForm()
		}
	}
}

func (m Model) pulseIndicator() string {
	frame := m.tickCount % len(pulseFrames)
	brightness := int(m.pulsePos*155) + 100
	if brightness > 255 {
		brightness = 255
	}
	hasDown := false
	for _, s := range m.sites {
		if !s.Paused && !m.isMonitorInMaintenance(s.ID) && (s.Status == "DOWN" || s.Status == "SSL EXP") {
			hasDown = true
			break
		}
	}
	var color string
	if hasDown {
		color = fmt.Sprintf("#%02x%02x%02x", brightness, brightness/4, brightness/4)
	} else {
		color = fmt.Sprintf("#%02x%02x%02x", brightness/3, brightness, brightness/2)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(pulseFrames[frame])
}

func (m Model) View() string {
	switch m.state {
	case stateConfirmDelete:
		kind := "monitor"
		if m.deleteTab == 1 {
			kind = "alert"
		} else if m.deleteTab == 4 {
			kind = "maintenance window"
		} else if m.deleteTab == 5 {
			kind = "user"
		}
		msg := dangerStyle.Render(fmt.Sprintf("Delete %s \"%s\"?", kind, m.deleteName))
		hint := subtleStyle.Render("[y] Confirm  [n] Cancel")
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#F25D94")).
			Padding(1, 3).
			Render(msg + "\n\n" + hint)
		return lipgloss.NewStyle().Padding(2, 4).Render(box)
	case stateFormSite, stateFormAlert, stateFormUser, stateFormMaint:
		if m.huhForm != nil {
			title := ""
			switch m.state {
			case stateFormSite:
				title = "Add Monitor"
				if m.editID > 0 {
					title = fmt.Sprintf("Edit Monitor #%d", m.editID)
				}
			case stateFormAlert:
				title = "Add Alert"
				if m.editID > 0 {
					title = fmt.Sprintf("Edit Alert #%d", m.editID)
				}
			case stateFormUser:
				title = "Add User"
				if m.editID > 0 {
					title = fmt.Sprintf("Edit User #%d", m.editID)
				}
			case stateFormMaint:
				title = "New Maintenance Window"
			}
			formHeight := m.termHeight - 7
			if formHeight < 5 {
				formHeight = 5
			}
			m.huhForm.WithHeight(formHeight)
			header := titleStyle.Render(title)
			footer := subtleStyle.Render("\n[Esc] Cancel")
			return lipgloss.NewStyle().Padding(1, 2).Render(header + "\n\n" + m.huhForm.View() + "\n" + footer)
		}
		return ""
	case stateDetail:
		return m.viewDetailPanel()
	default:
		return m.zones.Scan(m.viewDashboard())
	}
}

func (m Model) viewDashboard() string {
	downCount := 0
	for _, s := range m.sites {
		if !s.Paused && !m.isMonitorInMaintenance(s.ID) && (s.Status == "DOWN" || s.Status == "SSL EXP") {
			downCount++
		}
	}
	offlineNodes := 0
	for _, n := range m.nodes {
		if !n.LastSeen.IsZero() && time.Since(n.LastSeen) > 5*time.Minute {
			offlineNodes++
		}
	}

	var sitesLabel string
	if downCount > 0 {
		sitesLabel = fmt.Sprintf("Sites (%d↓)", downCount)
	} else if len(m.sites) > 0 {
		sitesLabel = fmt.Sprintf("Sites (%d)", len(m.sites))
	} else {
		sitesLabel = "Sites"
	}
	var nodesLabel string
	if offlineNodes > 0 {
		nodesLabel = fmt.Sprintf("Nodes (%d!)", offlineNodes)
	} else if len(m.nodes) > 0 {
		nodesLabel = fmt.Sprintf("Nodes (%d)", len(m.nodes))
	} else {
		nodesLabel = "Nodes"
	}

	activeMaint := 0
	for _, mw := range m.maintenanceWindows {
		now := time.Now()
		if !mw.StartTime.After(now) && (mw.EndTime.IsZero() || mw.EndTime.After(now)) {
			activeMaint++
		}
	}
	var maintLabel string
	if activeMaint > 0 {
		maintLabel = fmt.Sprintf("Maint (%d)", activeMaint)
	} else {
		maintLabel = "Maint"
	}

	tabs := []string{sitesLabel, "Alerts", "Logs", nodesLabel, maintLabel}
	if m.isAdmin {
		tabs = append(tabs, "Users")
	}
	var renderedTabs []string
	for i, t := range tabs {
		var rendered string
		if i == m.currentTab {
			rendered = activeTab.Render(t)
		} else {
			rendered = inactiveTab.Render(t)
		}
		renderedTabs = append(renderedTabs, m.zones.Mark(fmt.Sprintf("tab-%d", i), rendered))
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)

	pulse := m.pulseIndicator()
	header = pulse + " " + header

	var content string
	switch m.currentTab {
	case 0:
		content = m.viewSitesTab()
	case 1:
		content = m.viewAlertsTab()
	case 2:
		content = m.viewLogsTab()
	case 3:
		content = m.viewNodesTab()
	case 4:
		content = m.viewMaintTab()
	case 5:
		if m.isAdmin {
			content = m.viewUsersTab()
		}
	}

	upCount := len(m.sites) - downCount
	var upStr string
	if downCount > 0 {
		upStr = dangerStyle.Render(fmt.Sprintf("%d/%d UP", upCount, len(m.sites)))
	} else {
		upStr = specialStyle.Render(fmt.Sprintf("%d/%d UP", upCount, len(m.sites)))
	}
	statusParts := []string{upStr}
	if len(m.nodes) > 0 {
		online := 0
		for _, n := range m.nodes {
			if !n.LastSeen.IsZero() && time.Since(n.LastSeen) < 60*time.Second {
				online++
			}
		}
		statusParts = append(statusParts, fmt.Sprintf("%d probes", online))
	}
	statusLine := strings.Join(statusParts, subtleStyle.Render(" · "))

	var footer string
	if m.filterMode {
		cursor := lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Render("│")
		footer = "\n" + titleStyle.Render("/") + " " + m.filterText + cursor + "  " + subtleStyle.Render("[Enter]Apply [Esc]Clear")
	} else {
		var keys string
		switch m.currentTab {
		case 0:
			keys = "[/]Filter [n]New [e]Edit [i]Info [d]Del [p]Pause [Tab]Switch [q]Quit"
		case 4:
			keys = "[n]New [x]End [d]Del [Tab]Switch [q]Quit"
		case 5:
			keys = "[n]Add [d]Revoke [Tab]Switch [q]Quit"
		default:
			keys = "[Tab]Switch [q]Quit"
		}
		footer = "\n" + statusLine + "  " + subtleStyle.Render(keys)
		if m.filterText != "" && m.currentTab == 0 {
			footer = "\n" + subtleStyle.Render(fmt.Sprintf("filter: %s", m.filterText)) + "  " + statusLine + "  " + subtleStyle.Render(keys)
		}
	}
	s := lipgloss.NewStyle().Padding(1, 2)
	if m.termHeight > 0 {
		s = s.MaxHeight(m.termHeight)
	}
	return s.Render(header + "\n" + content + "\n" + footer)
}

func siteOrder(s models.Site) int {
	if s.Paused {
		return 3
	}
	switch s.Status {
	case "DOWN", "SSL EXP":
		return 0
	case "PENDING":
		return 2
	default:
		return 1
	}
}

func limitStr(text string, max int) string {
	if len(text) > max {
		return text[:max-3] + "..."
	}
	return text
}
