package tui

import "github.com/charmbracelet/lipgloss"

// styles 集中管理界面装饰；关闭颜色时不应用任何 ANSI 样式。
type styles struct {
	color    bool
	brand    lipgloss.Style
	accent   lipgloss.Style
	selected lipgloss.Style
	dim      lipgloss.Style
	bold     lipgloss.Style
	danger   lipgloss.Style
	warning  lipgloss.Style
	success  lipgloss.Style
}

func newStyles(color bool) styles {
	if !color {
		return styles{}
	}
	return styles{
		color:    true,
		brand:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#d9f2e0")).Background(lipgloss.Color("#122b1c")).Padding(0, 1),
		accent:   lipgloss.NewStyle().Foreground(lipgloss.Color("#7ee787")),
		selected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#b7f7c2")),
		dim:      lipgloss.NewStyle().Foreground(lipgloss.Color("#7f9487")),
		bold:     lipgloss.NewStyle().Bold(true),
		danger:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ff7b72")),
		warning:  lipgloss.NewStyle().Foreground(lipgloss.Color("#e3b341")),
		success:  lipgloss.NewStyle().Foreground(lipgloss.Color("#7ee787")),
	}
}

func (s styles) brandText(value string) string {
	if !s.color {
		return value
	}
	return s.brand.Render(value)
}

func (s styles) accentText(value string) string {
	if !s.color {
		return value
	}
	return s.accent.Render(value)
}

func (s styles) selectedText(value string) string {
	if !s.color {
		return value
	}
	return s.selected.Render(value)
}

func (s styles) dimText(value string) string {
	if !s.color {
		return value
	}
	return s.dim.Render(value)
}

func (s styles) boldText(value string) string {
	if !s.color {
		return value
	}
	return s.bold.Render(value)
}

func (s styles) dangerText(value string) string {
	if !s.color {
		return value
	}
	return s.danger.Render(value)
}

func (s styles) warningText(value string) string {
	if !s.color {
		return value
	}
	return s.warning.Render(value)
}

func (s styles) successText(value string) string {
	if !s.color {
		return value
	}
	return s.success.Render(value)
}
