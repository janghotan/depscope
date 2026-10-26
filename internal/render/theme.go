// Package render 将共享报告模型转换为可读的纯文本或带有限 ANSI 样式的终端内容。
// 所有语义都通过显式文字标签表达，颜色只用于强化层级，不是理解风险的必要条件。
package render

import (
	"strings"
	"unicode/utf8"

	"github.com/janghotan/depscope/internal/model"
)

const (
	ansiReset    = "\x1b[0m"
	ansiBold     = "\x1b[1m"
	ansiDim      = "\x1b[38;5;245m"
	ansiAccent   = "\x1b[38;5;114m"
	ansiDanger   = "\x1b[1;38;5;203m"
	ansiHigh     = "\x1b[1;38;5;215m"
	ansiMedium   = "\x1b[38;5;221m"
	ansiLow      = "\x1b[38;5;117m"
	ansiUnknown  = "\x1b[38;5;245m"
	ansiSuccess  = "\x1b[38;5;114m"
	ansiInfo     = "\x1b[38;5;111m"
	minWidth     = 32
	defaultWidth = 80
)

// Theme 控制内容宽度与是否输出 ANSI 颜色。
type Theme struct {
	Color bool
	Width int
}

// Width 返回实际使用的排版宽度。
func (t Theme) WidthForContent() int {
	if t.Width <= 0 {
		return defaultWidth
	}
	if t.Width < minWidth {
		return minWidth
	}
	return t.Width
}

func (t Theme) paint(text string, codes ...string) string {
	if !t.Color || text == "" {
		return text
	}
	return strings.Join(codes, "") + text + ansiReset
}

func (t Theme) bold(text string) string   { return t.paint(text, ansiBold) }
func (t Theme) dim(text string) string    { return t.paint(text, ansiDim) }
func (t Theme) accent(text string) string { return t.paint(text, ansiAccent) }

// StatusLabel 返回扫描状态的显式文字标签。
func StatusLabel(status model.ScanStatus) string {
	switch status {
	case model.StatusPending:
		return "PENDING"
	case model.StatusRunning:
		return "RUNNING"
	case model.StatusClean:
		return "CLEAN"
	case model.StatusFindings:
		return "FINDINGS"
	case model.StatusIncomplete:
		return "INCOMPLETE"
	case model.StatusFailed:
		return "FAILED"
	case model.StatusCancelled:
		return "CANCELLED"
	default:
		return strings.ToUpper(string(status))
	}
}

// SeverityLabel 返回严重性等级的显式文字标签。
func SeverityLabel(severity model.Severity) string {
	switch severity {
	case model.SeverityCritical:
		return "CRITICAL"
	case model.SeverityHigh:
		return "HIGH"
	case model.SeverityMedium:
		return "MEDIUM"
	case model.SeverityLow:
		return "LOW"
	case model.SeverityUnknown:
		return "UNKNOWN"
	default:
		return strings.ToUpper(string(severity))
	}
}

// KindLabel 返回发现类型的显式文字标签。
func KindLabel(kind model.FindingKind) string {
	switch kind {
	case model.FindingConfirmed:
		return "CONFIRMED"
	case model.FindingHeuristic:
		return "HEURISTIC"
	default:
		return strings.ToUpper(string(kind))
	}
}

// CheckLabel 返回检查状态的显式文字标签。
func CheckLabel(status model.CheckStatus) string {
	switch status {
	case model.CheckSucceeded:
		return "OK"
	case model.CheckUnavailable:
		return "UNAVAILABLE"
	case model.CheckSkipped:
		return "SKIPPED"
	case model.CheckFailed:
		return "FAILED"
	default:
		return strings.ToUpper(string(status))
	}
}

// DataModeLabel 返回数据模式的显式文字标签。
func DataModeLabel(mode model.DataMode) string {
	switch mode {
	case model.DataModeDemo:
		return "DEMO"
	case model.DataModeLive:
		return "LIVE"
	default:
		return strings.ToUpper(string(mode))
	}
}

func (t Theme) statusBadge(status model.ScanStatus) string {
	label := "[" + StatusLabel(status) + "]"
	switch status {
	case model.StatusClean:
		return t.paint(label, ansiSuccess)
	case model.StatusFindings:
		return t.paint(label, ansiHigh)
	case model.StatusIncomplete:
		return t.paint(label, ansiMedium)
	case model.StatusFailed, model.StatusCancelled:
		return t.paint(label, ansiDanger)
	default:
		return label
	}
}

func (t Theme) severityBadge(severity model.Severity) string {
	label := "[" + SeverityLabel(severity) + "]"
	switch severity {
	case model.SeverityCritical:
		return t.paint(label, ansiDanger)
	case model.SeverityHigh:
		return t.paint(label, ansiHigh)
	case model.SeverityMedium:
		return t.paint(label, ansiMedium)
	case model.SeverityLow:
		return t.paint(label, ansiLow)
	default:
		return t.paint(label, ansiUnknown)
	}
}

func (t Theme) kindBadge(kind model.FindingKind) string {
	label := "[" + KindLabel(kind) + "]"
	if kind == model.FindingHeuristic {
		return t.paint(label, ansiInfo)
	}
	return t.paint(label, ansiDim)
}

func (t Theme) checkBadge(status model.CheckStatus) string {
	label := "[" + CheckLabel(status) + "]"
	switch status {
	case model.CheckSucceeded:
		return t.paint(label, ansiSuccess)
	case model.CheckUnavailable, model.CheckSkipped:
		return t.paint(label, ansiMedium)
	case model.CheckFailed:
		return t.paint(label, ansiDanger)
	default:
		return label
	}
}

func wrapText(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := []string{}
	for _, raw := range strings.Split(text, "\n") {
		raw = strings.TrimRight(raw, " \t")
		if strings.TrimSpace(raw) == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, wrapLine(raw, width)...)
	}
	return lines
}

func wrapLine(line string, width int) []string {
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}
	result := []string{}
	current := ""
	for _, word := range words {
		for runeCount(word) > width {
			if current != "" {
				result = append(result, current)
				current = ""
			}
			head, tail := splitRunes(word, width)
			result = append(result, head)
			word = tail
		}
		if current == "" {
			current = word
			continue
		}
		if runeCount(current)+1+runeCount(word) <= width {
			current += " " + word
			continue
		}
		result = append(result, current)
		current = word
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func runeCount(value string) int {
	return utf8.RuneCountInString(value)
}

func splitRunes(value string, count int) (string, string) {
	runes := []rune(value)
	if count >= len(runes) {
		return value, ""
	}
	return string(runes[:count]), string(runes[count:])
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

func padRight(value string, width int) string {
	count := runeCount(value)
	if count >= width {
		return truncate(value, width)
	}
	return value + strings.Repeat(" ", width-count)
}

func indentLines(lines []string, prefix string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		result = append(result, prefix+line)
	}
	return result
}

func plural(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
