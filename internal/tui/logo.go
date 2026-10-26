package tui

import "strings"

// 终端品牌标识：以盾牌表示保护边界，内部的小方块与连线表示依赖图中的节点。
// 所有图形都是纯文本字符，可以直接在任意终端渲染，不使用 SVG 或外部图片。

// brandName 是品牌词标的唯一拼写来源；所有档位都必须由它派生。
const brandName = "DEPSCOPE"

// compactName 是窄终端使用的缩写，不是完整词标，但必须与 brandName 一致。
const compactName = "DS"

// logoSize 控制标识的尺寸档位。
type logoSize int

const (
	// logoCompact 用于窄终端：单行盾牌标记加缩写词标。
	logoCompact logoSize = iota
	// logoInline 用于页眉：单行标记加完整词标。
	logoInline
	// logoFull 用于空间充足的主菜单：多行大标识。
	logoFull
	// logoNone 表示宽度不足，只显示纯文本名称。
	logoNone
)

// shieldMark 是紧凑形式的盾牌标记：外框包裹一个依赖节点。
const shieldMark = "(⬢)"

// letterHeight 是所有块状字形的行数。
const letterHeight = 6

// letterGlyphs 按字母保存块状字形。每个字形内部各行等宽，
// 拼接时按字形宽度补齐，因此逐字拼接后列位始终对齐。
// 词标由 brandName 逐字查表生成，而不是手写整行图案：
// 这样漏写或错序某个字母都会在测试中立刻暴露。
var letterGlyphs = map[rune][]string{
	'D': {"██████╗ ", "██╔══██╗", "██║  ██║", "██║  ██║", "██████╔╝", "╚═════╝ "},
	'E': {"███████╗", "██╔════╝", "█████╗  ", "██╔══╝  ", "███████╗", "╚══════╝"},
	'P': {"██████╗ ", "██╔══██╗", "██████╔╝", "██╔═══╝ ", "██║     ", "╚═╝     "},
	'S': {"███████╗", "██╔════╝", "███████╗", "╚════██║", "███████║", "╚══════╝"},
	'C': {" ██████╗", "██╔════╝", "██║     ", "██║     ", "╚██████╗", " ╚═════╝"},
	'O': {" ██████╗ ", "██╔═══██╗", "██║   ██║", "██║   ██║", "╚██████╔╝", " ╚═════╝ "},
}

// glyphWidth 返回字形占用的列数；不同字形宽窄不同，但同一字形内部各行等宽。
func glyphWidth(glyph []string) int {
	width := 0
	for _, line := range glyph {
		if current := displayWidth(line); current > width {
			width = current
		}
	}
	return width
}

// renderWordmark 按字母逐行拼出块状词标。任何缺失的字形都会被跳过，
// 因此调用方必须先用 wordmarkComplete 校验；渲染函数本身保持纯粹。
func renderWordmark(word string) []string {
	rows := make([]string, letterHeight)
	for _, letter := range word {
		glyph, ok := letterGlyphs[letter]
		if !ok {
			continue
		}
		width := glyphWidth(glyph)
		for row := 0; row < letterHeight; row++ {
			line := ""
			if row < len(glyph) {
				line = glyph[row]
			}
			rows[row] += padPlain(line, width)
		}
	}
	return rows
}

// wordmarkComplete 报告词标中的每个字母都有完整且等宽的字形定义。
func wordmarkComplete(word string) bool {
	for _, letter := range word {
		glyph, ok := letterGlyphs[letter]
		if !ok || len(glyph) != letterHeight {
			return false
		}
		width := glyphWidth(glyph)
		for _, line := range glyph {
			if displayWidth(line) != width {
				return false
			}
		}
	}
	return true
}

// wordmarkWidth 返回给定词标的显示宽度。
func wordmarkWidth(word string) int {
	width := 0
	for _, letter := range word {
		if glyph, ok := letterGlyphs[letter]; ok {
			width += glyphWidth(glyph)
		}
	}
	return width
}

// shieldArt 是大标识中的盾牌轮廓，逐行与词标并排。
var shieldArt = []string{
	"  ╭─────╮  ",
	"  │ ⬢─╮ │  ",
	"  │   ╰─│  ",
	"  │ ╭─⬢ │  ",
	"  ╰─────╯  ",
}

// shieldWidth 返回盾牌轮廓占用的最大列数。
func shieldWidth() int {
	width := 0
	for _, line := range shieldArt {
		if current := displayWidth(line); current > width {
			width = current
		}
	}
	return width
}

// logoFullLines 生成大标识：左侧盾牌与右侧词标逐行并排，行数取两者较大值。
func logoFullLines(st styles) []string {
	word := renderWordmark(brandName)
	rows := len(shieldArt)
	if len(word) > rows {
		rows = len(word)
	}
	shieldColumn := shieldWidth()
	lines := make([]string, 0, rows)
	for index := 0; index < rows; index++ {
		shield := ""
		if index < len(shieldArt) {
			shield = shieldArt[index]
		}
		text := ""
		if index < len(word) {
			text = word[index]
		}
		lines = append(lines, st.brandAccentText(padPlain(shield, shieldColumn))+st.brandWordText(text))
	}
	return lines
}

// logoWidth 返回指定档位需要的终端列数。
func logoWidth(size logoSize) int {
	switch size {
	case logoFull:
		return shieldWidth() + wordmarkWidth(brandName)
	case logoInline:
		return displayWidth(shieldMark) + 1 + displayWidth(brandName)
	case logoCompact:
		return displayWidth(shieldMark) + 1 + displayWidth(compactName)
	default:
		return 0
	}
}

// selectLogoSize 根据可用宽度与高度选择标识档位，保证不会折行或被裁切。
func selectLogoSize(width, height int) logoSize {
	switch {
	// 大标识需要为下方的导航与说明留出空间，因此同时约束高度。
	case width >= logoWidth(logoFull) && height >= letterHeight+4:
		return logoFull
	// 单行标识要求宽度富余，避免与同行的其他信息挤在一起。
	case width >= logoWidth(logoInline)+8:
		return logoInline
	case width >= logoWidth(logoCompact)+4:
		return logoCompact
	default:
		return logoNone
	}
}

// renderLogo 生成品牌块。关闭颜色时不使用任何 ANSI 样式，仅依靠字符形状。
// 返回的多行文本已经过宽度约束，调用方不需要再次截断。
func renderLogo(s styles, size logoSize) string {
	switch size {
	case logoFull:
		return strings.Join(logoFullLines(s), "\n")
	case logoInline:
		return s.brandAccentText(shieldMark) + " " + s.brandWordText(brandName)
	case logoCompact:
		return s.brandAccentText(shieldMark) + " " + s.brandWordText(compactName)
	default:
		return s.brandWordText(brandName)
	}
}

// logoAccessibleText 返回与图形等价的纯文本描述，供无色或辅助场景使用。
// 即使图形档位使用缩写，辅助文本也始终给出完整品牌名称。
func logoAccessibleText(logoSize) string {
	return brandName
}
