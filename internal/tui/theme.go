package tui

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type Theme struct {
	Name string

	// Base layers
	Bg      lipgloss.Color
	Surface lipgloss.Color
	Panel   lipgloss.Color
	Border  lipgloss.Color

	// Text
	Fg     lipgloss.Color
	Muted  lipgloss.Color
	Subtle lipgloss.Color

	// Semantic
	Success lipgloss.Color
	Warning lipgloss.Color
	Danger  lipgloss.Color
	Info    lipgloss.Color
	Accent  lipgloss.Color
	Purple  lipgloss.Color

	// Table
	ZebraBg lipgloss.Color

	// Selection
	SelectedFg lipgloss.Color
	SelectedBg lipgloss.Color
}

var themes = []Theme{
	themeFlexokiDark,
	themeFlexokiLight,
	themeCatppuccinMocha,
	themeNord,
}

var themeFlexokiDark = Theme{
	Name:       "Flexoki Dark",
	Bg:         "#1C1B1A",
	Surface:    "#282726",
	Panel:      "#343331",
	Border:     "#575653",
	Fg:         "#CECDC3",
	Muted:      "#878580",
	Subtle:     "#6F6E69",
	Success:    "#879A39",
	Warning:    "#D0A215",
	Danger:     "#D14D41",
	Info:       "#4385BE",
	Accent:     "#3AA99F",
	Purple:     "#8B7EC8",
	ZebraBg:    "#222120",
	SelectedFg: "#FFFCF0",
	SelectedBg: "#403E3C",
}

var themeFlexokiLight = Theme{
	Name:       "Flexoki Light",
	Bg:         "#FFFCF0",
	Surface:    "#F2F0E5",
	Panel:      "#E6E4D9",
	Border:     "#B7B5AC",
	Fg:         "#100F0F",
	Muted:      "#6F6E69",
	Subtle:     "#878580",
	Success:    "#66800B",
	Warning:    "#AD8301",
	Danger:     "#AF3029",
	Info:       "#205EA6",
	Accent:     "#24837B",
	Purple:     "#5E409D",
	ZebraBg:    "#F7F5EA",
	SelectedFg: "#100F0F",
	SelectedBg: "#DAD8CE",
}

var themeCatppuccinMocha = Theme{
	Name:       "Catppuccin Mocha",
	Bg:         "#1e1e2e",
	Surface:    "#313244",
	Panel:      "#45475a",
	Border:     "#585b70",
	Fg:         "#cdd6f4",
	Muted:      "#a6adc8",
	Subtle:     "#6c7086",
	Success:    "#a6e3a1",
	Warning:    "#f9e2af",
	Danger:     "#f38ba8",
	Info:       "#89b4fa",
	Accent:     "#94e2d5",
	Purple:     "#cba6f7",
	ZebraBg:    "#232334",
	SelectedFg: "#cdd6f4",
	SelectedBg: "#45475a",
}

var themeNord = Theme{
	Name:       "Nord",
	Bg:         "#2e3440",
	Surface:    "#3b4252",
	Panel:      "#434c5e",
	Border:     "#4c566a",
	Fg:         "#d8dee9",
	Muted:      "#d8dee9",
	Subtle:     "#4c566a",
	Success:    "#a3be8c",
	Warning:    "#ebcb8b",
	Danger:     "#bf616a",
	Info:       "#81a1c1",
	Accent:     "#88c0d0",
	Purple:     "#b48ead",
	ZebraBg:    "#323845",
	SelectedFg: "#eceff4",
	SelectedBg: "#434c5e",
}

func (t Theme) HuhTheme() *huh.Theme {
	ht := huh.ThemeBase()

	ht.Focused.Base = ht.Focused.Base.BorderForeground(t.Border)
	ht.Focused.Card = ht.Focused.Base
	ht.Focused.Title = ht.Focused.Title.Foreground(t.Accent).Bold(true)
	ht.Focused.NoteTitle = ht.Focused.NoteTitle.Foreground(t.Accent).Bold(true).MarginBottom(1)
	ht.Focused.Description = ht.Focused.Description.Foreground(t.Muted)
	ht.Focused.ErrorIndicator = ht.Focused.ErrorIndicator.Foreground(t.Danger)
	ht.Focused.ErrorMessage = ht.Focused.ErrorMessage.Foreground(t.Danger)
	ht.Focused.SelectSelector = ht.Focused.SelectSelector.Foreground(t.Purple)
	ht.Focused.NextIndicator = ht.Focused.NextIndicator.Foreground(t.Purple)
	ht.Focused.PrevIndicator = ht.Focused.PrevIndicator.Foreground(t.Purple)
	ht.Focused.Option = ht.Focused.Option.Foreground(t.Fg)
	ht.Focused.MultiSelectSelector = ht.Focused.MultiSelectSelector.Foreground(t.Purple)
	ht.Focused.SelectedOption = ht.Focused.SelectedOption.Foreground(t.Success)
	ht.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(t.Success).SetString("✓ ")
	ht.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(t.Subtle).SetString("• ")
	ht.Focused.UnselectedOption = ht.Focused.UnselectedOption.Foreground(t.Fg)
	ht.Focused.FocusedButton = ht.Focused.FocusedButton.Foreground(t.Bg).Background(t.Accent)
	ht.Focused.Next = ht.Focused.FocusedButton
	ht.Focused.BlurredButton = ht.Focused.BlurredButton.Foreground(t.Fg).Background(t.Surface)
	ht.Focused.TextInput.Cursor = ht.Focused.TextInput.Cursor.Foreground(t.Accent)
	ht.Focused.TextInput.Placeholder = ht.Focused.TextInput.Placeholder.Foreground(t.Subtle)
	ht.Focused.TextInput.Prompt = ht.Focused.TextInput.Prompt.Foreground(t.Purple)

	ht.Blurred = ht.Focused
	ht.Blurred.Base = ht.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
	ht.Blurred.Card = ht.Blurred.Base
	ht.Blurred.NextIndicator = lipgloss.NewStyle()
	ht.Blurred.PrevIndicator = lipgloss.NewStyle()

	ht.Group.Title = ht.Focused.Title
	ht.Group.Description = ht.Focused.Description

	return ht
}

func themeByName(name string) Theme {
	for _, t := range themes {
		if t.Name == name {
			return t
		}
	}
	return themes[0]
}
