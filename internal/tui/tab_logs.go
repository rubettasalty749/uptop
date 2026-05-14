package tui

func (m Model) viewLogsTab() string {
	return "\n" + m.logViewport.View()
}
