package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

var (
	tableHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7D56F4")).
				Bold(true).
				Padding(0, 1)

	tableCellStyle = lipgloss.NewStyle().Padding(0, 1)

	tableSelectedStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Bold(true).
				Foreground(lipgloss.Color("#ffffff")).
				Background(lipgloss.Color("#3b3b5c"))

	tableBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#444"))
)

type StyleOverride func(row, col int) *lipgloss.Style

func (m Model) renderTable(headers []string, items int, buildRows func(start, end int) [][]string, colWidths []int, styleOverride StyleOverride) string {
	if items == 0 {
		return ""
	}

	end := m.tableOffset + m.maxTableRows
	if end > items {
		end = items
	}

	selectedVisual := m.cursor - m.tableOffset
	rows := buildRows(m.tableOffset, end)

	tableWidth := m.termWidth - 6
	if tableWidth < 40 {
		tableWidth = 40
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(tableBorderStyle).
		Width(tableWidth).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return tableHeaderStyle
			}
			if styleOverride != nil {
				if s := styleOverride(row, col); s != nil {
					if col < len(colWidths) && colWidths[col] > 0 {
						return s.Width(colWidths[col])
					}
					return *s
				}
			}
			base := tableCellStyle
			if row == selectedVisual {
				base = tableSelectedStyle
			}
			if col < len(colWidths) && colWidths[col] > 0 {
				base = base.Width(colWidths[col])
			}
			return base
		})

	return "\n" + t.Render()
}
