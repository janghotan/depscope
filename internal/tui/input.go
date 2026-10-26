package tui

import (
	"strings"
	"unicode"
	"unicode/utf8"
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

func runeLen(value string) int {
	return utf8.RuneCountInString(value)
}

func truncatePlain(value string, width int) string {
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

func padPlain(value string, width int) string {
	count := runeLen(value)
	if count >= width {
		return truncatePlain(value, width)
	}
	return value + strings.Repeat(" ", width-count)
}

func wrapPlain(value string, width int) []string {
	if width < 1 {
		width = 1
	}
	words := strings.Fields(value)
	if len(words) == 0 {
		return []string{""}
	}
	lines := []string{}
	current := ""
	for _, word := range words {
		runes := []rune(word)
		for len(runes) > width {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
			lines = append(lines, string(runes[:width]))
			runes = runes[width:]
		}
		word = string(runes)
		if current == "" {
			current = word
			continue
		}
		if runeLen(current)+1+runeLen(word) <= width {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = word
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
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

// visibleField 截取围绕光标的窗口，避免长输入超出终端宽度。
func visibleField(value []rune, cursor, width int) (string, int, int) {
	if width < 1 {
		width = 1
	}
	if len(value) <= width {
		return string(value), cursor, 0
	}
	start := 0
	if cursor > width-1 {
		start = cursor - (width - 1)
	}
	end := start + width
	if end > len(value) {
		end = len(value)
		start = end - width
		if start < 0 {
			start = 0
		}
	}
	return string(value[start:end]), cursor - start, start
}
