package tui

import (
	"fmt"
	"go-upkeep/internal/monitor"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	alertHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7D56F4")).
				Bold(true).
				Padding(0, 1)

	alertCellStyle = lipgloss.NewStyle().Padding(0, 1)

	alertSelectedStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Bold(true).
				Foreground(lipgloss.Color("#ffffff")).
				Background(lipgloss.Color("#3b3b5c"))

	alertBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#444"))
)

type alertFormData struct {
	Name       string
	AlertType  string
	WebhookURL string
	SMTPHost   string
	SMTPPort   string
	SMTPUser   string
	SMTPPass   string
	EmailFrom  string
	EmailTo    string
	NtfyURL    string
	NtfyTopic  string
	NtfyUser   string
	NtfyPass   string
	NtfyPri    string
}

func fmtAlertType(t string) string {
	switch t {
	case "discord":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#5865F2")).Render(t)
	case "slack":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#E01E5A")).Render(t)
	case "webhook":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#F0E442")).Render(t)
	case "email":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#73F59F")).Render(t)
	case "ntfy":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Render(t)
	default:
		return t
	}
}

func fmtAlertConfig(alert struct {
	Type     string
	Settings map[string]string
}) string {
	switch alert.Type {
	case "email":
		host := alert.Settings["host"]
		to := alert.Settings["to"]
		if host != "" && to != "" {
			return limitStr(fmt.Sprintf("%s → %s", host, to), 34)
		}
		if host != "" {
			return limitStr(host, 34)
		}
		return subtleStyle.Render("—")
	case "ntfy":
		topic := alert.Settings["topic"]
		url := alert.Settings["url"]
		if url != "" && topic != "" {
			return limitStr(fmt.Sprintf("%s/%s", url, topic), 34)
		}
		return subtleStyle.Render("—")
	default:
		if val, ok := alert.Settings["url"]; ok {
			return limitStr(val, 34)
		}
		return subtleStyle.Render("—")
	}
}

func (m Model) viewAlertsTab() string {
	if len(m.alerts) == 0 {
		return "\n  No alert channels configured. Press [n] to add one."
	}

	end := m.tableOffset + m.maxTableRows
	if end > len(m.alerts) {
		end = len(m.alerts)
	}

	selectedVisual := m.cursor - m.tableOffset

	var rows [][]string
	for i := m.tableOffset; i < end; i++ {
		alert := m.alerts[i]
		rows = append(rows, []string{
			fmt.Sprintf("%d", i+1),
			m.zones.Mark(fmt.Sprintf("alert-%d", i), limitStr(alert.Name, 15)),
			fmtAlertType(alert.Type),
			fmtAlertConfig(struct {
				Type     string
				Settings map[string]string
			}{alert.Type, alert.Settings}),
		})
	}

	tableWidth := m.termWidth - 6
	if tableWidth < 40 {
		tableWidth = 40
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(alertBorderStyle).
		Width(tableWidth).
		Headers("#", "NAME", "TYPE", "CONFIG").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return alertHeaderStyle
			}
			if row == selectedVisual {
				return alertSelectedStyle
			}
			return alertCellStyle
		})

	return "\n" + t.Render()
}

func (m *Model) initAlertHuhForm() tea.Cmd {
	m.alertFormData = &alertFormData{
		AlertType: "discord",
		NtfyPri:   "3",
	}

	if m.editID > 0 {
		for _, alert := range m.alerts {
			if alert.ID == m.editID {
				m.alertFormData.Name = alert.Name
				m.alertFormData.AlertType = alert.Type
				if url, ok := alert.Settings["url"]; ok {
					m.alertFormData.WebhookURL = url
				}
				switch alert.Type {
				case "email":
					m.alertFormData.SMTPHost = alert.Settings["host"]
					m.alertFormData.SMTPPort = alert.Settings["port"]
					m.alertFormData.SMTPUser = alert.Settings["user"]
					m.alertFormData.SMTPPass = alert.Settings["pass"]
					m.alertFormData.EmailFrom = alert.Settings["from"]
					m.alertFormData.EmailTo = alert.Settings["to"]
				case "ntfy":
					m.alertFormData.NtfyURL = alert.Settings["url"]
					m.alertFormData.NtfyTopic = alert.Settings["topic"]
					m.alertFormData.NtfyUser = alert.Settings["username"]
					m.alertFormData.NtfyPass = alert.Settings["password"]
					m.alertFormData.NtfyPri = alert.Settings["priority"]
				}
				break
			}
		}
	}

	m.huhForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Alert Name").
				Placeholder("My Alert Channel").
				Value(&m.alertFormData.Name).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("name is required")
					}
					return nil
				}),
			huh.NewSelect[string]().Title("Alert Type").
				Options(
					huh.NewOption("Discord", "discord"),
					huh.NewOption("Slack", "slack"),
					huh.NewOption("Webhook", "webhook"),
					huh.NewOption("Email (SMTP)", "email"),
					huh.NewOption("Ntfy", "ntfy"),
				).Value(&m.alertFormData.AlertType),
		).Title("Alert Config"),
		huh.NewGroup(
			huh.NewInput().Title("Webhook URL").
				Placeholder("https://discord.com/api/webhooks/...").
				Value(&m.alertFormData.WebhookURL),
		).Title("Webhook").WithHideFunc(func() bool {
			return m.alertFormData.AlertType == "email" || m.alertFormData.AlertType == "ntfy"
		}),
		huh.NewGroup(
			huh.NewInput().Title("Ntfy Server URL").
				Placeholder("https://ntfy.sh").
				Value(&m.alertFormData.NtfyURL),
			huh.NewInput().Title("Topic").
				Placeholder("my-alerts").
				Value(&m.alertFormData.NtfyTopic),
			huh.NewSelect[string]().Title("Priority").
				Options(
					huh.NewOption("Min (1)", "1"),
					huh.NewOption("Low (2)", "2"),
					huh.NewOption("Default (3)", "3"),
					huh.NewOption("High (4)", "4"),
					huh.NewOption("Urgent (5)", "5"),
				).Value(&m.alertFormData.NtfyPri),
			huh.NewInput().Title("Username (optional)").
				Placeholder("admin").
				Value(&m.alertFormData.NtfyUser),
			huh.NewInput().Title("Password (optional)").
				EchoMode(huh.EchoModePassword).
				Value(&m.alertFormData.NtfyPass),
		).Title("Ntfy Settings").WithHideFunc(func() bool {
			return m.alertFormData.AlertType != "ntfy"
		}),
		huh.NewGroup(
			huh.NewInput().Title("SMTP Host").
				Placeholder("smtp.gmail.com").
				Value(&m.alertFormData.SMTPHost),
			huh.NewInput().Title("SMTP Port").
				Placeholder("587").
				Value(&m.alertFormData.SMTPPort),
			huh.NewInput().Title("SMTP User").
				Placeholder("user@gmail.com").
				Value(&m.alertFormData.SMTPUser),
			huh.NewInput().Title("SMTP Password").
				EchoMode(huh.EchoModePassword).
				Value(&m.alertFormData.SMTPPass),
			huh.NewInput().Title("From Email").
				Placeholder("alerts@domain.com").
				Value(&m.alertFormData.EmailFrom),
			huh.NewInput().Title("To Email").
				Placeholder("oncall@domain.com").
				Value(&m.alertFormData.EmailTo),
		).Title("Email Settings").WithHideFunc(func() bool {
			return m.alertFormData.AlertType != "email"
		}),
	).WithTheme(huh.ThemeDracula())

	return m.huhForm.Init()
}

func (m *Model) submitAlertForm() {
	d := m.alertFormData
	settings := make(map[string]string)

	switch d.AlertType {
	case "email":
		settings["host"] = d.SMTPHost
		settings["port"] = d.SMTPPort
		settings["user"] = d.SMTPUser
		settings["pass"] = d.SMTPPass
		settings["from"] = d.EmailFrom
		settings["to"] = d.EmailTo
	case "ntfy":
		settings["url"] = d.NtfyURL
		settings["topic"] = d.NtfyTopic
		settings["priority"] = d.NtfyPri
		settings["username"] = d.NtfyUser
		settings["password"] = d.NtfyPass
	default:
		settings["url"] = d.WebhookURL
	}

	if m.editID > 0 {
		if err := m.store.UpdateAlert(m.editID, d.Name, d.AlertType, settings); err != nil {
			monitor.AddLog("Update alert failed: " + err.Error())
		}
	} else {
		if err := m.store.AddAlert(d.Name, d.AlertType, settings); err != nil {
			monitor.AddLog("Add alert failed: " + err.Error())
		}
	}
	m.state = stateDashboard
}
