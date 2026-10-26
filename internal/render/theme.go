// Package render 将共享报告模型转换为可读的纯文本或带有限 ANSI 样式的终端内容。
// 所有语义都通过显式文字标签表达，颜色只用于强化层级，不是理解风险的必要条件。
package render

import (
	"strings"

	"github.com/janghotan/depscope/internal/i18n"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/termtext"
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

// Theme 控制内容宽度、是否输出 ANSI 颜色与展示语言。
// Language 留空时按英语处理，保证 CLI 与机器输出的默认行为不变。
type Theme struct {
	Color    bool
	Width    int
	Language i18n.Language
}

// Lang 返回归一化后的展示语言。
func (t Theme) Lang() i18n.Language {
	return i18n.Normalize(t.Language)
}

func (t Theme) text(key i18n.Key) string {
	return i18n.For(t.Language).Text(key)
}

func (t Theme) format(key i18n.Key, args ...any) string {
	return i18n.For(t.Language).Format(key, args...)
}

// WidthForContent 返回实际使用的排版宽度。
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

// StatusLabel 返回英语扫描状态标签；本地化界面使用主题方法。
func StatusLabel(status model.ScanStatus) string {
	return Theme{}.statusLabel(status)
}

// SeverityLabel 返回英语严重性等级标签。
func SeverityLabel(severity model.Severity) string {
	return Theme{}.severityLabel(severity)
}

// KindLabel 返回英语发现类型标签。
func KindLabel(kind model.FindingKind) string {
	return Theme{}.kindLabel(kind)
}

// CheckLabel 返回英语检查状态标签。
func CheckLabel(status model.CheckStatus) string {
	return Theme{}.checkLabel(status)
}

// DataModeLabel 返回英语数据模式标签。
func DataModeLabel(mode model.DataMode) string {
	return Theme{}.dataModeLabel(mode)
}

func (t Theme) statusLabel(status model.ScanStatus) string {
	switch status {
	case model.StatusPending:
		return t.text(i18n.KeyStatusPending)
	case model.StatusRunning:
		return t.text(i18n.KeyStatusRunning)
	case model.StatusClean:
		return t.text(i18n.KeyStatusClean)
	case model.StatusFindings:
		return t.text(i18n.KeyStatusFindings)
	case model.StatusIncomplete:
		return t.text(i18n.KeyStatusIncomplete)
	case model.StatusFailed:
		return t.text(i18n.KeyStatusFailed)
	case model.StatusCancelled:
		return t.text(i18n.KeyStatusCancelled)
	default:
		return strings.ToUpper(string(status))
	}
}

func (t Theme) severityLabel(severity model.Severity) string {
	switch severity {
	case model.SeverityCritical:
		return t.text(i18n.KeySeverityCritical)
	case model.SeverityHigh:
		return t.text(i18n.KeySeverityHigh)
	case model.SeverityMedium:
		return t.text(i18n.KeySeverityMedium)
	case model.SeverityLow:
		return t.text(i18n.KeySeverityLow)
	case model.SeverityUnknown:
		return t.text(i18n.KeySeverityUnknown)
	default:
		return strings.ToUpper(string(severity))
	}
}

func (t Theme) kindLabel(kind model.FindingKind) string {
	switch kind {
	case model.FindingConfirmed:
		return t.text(i18n.KeyKindConfirmed)
	case model.FindingHeuristic:
		return t.text(i18n.KeyKindHeuristic)
	default:
		return strings.ToUpper(string(kind))
	}
}

func (t Theme) checkLabel(status model.CheckStatus) string {
	switch status {
	case model.CheckSucceeded:
		return t.text(i18n.KeyCheckOK)
	case model.CheckUnavailable:
		return t.text(i18n.KeyCheckUnavailable)
	case model.CheckSkipped:
		return t.text(i18n.KeyCheckSkipped)
	case model.CheckFailed:
		return t.text(i18n.KeyCheckFailed)
	default:
		return strings.ToUpper(string(status))
	}
}

func (t Theme) dataModeLabel(mode model.DataMode) string {
	switch mode {
	case model.DataModeDemo:
		return t.text(i18n.KeyDataModeDemo)
	case model.DataModeLive:
		return t.text(i18n.KeyDataModeLive)
	default:
		return strings.ToUpper(string(mode))
	}
}

func (t Theme) statusBadge(status model.ScanStatus) string {
	label := "[" + t.statusLabel(status) + "]"
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
	label := "[" + t.severityLabel(severity) + "]"
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
	label := "[" + t.kindLabel(kind) + "]"
	if kind == model.FindingHeuristic {
		return t.paint(label, ansiInfo)
	}
	return t.paint(label, ansiDim)
}

func (t Theme) checkBadge(status model.CheckStatus) string {
	label := "[" + t.checkLabel(status) + "]"
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

// wrapText 按显示宽度换行，中文与英文都按终端单元格计算。
func wrapText(text string, width int) []string {
	return termtext.Wrap(text, width)
}

// truncate 按显示宽度截断并附加省略号。
func truncate(value string, width int) string {
	return termtext.Truncate(value, width, "…")
}

// padRight 按显示宽度补齐；内容过宽时先截断，保证不会破坏列布局。
func padRight(value string, width int) string {
	return termtext.PadRight(truncate(value, width), width)
}
