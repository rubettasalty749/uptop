package tui

import (
	"fmt"
	"go-upkeep/internal/store"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	userHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true).
			Padding(0, 1)

	userCellStyle = lipgloss.NewStyle().Padding(0, 1)

	userSelectedStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Bold(true).
				Foreground(lipgloss.Color("#ffffff")).
				Background(lipgloss.Color("#3b3b5c"))

	userBorderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444"))

	userColWidths = []int{4, 16, 10, 44}
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

	end := m.tableOffset + m.maxTableRows
	if end > len(m.users) {
		end = len(m.users)
	}

	selectedVisual := m.cursor - m.tableOffset

	var rows [][]string
	for i := m.tableOffset; i < end; i++ {
		u := m.users[i]
		rows = append(rows, []string{
			fmt.Sprintf("%d", u.ID),
			m.zones.Mark(fmt.Sprintf("user-%d", i), limitStr(u.Username, 15)),
			fmtRole(u.Role),
			fmtKey(u.PublicKey),
		})
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(userBorderStyle).
		Headers("ID", "USERNAME", "ROLE", "PUBLIC KEY").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				s := userHeaderStyle
				if col < len(userColWidths) {
					s = s.Width(userColWidths[col])
				}
				return s
			}
			s := userCellStyle
			if row == selectedVisual {
				s = userSelectedStyle
			}
			if col < len(userColWidths) {
				s = s.Width(userColWidths[col])
			}
			return s
		})

	return "\n" + t.Render()
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
	).WithTheme(huh.ThemeDracula())

	return m.huhForm.Init()
}

func (m *Model) submitUserForm() {
	d := m.userFormData
	if m.editID > 0 {
		store.Get().UpdateUser(m.editID, d.Username, d.PublicKey, d.Role)
	} else {
		store.Get().AddUser(d.Username, d.PublicKey, d.Role)
	}
	m.state = stateUsers
}
