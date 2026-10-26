package render

import (
	"fmt"
	"strings"
)

// ColorMode 控制 ANSI 颜色策略。
type ColorMode string

const (
	// ColorAuto 仅在输出到终端且环境允许颜色时启用颜色。
	ColorAuto ColorMode = "auto"
	// ColorAlways 在环境允许时启用颜色，即使输出被重定向。
	ColorAlways ColorMode = "always"
	// ColorNever 始终禁用颜色。
	ColorNever ColorMode = "never"
)

// ParseColorMode 解析命令行或设置中的颜色策略。
func ParseColorMode(value string) (ColorMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(ColorAuto):
		return ColorAuto, nil
	case string(ColorAlways):
		return ColorAlways, nil
	case string(ColorNever):
		return ColorNever, nil
	default:
		return "", fmt.Errorf("color mode %q must be auto, always, or never", value)
	}
}

// ColorEnabled 决定是否输出 ANSI 颜色。
// NO_COLOR 与 TERM=dumb 始终优先，避免在明确禁用颜色的终端中注入转义序列。
func ColorEnabled(mode ColorMode, outputIsTerminal bool, lookupEnv func(string) string) bool {
	if lookupEnv == nil {
		lookupEnv = func(string) string { return "" }
	}
	if lookupEnv("NO_COLOR") != "" || strings.EqualFold(lookupEnv("TERM"), "dumb") {
		return false
	}
	switch mode {
	case ColorAlways:
		return true
	case ColorNever:
		return false
	default:
		return outputIsTerminal
	}
}
