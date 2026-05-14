package tui

import (
	"fmt"
	"strings"
)

func colorizeLog(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "confirmed down"),
		strings.Contains(lower, "is down"),
		strings.Contains(lower, "missed heartbeat"),
		strings.Contains(lower, "failed check"),
		strings.Contains(lower, "ssl warning"):
		return dangerStyle.Render(line)
	case strings.Contains(lower, "recovered"),
		strings.Contains(lower, "is up"),
		strings.Contains(lower, "recovery"):
		return specialStyle.Render(line)
	case strings.Contains(lower, "engine"),
		strings.Contains(lower, "cluster"):
		return titleStyle.Render(line)
	default:
		return line
	}
}

func (m Model) viewLogsTab() string {
	content := m.logViewport.View()
	if strings.TrimSpace(content) == "" || content == "Waiting for logs..." {
		return "\n  No log entries yet. Logs appear as monitors run checks."
	}

	lines := strings.Split(content, "\n")
	var colored []string
	for _, line := range lines {
		if line == "" {
			colored = append(colored, line)
			continue
		}
		colored = append(colored, colorizeLog(line))
	}

	count := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			count++
		}
	}

	header := subtleStyle.Render(fmt.Sprintf("  %d entries  [↑/↓] Scroll  [PgUp/PgDn] Page", count))
	return "\n" + header + "\n\n" + strings.Join(colored, "\n")
}
