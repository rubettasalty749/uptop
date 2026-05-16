package tui

import (
	"fmt"
	"go-upkeep/internal/models"
	"strings"
	"time"
)

func (m Model) viewNodesTab() string {
	if len(m.nodes) == 0 {
		return "\n  No probe nodes connected."
	}

	colWidths := []int{0, 12, 20, 10, 8}

	return m.renderTable(
		[]string{"NAME", "REGION", "LAST SEEN", "VERSION", "STATUS"},
		len(m.nodes),
		func(start, end int) [][]string {
			var rows [][]string
			for i := start; i < end; i++ {
				node := m.nodes[i]
				name := limitStr(node.Name, 20)
				if name == "" {
					name = node.ID
				}
				region := node.Region
				if region == "" {
					region = subtleStyle.Render("—")
				}
				lastSeen := fmtNodeLastSeen(node.LastSeen)
				version := node.Version
				if version == "" {
					version = subtleStyle.Render("—")
				}
				status := fmtNodeStatus(node.LastSeen)
				rows = append(rows, []string{name, region, lastSeen, version, status})
			}
			return rows
		},
		colWidths,
		nil,
	)
}

func fmtNodeStatus(lastSeen time.Time) string {
	if lastSeen.IsZero() {
		return subtleStyle.Render("UNKNOWN")
	}
	ago := time.Since(lastSeen)
	if ago < 60*time.Second {
		return specialStyle.Render("ONLINE")
	}
	if ago < 5*time.Minute {
		return warnStyle.Render("STALE")
	}
	return dangerStyle.Render("OFFLINE")
}

func fmtNodeLastSeen(t time.Time) string {
	if t.IsZero() {
		return subtleStyle.Render("never")
	}
	ago := time.Since(t)
	if ago < time.Minute {
		return fmt.Sprintf("%ds ago", int(ago.Seconds()))
	}
	if ago < time.Hour {
		return fmt.Sprintf("%dm ago", int(ago.Minutes()))
	}
	return fmt.Sprintf("%dh ago", int(ago.Hours()))
}

func fmtProbeRegions(site models.Site, probeResults map[string]probeStatus) string {
	if len(probeResults) == 0 {
		return subtleStyle.Render("—")
	}
	var parts []string
	for region, status := range probeResults {
		short := region
		if len(short) > 6 {
			short = short[:6]
		}
		if status.isUp {
			parts = append(parts, specialStyle.Render(short+":UP"))
		} else {
			parts = append(parts, dangerStyle.Render(short+":DN"))
		}
	}
	return strings.Join(parts, " ")
}

type probeStatus struct {
	isUp bool
}
