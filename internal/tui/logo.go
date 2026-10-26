package tui

import "strings"

// 终端品牌标识：以盾牌表示保护边界，内部的小方块与连线表示依赖图中的节点。
// 所有图形都是纯文本字符，可以直接在任意终端渲染，不使用 SVG 或外部图片。

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

// logoGlyphs 汇总各档位使用的图形片段。
type logoGlyphs struct {
	shield string
	word   string
	tag    string
}

// shieldMark 是紧凑形式的盾牌标记：外框包裹一个依赖节点。
const shieldMark = "(⬢)"

// logoArt 是大尺寸标识的逐行文本；左侧是盾牌轮廓，右侧是词标。
var logoArt = []struct{ shield, text string }{
	{"  ╭─────╮  ", "██████╗ ███████╗██████╗ ███████╗ ██████╗ ██████╗ ███████╗"},
	{"  │ ⬢─╮ │  ", "██╔══██╗██╔════╝██╔══██╗██╔════╝██╔════╝██╔═══██╗██╔════╝"},
	{"  │   ╰─│  ", "██║  ██║█████╗  ██████╔╝███████╗██║     ██║   ██║█████╗  "},
	{"  │ ╭─⬢ │  ", "██║  ██║██╔══╝  ██╔═══╝ ╚════██║██║     ██║   ██║██╔══╝  "},
	{"  ╰─────╯  ", "██████╔╝███████╗██║     ███████║╚██████╗╚██████╔╝███████╗"},
	{"           ", "╚═════╝ ╚══════╝╚═╝     ╚══════╝ ╚═════╝ ╚═════╝ ╚══════╝"},
}

// tagline 是一行产品定位；主菜单会显示它。
const tagline = "Dependency risk, explained."

// logoWidth 返回指定档位需要的终端列数。
func logoWidth(size logoSize) int {
	switch size {
	case logoFull:
		widest := 0
		for _, line := range logoArt {
			if width := runeLen(line.shield) + runeLen(line.text); width > widest {
				widest = width
			}
		}
		return widest
	case logoInline:
		return runeLen(shieldMark) + 1 + runeLen("DEPSCOPE")
	case logoCompact:
		return runeLen(shieldMark) + 1 + runeLen("DS")
	default:
		return 0
	}
}

// selectLogoSize 根据可用宽度与高度选择标识档位，保证不会折行或被裁切。
func selectLogoSize(width, height int) logoSize {
	switch {
	// 大标识需要为下方的导航与说明留出空间，因此同时约束高度。
	case width >= logoWidth(logoFull) && height >= len(logoArt)+4:
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

// renderLogo 生成品牌块。monochrome 为真时不使用任何颜色，仅依靠字符形状。
// 返回的多行文本已经过宽度约束，调用方不需要再次截断。
func renderLogo(s styles, size logoSize) string {
	switch size {
	case logoFull:
		lines := make([]string, 0, len(logoArt)+2)
		for _, line := range logoArt {
			shield := s.brandAccentText(line.shield)
			word := s.brandWordText(line.text)
			lines = append(lines, shield+word)
		}
		lines = append(lines, "", s.subtleText(tagline))
		return strings.Join(lines, "\n")
	case logoInline:
		return s.brandAccentText(shieldMark) + " " + s.brandWordText("DEPSCOPE")
	case logoCompact:
		return s.brandAccentText(shieldMark) + " " + s.brandWordText("DS")
	default:
		return s.brandWordText("DEPSCOPE")
	}
}

// logoAccessibleText 返回与图形等价的纯文本描述，供无色或辅助场景使用。
func logoAccessibleText(size logoSize) string {
	if size == logoFull || size == logoInline || size == logoCompact {
		return "DepScope"
	}
	return "DEPSCOPE"
}
