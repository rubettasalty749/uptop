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
	subtleStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"})
	specialStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"})
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#F0E442", Dark: "#F0E442"})
	dangerStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#F25D94", Dark: "#F25D94"})
	titleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true)

	activeTab   = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, true, false).BorderForeground(lipgloss.Color("#7D56F4")).Foreground(lipgloss.Color("#7D56F4")).Bold(true).Padding(0, 1)
	inactiveTab = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.AdaptiveColor{Light: "#AAA", Dark: "#555"})
)

var pulseFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type sessionState int

const (
	stateDashboard sessionState = iota
	stateLogs
	stateUsers
	stateFormSite
	stateFormAlert
	stateFormUser
)

type Model struct {
	state        sessionState
	currentTab   int
	cursor       int
	tableOffset  int
	maxTableRows int
	editID       int
	editToken    string

	huhForm       *huh.Form
	siteFormData  *siteFormData
	alertFormData *alertFormData
	userFormData  *userFormData

	logViewport viewport.Model
	isAdmin     bool
	zones       *zone.Manager

	// harmonica animation state
	pulseSpring harmonica.Spring
	pulsePos    float64
	pulseVel    float64
	tickCount   int

	sites  []models.Site
	alerts []models.AlertConfig
	users  []models.User
}

func InitialModel(isAdmin bool) Model {
	vpLogs := viewport.New(100, 20)
	vpLogs.SetContent("Waiting for logs...")
	z := zone.New()
	spring := harmonica.NewSpring(harmonica.FPS(10), 6.0, 0.4)
	return Model{
		state:        stateDashboard,
		logViewport:  vpLogs,
		maxTableRows: 5,
		isAdmin:      isAdmin,
		zones:        z,
		pulseSpring:  spring,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.ClearScreen, tea.Tick(time.Second, func(t time.Time) tea.Msg { return t }))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Form state: forward ALL messages to huh (keys, timers, resize, etc.)
	if m.state == stateFormSite || m.state == stateFormAlert || m.state == stateFormUser {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			if keyMsg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			if keyMsg.String() == "esc" {
				m.huhForm = nil
				m.state = stateDashboard
				if m.currentTab == 3 {
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
		m.maxTableRows = msg.Height - 12
		if m.maxTableRows < 1 {
			m.maxTableRows = 1
		}
		m.logViewport.Width = msg.Width
		m.logViewport.Height = msg.Height - 6
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
		}

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "ctrl+l" {
			return m, tea.ClearScreen
		}

		switch m.state {
		case stateDashboard, stateLogs, stateUsers:
			switch msg.String() {
			case "q":
				return m, tea.Quit
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
				} else if m.currentTab == 3 && m.isAdmin {
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
				} else if m.currentTab == 3 && m.isAdmin && len(m.users) > 0 {
					m.editID = m.users[m.cursor].ID
					m.state = stateFormUser
					return m, m.initUserHuhForm()
				}
			case "d", "backspace":
				if m.currentTab == 1 && len(m.alerts) > 0 {
					store.Get().DeleteAlert(m.alerts[m.cursor].ID)
					m.adjustCursor(len(m.alerts) - 1)
				} else if m.currentTab == 0 && len(m.sites) > 0 {
					id := m.sites[m.cursor].ID
					store.Get().DeleteSite(id)
					monitor.RemoveSite(id)
					m.adjustCursor(len(m.sites) - 1)
				} else if m.currentTab == 3 && m.isAdmin && len(m.users) > 0 {
					store.Get().DeleteUser(m.users[m.cursor].ID)
					m.adjustCursor(len(m.users) - 1)
				}
				m.refreshData()
			}
		}
	}
	return m, nil
}

func (m *Model) handleClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	maxTabs := 3
	if !m.isAdmin {
		maxTabs = 2
	}
	for i := 0; i <= maxTabs; i++ {
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

	if m.currentTab == 3 {
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
	maxTabs := 2
	if m.isAdmin {
		maxTabs = 3
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
	case 3:
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
	monitor.Mutex.RLock()
	var sites []models.Site
	for _, s := range monitor.LiveState {
		sites = append(sites, s)
	}
	monitor.Mutex.RUnlock()
	sort.Slice(sites, func(i, j int) bool { return sites[i].ID < sites[j].ID })
	m.sites = sites
	if store.Get() != nil {
		m.alerts = store.Get().GetAllAlerts()
		if m.isAdmin {
			m.users = store.Get().GetAllUsers()
		}
	}
	m.logViewport.SetContent(strings.Join(monitor.GetLogs(), "\n"))
}

func (m *Model) submitForm() {
	if store.Get() == nil {
		return
	}
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
	}
}

func (m Model) pulseIndicator() string {
	frame := m.tickCount % len(pulseFrames)
	brightness := int(m.pulsePos*155) + 100
	if brightness > 255 {
		brightness = 255
	}
	color := fmt.Sprintf("#%02x%02x%02x", brightness/3, brightness, brightness/2)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(pulseFrames[frame])
}

func (m Model) View() string {
	switch m.state {
	case stateFormSite, stateFormAlert, stateFormUser:
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
			}
			header := titleStyle.Render(title)
			footer := subtleStyle.Render("\n[Esc] Cancel")
			return lipgloss.NewStyle().Padding(1, 2).Render(header + "\n\n" + m.huhForm.View() + "\n" + footer)
		}
		return ""
	default:
		return m.zones.Scan(m.viewDashboard())
	}
}

func (m Model) viewDashboard() string {
	tabs := []string{"Sites", "Alerts", "Logs"}
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
		if m.isAdmin {
			content = m.viewUsersTab()
		}
	}

	footer := subtleStyle.Render("\n[n] New  [e/Enter] Edit  [d] Delete  [Tab/Click] Switch  [Ctrl+L] Clear  [q] Quit")
	if m.currentTab == 3 {
		footer = subtleStyle.Render("\n[n] Add User  [d] Revoke  [Tab/Click] Switch  [Ctrl+L] Clear  [q] Quit")
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(header + "\n" + content + "\n" + footer)
}

func limitStr(text string, max int) string {
	if len(text) > max {
		return text[:max-3] + "..."
	}
	return text
}
