package tui

import (
	"fmt"
	"go-upkeep/internal/store"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type userFormData struct {
	Username  string
	PublicKey string
}

func (m Model) viewUsersTab() string {
	var content string
	content += fmt.Sprintf("\n%-3s %-15s %-10s %s\n", "ID", "USER", "ROLE", "KEY")
	content += subtleStyle.Render("----------------------------------------------------------------") + "\n"
	end := m.tableOffset + m.maxTableRows
	if end > len(m.users) {
		end = len(m.users)
	}
	for i := m.tableOffset; i < end; i++ {
		u := m.users[i]
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		row := fmt.Sprintf("%s %-3d %-15s %-10s %s", cursor, u.ID, limitStr(u.Username, 15), u.Role, limitStr(u.PublicKey, 40))
		if m.cursor == i {
			row = lipgloss.NewStyle().Bold(true).Render(row)
		}
		content += row + "\n"
	}
	return content
}

func (m *Model) initUserHuhForm() tea.Cmd {
	m.userFormData = &userFormData{}

	m.huhForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Username").
				Placeholder("admin").
				Value(&m.userFormData.Username).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("username is required")
					}
					return nil
				}),
			huh.NewInput().Title("SSH Public Key").
				Placeholder("ssh-ed25519 AAAA...").
				Value(&m.userFormData.PublicKey).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("public key is required")
					}
					return nil
				}),
		).Title("SSH Access"),
	).WithTheme(huh.ThemeDracula())

	return m.huhForm.Init()
}

func (m *Model) submitUserForm() {
	store.Get().AddUser(m.userFormData.Username, m.userFormData.PublicKey, "user")
	m.state = stateUsers
}
