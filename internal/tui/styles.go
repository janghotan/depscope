package tui

import "github.com/charmbracelet/lipgloss"

// 终端配色语义：
//   - 品牌与导航强调色使用翡翠绿；
//   - 上下文与次要信息使用中性灰；
//   - 警告与覆盖不足使用琥珀色；
//   - 高危与严重发现使用红色，且只用于风险，不用于装饰；
//   - 成功与干净结果使用绿色。
//
// 关闭颜色时所有样式都是恒等变换，界面完全依靠文字标签表达状态。
const (
	colorBrand    = "#7ee787"
	colorBrandDim = "#4f9d5c"
	colorInk      = "#d6e2da"
	colorMuted    = "#7f9487"
	colorRule     = "#3c4f44"
	colorWarn     = "#e3b341"
	colorDanger   = "#ff7b72"
	colorSuccess  = "#7ee787"
)

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
	// rule 用于分隔线与边框，属于结构性元素而不是状态色。
	rule lipgloss.Style
	// inverse 用于选中行的高亮背景，无颜色时退化为文字标记。
	inverse lipgloss.Style
	// badge 用于状态徽标。
	badge lipgloss.Style
}

func newStyles(color bool) styles {
	if !color {
		return styles{}
	}
	return styles{
		color:    true,
		brand:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorBrand)),
		accent:   lipgloss.NewStyle().Foreground(lipgloss.Color(colorBrand)),
		selected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorBrand)),
		dim:      lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)),
		bold:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorInk)),
		danger:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorDanger)),
		warning:  lipgloss.NewStyle().Foreground(lipgloss.Color(colorWarn)),
		success:  lipgloss.NewStyle().Foreground(lipgloss.Color(colorSuccess)),
		rule:     lipgloss.NewStyle().Foreground(lipgloss.Color(colorRule)),
		inverse:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#0d1519")).Background(lipgloss.Color(colorBrand)),
		badge:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorBrand)),
	}
}

// render 在启用颜色时应用样式，否则原样返回文本。
func (s styles) render(style lipgloss.Style, value string) string {
	if !s.color {
		return value
	}
	return style.Render(value)
}

func (s styles) brandText(value string) string {
	return s.render(s.brand, value)
}

// brandWordText 用于词标本身，比普通强调更亮。
func (s styles) brandWordText(value string) string {
	return s.render(s.brand, value)
}

// brandAccentText 用于标识图形部分，使用略暗的品牌色避免喧宾夺主。
func (s styles) brandAccentText(value string) string {
	if !s.color {
		return value
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorBrandDim)).Render(value)
}

func (s styles) accentText(value string) string {
	return s.render(s.accent, value)
}

func (s styles) selectedText(value string) string {
	return s.render(s.selected, value)
}

// inverseText 提供高对比的选中行；无颜色时由调用方另外附加箭头标记。
func (s styles) inverseText(value string) string {
	return s.render(s.inverse, value)
}

func (s styles) dimText(value string) string {
	return s.render(s.dim, value)
}

// subtleText 用于标签行等最低层级信息。
func (s styles) subtleText(value string) string {
	return s.render(s.dim, value)
}

func (s styles) boldText(value string) string {
	return s.render(s.bold, value)
}

func (s styles) dangerText(value string) string {
	return s.render(s.danger, value)
}

func (s styles) warningText(value string) string {
	return s.render(s.warning, value)
}

func (s styles) successText(value string) string {
	return s.render(s.success, value)
}

// ruleText 渲染分隔字符；颜色关闭时仍输出字符，保持结构可读。
func (s styles) ruleText(value string) string {
	return s.render(s.rule, value)
}

func (s styles) badgeText(value string) string {
	return s.render(s.badge, value)
}
