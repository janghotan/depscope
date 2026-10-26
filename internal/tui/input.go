package tui

import (
	"strings"
	"unicode"

	"github.com/janghotan/depscope/internal/termtext"
)

const marginWidth = 2

// textField 实现无光标转义依赖的单行输入。
type textField struct {
	value       []rune
	cursor      int
	placeholder string
}

func newTextField(placeholder string) textField {
	return textField{placeholder: placeholder}
}

func (f *textField) SetValue(value string) {
	f.value = nil
	for _, r := range value {
		if printable(r) {
			f.value = append(f.value, r)
		}
	}
	f.cursor = len(f.value)
}

func (f textField) String() string {
	return string(f.value)
}

func (f *textField) Insert(runes []rune) {
	cleaned := make([]rune, 0, len(runes))
	for _, r := range runes {
		if printable(r) {
			cleaned = append(cleaned, r)
		}
	}
	if len(cleaned) == 0 {
		return
	}
	next := make([]rune, 0, len(f.value)+len(cleaned))
	next = append(next, f.value[:f.cursor]...)
	next = append(next, cleaned...)
	next = append(next, f.value[f.cursor:]...)
	f.value = next
	f.cursor += len(cleaned)
}

func (f *textField) Backspace() {
	if f.cursor == 0 {
		return
	}
	f.value = append(f.value[:f.cursor-1], f.value[f.cursor:]...)
	f.cursor--
}

func (f *textField) Delete() {
	if f.cursor >= len(f.value) {
		return
	}
	f.value = append(f.value[:f.cursor], f.value[f.cursor+1:]...)
}

func (f *textField) Left() {
	if f.cursor > 0 {
		f.cursor--
	}
}

func (f *textField) Right() {
	if f.cursor < len(f.value) {
		f.cursor++
	}
}

func (f *textField) Home() { f.cursor = 0 }

func (f *textField) End() { f.cursor = len(f.value) }

func (f *textField) Clear() {
	f.value = nil
	f.cursor = 0
}

func printable(r rune) bool {
	return unicode.IsPrint(r) && r != '\n' && r != '\r'
}

// displayWidth 返回终端显示宽度；中文等宽字符占两列，不能按 rune 计数。
func displayWidth(value string) int {
	return termtext.Width(value)
}

func truncatePlain(value string, width int) string {
	return termtext.Truncate(value, width, "…")
}

func padPlain(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return termtext.PadRight(termtext.Truncate(value, width, "…"), width)
}

func wrapPlain(value string, width int) []string {
	return termtext.Wrap(value, width)
}

func indentText(value, prefix string) string {
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		if line != "" {
			lines[index] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// visibleField 按显示宽度截取围绕光标的窗口，避免长中文输入超出终端宽度。
func visibleField(value []rune, cursor, width int) (string, int, int) {
	return termtext.CellWindow(value, cursor, width)
}
