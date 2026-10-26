package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// 布局辅助：把不同宽度下面板的排列方式集中在一处，便于测试与维护。

// visibleWidth 返回文本的终端显示宽度，忽略 ANSI 转义序列。
func visibleWidth(value string) int {
	return lipgloss.Width(value)
}

// padVisible 按可见宽度补齐文本；已满足宽度时原样返回，避免截断样式序列。
func padVisible(value string, width int) string {
	missing := width - visibleWidth(value)
	if missing <= 0 {
		return value
	}
	return value + strings.Repeat(" ", missing)
}

// layoutMode 表示主菜单的排列方式。
type layoutMode int

const (
	// layoutSingle 为单列布局，用于窄终端。
	layoutSingle layoutMode = iota
	// layoutTwoColumn 为双列布局，用于宽度足够时把导航与说明并排。
	layoutTwoColumn
)

// twoColumnMinWidth 是启用双列布局所需的最小内容宽度。
// 左列放导航与操作，右列放说明；两列之间保留间隙。
const (
	twoColumnMinWidth = 74
	columnGap         = 4
	// panelPadding 是面板内边距。
	panelPadding = 2
)

// layoutFor 根据内容宽度与高度决定排列方式。
func layoutFor(width, height int) layoutMode {
	if width >= twoColumnMinWidth && height >= 20 {
		return layoutTwoColumn
	}
	return layoutSingle
}

// columnWidths 计算双列布局下两列的宽度。
func columnWidths(width int) (left, right int) {
	usable := width - columnGap
	if usable < 2 {
		return width, 0
	}
	left = usable * 55 / 100
	right = usable - left
	return left, right
}

// joinColumns 把左右两列按行拼接，缺失的行用空白补齐，保证行宽一致。
// 左侧参数为每一行的渲染结果，已经由调用方保证不超过列宽。
func joinColumns(left, right []string, leftWidth int) []string {
	rows := len(left)
	if len(right) > rows {
		rows = len(right)
	}
	out := make([]string, 0, rows)
	for index := 0; index < rows; index++ {
		leftLine := ""
		if index < len(left) {
			leftLine = left[index]
		}
		rightLine := ""
		if index < len(right) {
			rightLine = right[index]
		}
		if rightLine == "" {
			out = append(out, strings.TrimRight(leftLine, " "))
			continue
		}
		out = append(out, padVisible(leftLine, leftWidth)+strings.Repeat(" ", columnGap)+rightLine)
	}
	return out
}

// padBlock 给每一行加上统一的水平内边距。
func padBlock(lines []string, padding int) []string {
	if padding <= 0 {
		return lines
	}
	prefix := strings.Repeat(" ", padding)
	out := make([]string, len(lines))
	for index, line := range lines {
		if line == "" {
			out[index] = ""
			continue
		}
		out[index] = prefix + line
	}
	return out
}

// wrapBlock 按给定宽度换行，并统一缩进段落，避免描述文字在窄终端溢出。
func wrapBlock(text string, width, indent int) []string {
	if width <= indent {
		width = indent + 1
	}
	lines := wrapPlain(text, width-indent)
	prefix := strings.Repeat(" ", indent)
	out := make([]string, len(lines))
	for index, line := range lines {
		out[index] = prefix + line
	}
	return out
}

// labelValue 生成“标签 + 值”的对齐行，标签固定宽度，保证同一组内数值对齐。
func labelValue(label, value string, labelWidth int) string {
	return padPlain(label, labelWidth) + value
}
