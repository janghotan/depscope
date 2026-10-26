// Package termtext 提供终端显示宽度感知的文本辅助函数。
// 中文、日文与韩文字符占据两个单元格，因此换行、截断与补齐都必须按显示宽度
// 而不是 rune 数量计算，才能保证列对齐与右边缘不被裁切。
package termtext

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// Width 返回文本在终端中的显示宽度；ANSI 转义序列不占宽度。
func Width(value string) int {
	return ansi.StringWidth(value)
}

// Truncate 按显示宽度截断文本；尾部标记也会计入宽度。
// width 小于等于零时返回空串，保证调用方不会得到超出宽度的内容。
func Truncate(value string, width int, tail string) string {
	if width <= 0 {
		return ""
	}
	if Width(value) <= width {
		return value
	}
	return ansi.Truncate(value, width, tail)
}

// TruncateLeft 从左侧按显示宽度截断文本，并在开头添加前缀。
func TruncateLeft(value string, width int, prefix string) string {
	if width <= 0 {
		return ""
	}
	return ansi.TruncateLeft(value, width, prefix)
}

// PadRight 用空格把文本补齐到指定显示宽度；已满足宽度时原样返回。
func PadRight(value string, width int) string {
	missing := width - Width(value)
	if missing <= 0 {
		return value
	}
	return value + strings.Repeat(" ", missing)
}

// Wrap 按显示宽度换行，返回不含换行符的文本行。
// 英文按单词边界换行；中文等没有空格的文字按字符边界换行。
// 输入应视为纯文本，样式应在换行后应用，避免 ANSI 序列被切断。
func Wrap(value string, width int) []string {
	if width < 1 {
		width = 1
	}
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := make([]string, 0, 4)
	for _, raw := range strings.Split(value, "\n") {
		raw = strings.TrimRight(raw, " \t")
		if strings.TrimSpace(raw) == "" {
			if raw == "" {
				lines = append(lines, "")
			}
			continue
		}
		lines = append(lines, wrapParagraph(raw, width)...)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// wrapParagraph 把单个段落拆成显示宽度不超过 width 的行。
func wrapParagraph(text string, width int) []string {
	lines := []string{}
	current := ""
	currentWidth := 0
	pendingSpace := false

	flush := func() {
		if current != "" {
			lines = append(lines, current)
			current = ""
			currentWidth = 0
		}
		pendingSpace = false
	}

	appendToken := func(token string, tokenWidth int, spacing bool) {
		if current == "" {
			current = token
			currentWidth = tokenWidth
			pendingSpace = false
			return
		}
		spaceWidth := 0
		if spacing && pendingSpace {
			spaceWidth = 1
		}
		if currentWidth+spaceWidth+tokenWidth <= width {
			if spaceWidth > 0 {
				current += " "
				currentWidth++
			}
			current += token
			currentWidth += tokenWidth
			pendingSpace = false
			return
		}
		flush()
		current = token
		currentWidth = tokenWidth
	}

	for _, token := range tokenize(text) {
		if token == "" {
			continue
		}
		tokenWidth := Width(token)
		if isSpace(token) {
			pendingSpace = true
			continue
		}
		// 单个宽字符超过可用宽度时也必须保留，否则信息会被静默丢弃。
		if tokenWidth <= width {
			appendToken(token, tokenWidth, true)
			continue
		}
		// 超长英文单词按显示宽度硬切，保证每行都有进展。
		for token != "" {
			if current != "" {
				flush()
			}
			chunk, rest := cutWidth(token, width)
			if chunk == "" {
				// 宽度不足一个字符时至少取一个 grapheme，避免死循环。
				runes := []rune(token)
				chunk = string(runes[0])
				rest = string(runes[1:])
			}
			lines = append(lines, chunk)
			token = rest
		}
	}
	flush()
	if len(lines) == 0 {
		lines = append(lines, "")
	}
	return lines
}

// tokenize 把文本拆成可换行的最小单元：空格、单个宽字符或连续窄字符单词。
func tokenize(text string) []string {
	tokens := make([]string, 0, len(text)/2+1)
	var word strings.Builder
	flushWord := func() {
		if word.Len() > 0 {
			tokens = append(tokens, word.String())
			word.Reset()
		}
	}
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			flushWord()
			tokens = append(tokens, " ")
		case Width(string(r)) >= 2:
			flushWord()
			tokens = append(tokens, string(r))
		default:
			word.WriteRune(r)
		}
	}
	flushWord()
	return tokens
}

func isSpace(token string) bool {
	return token == " "
}

// cutWidth 按显示宽度切出前缀，返回前缀与剩余文本。
func cutWidth(value string, width int) (string, string) {
	var head strings.Builder
	used := 0
	runes := []rune(value)
	for index, r := range runes {
		runeWidth := Width(string(r))
		if used+runeWidth > width {
			return head.String(), string(runes[index:])
		}
		head.WriteRune(r)
		used += runeWidth
	}
	return head.String(), ""
}

// CellWindow 返回围绕光标的可见窗口、光标在窗口中的显示列，以及窗口起点。
// 用于单行输入框：长中文输入时保持光标可见，同时不把宽字符切成半个。
func CellWindow(value []rune, cursor, width int) (visible string, cursorCell int, start int) {
	if len(value) == 0 || width < 1 {
		return "", 0, 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(value) {
		cursor = len(value)
	}

	start = 0
	// 光标前方的内容必须留出至少一列给光标本身。
	for start < cursor && cellWidth(value[start:cursor]) >= width {
		start++
	}
	end := start
	used := 0
	for end < len(value) {
		runeWidth := Width(string(value[end]))
		if used+runeWidth > width {
			break
		}
		used += runeWidth
		end++
	}
	if end < cursor {
		end = cursor
	}
	return string(value[start:end]), cellWidth(value[start:cursor]), start
}

func cellWidth(runes []rune) int {
	total := 0
	for _, r := range runes {
		total += Width(string(r))
	}
	return total
}
