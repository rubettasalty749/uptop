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

	tableZebraStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("#1a1a2e"))
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

	tableWidth := m.termWidth - chromePadH - 2
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
			isSelected := row == selectedVisual
			if styleOverride != nil {
				if s := styleOverride(row, col); s != nil {
					style := *s
					if isSelected {
						style = tableSelectedStyle.Foreground(s.GetForeground())
					}
					if col < len(colWidths) && colWidths[col] > 0 {
						style = style.Width(colWidths[col])
					}
					return style
				}
			}
			base := tableCellStyle
			if row%2 == 1 {
				base = tableZebraStyle
			}
			if isSelected {
				base = tableSelectedStyle
			}
			if col < len(colWidths) && colWidths[col] > 0 {
				base = base.Width(colWidths[col])
			}
			return base
		})

	return "\n" + t.Render()
}
