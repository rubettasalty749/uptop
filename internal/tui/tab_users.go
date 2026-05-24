package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

type userFormData struct {
	Username  string
	PublicKey string
	Role      string
}

func fmtRole(role string) string {
	if role == "admin" {
		return specialStyle.Render(role)
	}
	return role
}

func fmtKey(key string) string {
	if len(key) > 40 {
		return key[:20] + "..." + key[len(key)-17:]
	}
	return key
}

func (m Model) viewUsersTab() string {
	if len(m.users) == 0 {
		return "\n  No users configured. Press [n] to add one."
	}

	return m.renderTable(
		[]string{"#", "USERNAME", "ROLE", "PUBLIC KEY"},
		len(m.users),
		func(start, end int) [][]string {
			var rows [][]string
			for i := start; i < end; i++ {
				u := m.users[i]
				rows = append(rows, []string{
					fmt.Sprintf("%d", i+1),
					m.zones.Mark(fmt.Sprintf("user-%d", i), limitStr(u.Username, 15)),
					fmtRole(u.Role),
					fmtKey(u.PublicKey),
				})
			}
			return rows
		},
		nil, nil,
	)
}

func (m *Model) initUserHuhForm() tea.Cmd {
	m.userFormData = &userFormData{
		Role: "user",
	}

	if m.editID > 0 {
		for _, u := range m.users {
			if u.ID == m.editID {
				m.userFormData.Username = u.Username
				m.userFormData.PublicKey = u.PublicKey
				m.userFormData.Role = u.Role
				break
			}
		}
	}

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
			huh.NewSelect[string]().Title("Role").
				Options(
					huh.NewOption("User", "user"),
					huh.NewOption("Admin", "admin"),
				).Value(&m.userFormData.Role),
		).Title("SSH Access"),
	).WithTheme(m.theme.HuhTheme())

	return m.huhForm.Init()
}

func (m *Model) submitUserForm() {
	d := m.userFormData
	if m.editID > 0 {
		if err := m.store.UpdateUser(m.editID, d.Username, d.PublicKey, d.Role); err != nil {
			m.engine.AddLog("Update user failed: " + err.Error())
		}
	} else {
		if err := m.store.AddUser(d.Username, d.PublicKey, d.Role); err != nil {
			m.engine.AddLog("Add user failed: " + err.Error())
		}
	}
	m.state = stateUsers
}
