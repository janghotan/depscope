package render

import "strings"

// Sanitize 移除控制字符与 ANSI 转义序列。
// 注册表与漏洞来源的文本属于不可信输入，进入终端前必须经过此函数。
func Sanitize(value string) string {
	if value == "" {
		return ""
	}
	runes := []rune(value)
	result := make([]rune, 0, len(runes))
	for index := 0; index < len(runes); index++ {
		current := runes[index]
		switch {
		case current == 0x1b:
			index = skipEscapeSequence(runes, index)
		case current == '\n' || current == '\t':
			result = append(result, current)
		case current < 0x20 || current == 0x7f:
			// 丢弃其余 C0 控制字符。
		case current >= 0x80 && current <= 0x9f:
			// 丢弃 C1 控制字符。
		default:
			result = append(result, current)
		}
	}
	return string(result)
}

// SanitizeMultiline 在 Sanitize 基础上压缩连续空行，适合简短的状态文本。
func SanitizeMultiline(value string) string {
	value = strings.TrimSpace(Sanitize(value))
	lines := strings.Split(value, "\n")
	cleaned := make([]string, 0, len(lines))
	previousBlank := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		blank := strings.TrimSpace(line) == ""
		if blank && previousBlank {
			continue
		}
		cleaned = append(cleaned, line)
		previousBlank = blank
	}
	return strings.Join(cleaned, "\n")
}

// skipEscapeSequence 返回当前 ESC 序列最后一个字符的索引。
func skipEscapeSequence(runes []rune, start int) int {
	index := start + 1
	if index >= len(runes) {
		return start
	}
	switch runes[index] {
	case '[':
		index++
		for index < len(runes) {
			if runes[index] >= 0x40 && runes[index] <= 0x7e {
				return index
			}
			index++
		}
		return len(runes) - 1
	case ']':
		index++
		for index < len(runes) {
			if runes[index] == 0x07 {
				return index
			}
			if runes[index] == 0x1b && index+1 < len(runes) && runes[index+1] == '\\' {
				return index + 1
			}
			index++
		}
		return len(runes) - 1
	default:
		return index
	}
}
