package tui

import (
	"fmt"
	"go-upkeep/internal/store"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
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
}

func (m Model) viewAlertsTab() string {
	var content string
	content += fmt.Sprintf("\n%-3s %-15s %-10s %s\n", "ID", "NAME", "TYPE", "CONFIG")
	content += subtleStyle.Render("----------------------------------------------------------------") + "\n"
	end := m.tableOffset + m.maxTableRows
	if end > len(m.alerts) {
		end = len(m.alerts)
	}
	for i := m.tableOffset; i < end; i++ {
		alert := m.alerts[i]
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		confStr := "settings..."
		if val, ok := alert.Settings["url"]; ok {
			confStr = limitStr(val, 30)
		}
		if alert.Type == "email" {
			confStr = fmt.Sprintf("SMTP: %s", alert.Settings["host"])
		}
		row := fmt.Sprintf("%s %-3d %-15s %-10s %s", cursor, alert.ID, limitStr(alert.Name, 15), alert.Type, confStr)
		if m.cursor == i {
			row = lipgloss.NewStyle().Bold(true).Render(row)
		}
		content += row + "\n"
	}
	return content
}

func (m *Model) initAlertHuhForm() tea.Cmd {
	m.alertFormData = &alertFormData{
		AlertType: "discord",
	}

	if m.editID > 0 {
		for _, alert := range m.alerts {
			if alert.ID == m.editID {
				m.alertFormData.Name = alert.Name
				m.alertFormData.AlertType = alert.Type
				if url, ok := alert.Settings["url"]; ok {
					m.alertFormData.WebhookURL = url
				}
				if alert.Type == "email" {
					m.alertFormData.SMTPHost = alert.Settings["host"]
					m.alertFormData.SMTPPort = alert.Settings["port"]
					m.alertFormData.SMTPUser = alert.Settings["user"]
					m.alertFormData.SMTPPass = alert.Settings["pass"]
					m.alertFormData.EmailFrom = alert.Settings["from"]
					m.alertFormData.EmailTo = alert.Settings["to"]
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
				).Value(&m.alertFormData.AlertType),
		).Title("Alert Config"),
		huh.NewGroup(
			huh.NewInput().Title("Webhook URL").
				Placeholder("https://discord.com/api/webhooks/...").
				Value(&m.alertFormData.WebhookURL),
		).Title("Webhook").WithHideFunc(func() bool {
			return m.alertFormData.AlertType == "email"
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

	if d.AlertType == "email" {
		settings["host"] = d.SMTPHost
		settings["port"] = d.SMTPPort
		settings["user"] = d.SMTPUser
		settings["pass"] = d.SMTPPass
		settings["from"] = d.EmailFrom
		settings["to"] = d.EmailTo
	} else {
		settings["url"] = d.WebhookURL
	}

	if m.editID > 0 {
		store.Get().UpdateAlert(m.editID, d.Name, d.AlertType, settings)
	} else {
		store.Get().AddAlert(d.Name, d.AlertType, settings)
	}
	m.state = stateDashboard
}
