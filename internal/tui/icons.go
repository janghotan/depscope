package tui

import (
	"strings"

	"github.com/janghotan/depscope/internal/render"
)

// 图标只用于强化语义，不替代文字标签，也不参与风险判断。
// 所有字形都集中在这里，并按终端显示单元格测量；字体缺失或终端不支持时
// 可以退化为纯 ASCII，绝不依赖 Nerd Font 或 emoji 字体。

// glyphPair 保存一个语义图标的 Unicode 字形与 ASCII 回退字形。
type glyphPair struct {
	unicode string
	ascii   string
}

// iconToken 是界面使用的语义图标标识；界面代码只引用这些常量。
type iconToken int

const (
	iconHome iconToken = iota
	iconPackage
	iconProject
	iconCompare
	iconInventory
	iconDemo
	iconSettings
	iconLanguage
	iconDependency
	iconConfirmed
	iconHeuristic
	iconCritical
	iconHigh
	iconMedium
	iconLow
	iconUnknown
	iconWarning
	iconClean
	iconCheckOK
	iconCheckUnavailable
	iconCheckSkipped
	iconCheckFailed
	iconSource
	iconAction
	iconLoading
	iconError
	iconKeyboard
	iconColor
	iconAnimation
	iconCount
)

// iconGlyphs 按 iconToken 的顺序定义所有字形。
// 新增语义图标时必须同时提供 Unicode 与 ASCII 两个版本，并让测试检查宽度。
var iconGlyphs = [...]glyphPair{
	iconHome:             {unicode: "⌂", ascii: "#"},
	iconPackage:          {unicode: "◈", ascii: "o"},
	iconProject:          {unicode: "▤", ascii: "f"},
	iconCompare:          {unicode: "⇄", ascii: "<>"},
	iconInventory:        {unicode: "☰", ascii: "="},
	iconDemo:             {unicode: "▷", ascii: ">"},
	iconSettings:         {unicode: "⚙", ascii: "*"},
	iconLanguage:         {unicode: "文", ascii: "A"},
	iconDependency:       {unicode: "❖", ascii: "*"},
	iconConfirmed:        {unicode: "●", ascii: "!"},
	iconHeuristic:        {unicode: "◐", ascii: "?"},
	iconCritical:         {unicode: "▲", ascii: "!"},
	iconHigh:             {unicode: "△", ascii: "!"},
	iconMedium:           {unicode: "▽", ascii: "!"},
	iconLow:              {unicode: "▿", ascii: "."},
	iconUnknown:          {unicode: "○", ascii: "?"},
	iconWarning:          {unicode: "!", ascii: "!"},
	iconClean:            {unicode: "✓", ascii: "+"},
	iconCheckOK:          {unicode: "✓", ascii: "+"},
	iconCheckUnavailable: {unicode: "○", ascii: "-"},
	iconCheckSkipped:     {unicode: "–", ascii: "-"},
	iconCheckFailed:      {unicode: "✕", ascii: "x"},
	iconSource:           {unicode: "⌕", ascii: "s"},
	iconAction:           {unicode: "→", ascii: ">"},
	iconLoading:          {unicode: "◌", ascii: "~"},
	iconError:            {unicode: "✕", ascii: "x"},
	iconKeyboard:         {unicode: "⌨", ascii: "?"},
	iconColor:            {unicode: "◑", ascii: "c"},
	iconAnimation:        {unicode: "↻", ascii: "~"},
}

// iconMode 记录图标是否启用以及是否使用 ASCII 回退。
type iconMode struct {
	enabled bool
	ascii   bool
}

// glyph 返回当前模式下的字形；图标关闭时返回空串，调用方不需要分支。
func (m iconMode) glyph(token iconToken) string {
	if !m.enabled || token < 0 || int(token) >= len(iconGlyphs) {
		return ""
	}
	if m.ascii {
		return iconGlyphs[token].ascii
	}
	return iconGlyphs[token].unicode
}

// prefix 返回带尾随空格的图标，便于直接拼在标签之前；关闭时为空串。
func (m iconMode) prefix(token iconToken) string {
	glyph := m.glyph(token)
	if glyph == "" {
		return ""
	}
	return glyph + " "
}

// resolveIconMode 按显式设置、环境变量与终端能力决定图标模式。
// 显式 on/ascii/off 优先；否则 DEPSCOPE_NO_ICONS 关闭图标，
// DEPSCOPE_ASCII_ICONS、TERM=dumb 或非 UTF-8 locale 使用 ASCII 回退。
func resolveIconMode(explicit string, lookupEnv func(string) string) iconMode {
	switch strings.ToLower(strings.TrimSpace(explicit)) {
	case "off", "never", "false", "no", "0":
		return iconMode{}
	case "ascii":
		return iconMode{enabled: true, ascii: true}
	case "on", "always", "true", "yes", "1":
		return iconMode{enabled: true}
	}
	if lookupEnv == nil {
		return iconMode{enabled: true}
	}
	if _, present := envPresent(lookupEnv, "DEPSCOPE_NO_ICONS"); present {
		return iconMode{}
	}
	ascii := false
	if _, present := envPresent(lookupEnv, "DEPSCOPE_ASCII_ICONS"); present {
		ascii = true
	}
	if strings.EqualFold(strings.TrimSpace(lookupEnv("TERM")), "dumb") {
		ascii = true
	}
	if !localeUsesUTF8(lookupEnv) {
		ascii = true
	}
	return iconMode{enabled: true, ascii: ascii}
}

// localeUsesUTF8 判断当前 locale 是否声明了 UTF-8；未设置时按 UTF-8 处理。
func localeUsesUTF8(lookupEnv func(string) string) bool {
	for _, name := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		value := strings.TrimSpace(lookupEnv(name))
		if value == "" {
			continue
		}
		upper := strings.ToUpper(value)
		return strings.Contains(upper, "UTF-8") || strings.Contains(upper, "UTF8")
	}
	return true
}

// reportIcons 把界面图标映射为共享渲染主题的可选图标集合。
// 关闭图标时返回零值，报告输出与 CLI 完全一致。
func (m *Model) reportIcons() render.IconSet {
	if !m.icons.enabled {
		return render.IconSet{}
	}
	return render.IconSet{
		Demo:             m.icons.glyph(iconDemo),
		Confirmed:        m.icons.glyph(iconConfirmed),
		Heuristic:        m.icons.glyph(iconHeuristic),
		Critical:         m.icons.glyph(iconCritical),
		High:             m.icons.glyph(iconHigh),
		Medium:           m.icons.glyph(iconMedium),
		Low:              m.icons.glyph(iconLow),
		Unknown:          m.icons.glyph(iconUnknown),
		Clean:            m.icons.glyph(iconClean),
		Warning:          m.icons.glyph(iconWarning),
		Error:            m.icons.glyph(iconError),
		CheckOK:          m.icons.glyph(iconCheckOK),
		CheckUnavailable: m.icons.glyph(iconCheckUnavailable),
		CheckSkipped:     m.icons.glyph(iconCheckSkipped),
		CheckFailed:      m.icons.glyph(iconCheckFailed),
		Source:           m.icons.glyph(iconSource),
		Dependency:       m.icons.glyph(iconDependency),
		Action:           m.icons.glyph(iconAction),
	}
}
