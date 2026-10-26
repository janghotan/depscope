package tui

import (
	"fmt"
	"strings"

	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/render"
)

// View 渲染当前界面；颜色关闭时不产生任何 ANSI 转义序列。
func (m *Model) View() string {
	if m.width < minWidth || m.height < minHeight {
		return m.smallTerminalView()
	}
	header := m.header()
	body := m.windowContent(m.bodyText())
	footer := m.styles.dimText(truncatePlain(m.footerHints(), m.width))
	return header + "\n" + body + "\n" + footer
}

func (m *Model) smallTerminalView() string {
	return strings.Join([]string{
		"Terminal too small",
		"",
		fmt.Sprintf("DepScope needs at least %d columns x %d rows.", minWidth, minHeight),
		fmt.Sprintf("Current size: %d x %d.", m.width, m.height),
		"",
		"Resize the window, or press q to quit.",
	}, "\n")
}

func (m *Model) header() string {
	title := m.screenTitle()
	brand := m.headerBrand()
	brandWidth := runeLen(brand)
	available := m.width - brandWidth - 2
	if available < 1 {
		available = 1
	}
	title = truncatePlain(title, available)
	gap := m.width - brandWidth - runeLen(title) - 1
	if gap < 1 {
		gap = 1
	}
	line := brand + strings.Repeat(" ", gap) + m.styles.subtleText(title)
	rule := m.styles.ruleText(strings.Repeat("─", m.width))
	return line + "\n" + rule
}

// headerBrand 生成页眉左侧的品牌文本；窄终端退化为纯文本名称。
func (m *Model) headerBrand() string {
	size := logoInline
	if m.width < logoWidth(logoInline)+24 {
		size = logoCompact
	}
	if m.width < logoWidth(logoCompact)+12 {
		size = logoNone
	}
	return renderLogo(m.styles, size)
}

func (m *Model) screenTitle() string {
	switch m.screen {
	case screenMenu:
		return "Start"
	case screenPackageForm:
		return "Check a single package"
	case screenProjectForm:
		return "Scan my project"
	case screenCompareForm:
		return "Compare two packages"
	case screenInventoryConfirm:
		return "Check my global installs"
	case screenDemoPicker:
		return "Offline demo"
	case screenSettings:
		return "Settings"
	case screenLoading:
		return "Working"
	case screenReport:
		if m.document != nil {
			title := m.document.Title
			if m.document.DataMode == model.DataModeDemo {
				title += " · DEMO"
			}
			return title
		}
		return "Report"
	case screenComparison:
		return "Comparison"
	case screenError:
		return "Error"
	case screenHelp:
		return "Keyboard help"
	default:
		return ""
	}
}

func (m *Model) footerHints() string {
	switch m.screen {
	case screenMenu:
		return "↑/↓ move · enter select · 1-6 jump · ? help · q quit"
	case screenPackageForm, screenProjectForm:
		return "enter scan · esc back · ? is part of the input"
	case screenCompareForm:
		return "enter next/scan · tab switch · esc back"
	case screenInventoryConfirm:
		return "enter start scan · esc back"
	case screenDemoPicker:
		return "↑/↓ choose · enter run · esc back"
	case screenSettings:
		return "↑/↓ choose · enter or ←/→ change · esc back"
	case screenLoading:
		return "esc cancel · ctrl+c quit"
	case screenReport:
		switch m.reportView {
		case viewOverview:
			return "↑/↓ finding · enter details · c checks · d deps · i info · s summary · v sources · esc menu"
		case viewFinding:
			return "↑/↓ finding · esc overview · c/d/i/s/v views · ? keys"
		default:
			return "↑/↓ scroll · esc overview · tab next view · ? keys"
		}
	case screenComparison:
		return "↑/↓ scroll · esc menu · ? keys · q quit"
	case screenError:
		return "esc menu · q quit"
	case screenHelp:
		return "esc back · q quit"
	default:
		return "esc back · q quit"
	}
}

func (m *Model) bodyText() string {
	var content string
	switch m.screen {
	case screenMenu:
		content = m.menuView()
	case screenPackageForm:
		content = m.packageFormView()
	case screenProjectForm:
		content = m.projectFormView()
	case screenCompareForm:
		content = m.compareFormView()
	case screenInventoryConfirm:
		content = m.inventoryView()
	case screenDemoPicker:
		content = m.demoView()
	case screenSettings:
		content = m.settingsView()
	case screenLoading:
		content = m.loadingView()
	case screenReport:
		content = m.reportContent()
	case screenComparison:
		content = m.comparisonView()
	case screenError:
		content = m.errorView()
	case screenHelp:
		content = m.helpView()
	}
	return indentText(content, strings.Repeat(" ", marginWidth))
}

func (m *Model) windowContent(content string) string {
	lines := strings.Split(content, "\n")
	height := m.contentHeight()
	max := len(lines) - height
	if max < 0 {
		max = 0
	}
	if m.scroll > max {
		m.scroll = max
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
	end := m.scroll + height
	if end > len(lines) {
		end = len(lines)
	}
	visible := append([]string(nil), lines[m.scroll:end]...)
	for len(visible) < height {
		visible = append(visible, "")
	}
	return strings.Join(visible, "\n")
}

// menuView 渲染主菜单：品牌、操作导航、上下文说明与只读安全说明。
// 宽度充足时使用双列布局，窄终端退化为单列并压缩描述。
func (m *Model) menuView() string {
	width := m.contentWidth()
	height := m.contentHeight()
	size := selectLogoSize(width, height)
	lines := []string{}

	// 品牌块：大标识只在空间充足时出现，否则用紧凑形式，避免挤压导航。
	if size == logoFull {
		lines = append(lines, strings.Split(m.brandedLogo(size), "\n")...)
		lines = append(lines, "")
	} else {
		lines = append(lines, m.brandedLogo(size))
		lines = append(lines, m.styles.subtleText(tagline))
		lines = append(lines, "")
	}

	layout := layoutFor(width, height)
	if layout == layoutTwoColumn {
		leftWidth, rightWidth := columnWidths(width)
		left := m.menuNavLines(leftWidth)
		right := m.menuContextLines(rightWidth)
		lines = append(lines, joinColumns(left, right, leftWidth)...)
	} else {
		lines = append(lines, m.menuNavLines(width)...)
		// 单列时只保留当前选中项的说明，避免整屏文字。
		lines = append(lines, "")
		lines = append(lines, wrapBlock(menuEntries[m.menuIndex].description, width, 2)...)
		if height >= 22 {
			lines = append(lines, "")
			lines = append(lines, m.safetyLines(width)...)
		}
	}

	if m.notice != "" {
		lines = append(lines, "")
		for _, line := range wrapBlock(m.notice, width, 2) {
			lines = append(lines, m.styles.successText(line))
		}
	}
	return strings.Join(lines, "\n")
}

// brandedLogo 渲染带入场动画的品牌标识；动画结束后完全静态。
func (m *Model) brandedLogo(size logoSize) string {
	if size != logoFull && m.brandVisible() && size == logoInline {
		return m.styles.brandAccentText(shieldMark) + " " + m.styles.brandWordText(m.revealWord("DEPSCOPE"))
	}
	return renderLogo(m.styles, size)
}

// menuNavLines 生成导航列表。选中项同时使用箭头与高亮，无颜色时箭头仍可辨识。
func (m *Model) menuNavLines(width int) []string {
	lines := []string{m.sectionLabel("ACTIONS"), ""}
	for index, entry := range menuEntries {
		selected := index == m.menuIndex
		marker := "  "
		title := entry.title
		shortcut := fmt.Sprintf("%d", index+1)
		if selected {
			marker = m.styles.accentText("› ")
			title = m.styles.selectedText(title)
		} else {
			title = m.styles.boldText(title)
		}
		line := fmt.Sprintf("%s%s  %s", marker, m.styles.subtleText(shortcut), title)
		// 选中行附带焦点指示，保证在无色环境下也一眼可辨。
		if selected && m.anim.focus > 0 && m.anim.enabled {
			line += m.styles.accentText(" ◂")
		}
		lines = append(lines, truncatePlain(line, width))
	}
	return lines
}

// menuContextLines 生成右侧的上下文说明与安全信息。
func (m *Model) menuContextLines(width int) []string {
	lines := []string{m.sectionLabel("SELECTED"), ""}
	lines = append(lines, m.styles.boldText(menuEntries[m.menuIndex].title))
	lines = append(lines, "")
	lines = append(lines, wrapBlock(menuEntries[m.menuIndex].description, width, 0)...)
	lines = append(lines, "")
	lines = append(lines, m.safetyLines(width)...)
	return lines
}

// safetyLines 是只读、证据优先的简短说明；不包含任何统计或健康指标。
func (m *Model) safetyLines(width int) []string {
	lines := []string{m.sectionLabel("HOW IT WORKS"), ""}
	for _, text := range []string{
		"Reads registry metadata and advisories. Never installs or executes packages.",
		"Confirmed advisories and heuristic signals are reported separately.",
		"Missing or failed checks are shown, never counted as clean.",
	} {
		wrapped := wrapBlock(text, width, 0)
		for index, line := range wrapped {
			prefix := "  "
			if index == 0 {
				prefix = m.styles.accentText("· ")
			} else {
				prefix = "  "
			}
			lines = append(lines, prefix+m.styles.subtleText(line))
		}
	}
	return lines
}

// sectionLabel 渲染统一的区块标题，保持全应用一致的层级。
func (m *Model) sectionLabel(text string) string {
	return m.styles.accentText(strings.ToUpper(text))
}

func (m *Model) packageFormView() string {
	var b strings.Builder
	b.WriteString(m.styles.boldText("PACKAGE"))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("One read-only scan. Registry metadata, advisories, lifecycle scripts, and name signals are checked; no package code is executed."))
	b.WriteString("\n\n")
	b.WriteString(m.fieldBlock("Package", 0))
	b.WriteString(m.wrappedDim("Examples: lodash@4.17.21 · pypi:requests==2.31.0 · @scope/pkg"))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("Default ecosystem: " + string(m.ecosystem) + ". Prefix with pypi: for Python packages."))
	b.WriteString(m.formErrorView())
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) projectFormView() string {
	var b strings.Builder
	b.WriteString(m.styles.boldText("PROJECT SCAN"))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("Reads package.json with package-lock.json v2/v3, and exact-pinned requirements.txt. Project code, package code, and install scripts are never executed."))
	b.WriteString("\n\n")
	b.WriteString(m.fieldBlock("Directory", 0))
	b.WriteString(m.wrappedDim("Unsupported formats are reported precisely. Missing or unresolved coverage never produces a clean result."))
	b.WriteString(m.formErrorView())
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) compareFormView() string {
	var b strings.Builder
	b.WriteString(m.styles.boldText("COMPARE PACKAGES"))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("Two independent reports. If one side is unavailable, the other is still shown with its own coverage."))
	b.WriteString("\n\n")
	b.WriteString(m.fieldBlock("Left package", 0))
	b.WriteString("\n")
	b.WriteString(m.fieldBlock("Right package", 1))
	b.WriteString(m.wrappedDim("Tab or shift+tab switches fields · enter moves to the next empty field, then scans."))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("Both sides use the default ecosystem unless a package has an npm: or pypi: prefix."))
	b.WriteString(m.formErrorView())
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) inventoryView() string {
	var b strings.Builder
	b.WriteString(m.styles.boldText("GLOBAL INVENTORY"))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("DepScope will ask npm and pip for their installed package lists, then match exact installed versions against the configured security sources."))
	b.WriteString("\n\n")
	b.WriteString(m.wrappedStyled("It will not install, upgrade, import, or execute any package.", m.styles.warningText))
	b.WriteString("\n\n")
	b.WriteString(m.wrappedDim("Top-level npm global packages and the selected Python interpreter's default environment are covered. Other environments are not."))
	b.WriteString("\n\n")
	b.WriteString(m.wrappedStyled("Press enter to start the read-only scan.", m.styles.accentText))
	b.WriteString("\n")
	return b.String()
}

func (m *Model) demoView() string {
	var b strings.Builder
	b.WriteString(m.styles.boldText("OFFLINE DEMO"))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("Deterministic synthetic reports from the same engine and risk rules. No network requests are made, and every report is marked DEMO DATA."))
	b.WriteString("\n\n")
	if len(m.demos) == 0 {
		b.WriteString(m.styles.dangerText("No demo scenarios are available."))
		return b.String()
	}
	for index, scenario := range m.demos {
		marker := "  "
		title := scenario.Title
		if index == m.demoAt {
			marker = m.styles.accentText("› ")
			title = m.styles.selectedText(title)
		}
		fmt.Fprintf(&b, "%s%d  %s\n", marker, index+1, title)
		width := m.contentWidth() - 6
		if width < 10 {
			width = 10
		}
		for _, line := range wrapPlain(scenario.Description, width) {
			b.WriteString("      " + m.styles.dimText(line) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("The demo never mixes synthetic data into live scans; live and demo scanners use separate data modes."))
	b.WriteString(m.formErrorView())
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) settingsView() string {
	var b strings.Builder
	b.WriteString(m.styles.boldText("SETTINGS"))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("Session-only preferences. Nothing is written to disk."))
	b.WriteString("\n\n")
	rows := []string{
		labelValue("Default ecosystem", string(m.ecosystem), 20),
		labelValue("Color", m.colorDescription(), 20),
		labelValue("Animation", m.animationDescription(), 20),
		"Back",
	}
	for index, row := range rows {
		marker := "  "
		if index == m.settingsAt {
			marker = m.styles.accentText("› ")
			row = m.styles.selectedText(row)
		}
		b.WriteString(marker + row + "\n")
	}
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("NO_COLOR and TERM=dumb always disable color, even when the setting is always. Settings affect new scans and the interface."))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("Animations can also be disabled for a session with DEPSCOPE_NO_ANIMATION=1."))
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) loadingView() string {
	var b strings.Builder
	b.WriteString(m.styles.boldText(m.loadingName))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("Real read-only requests are in progress. DepScope does not estimate or fabricate progress."))
	b.WriteString("\n\n")
	if m.cancelling {
		b.WriteString(m.wrappedStyled("Cancellation requested · waiting for in-flight requests to stop.", m.styles.warningText))
		b.WriteString("\n")
		b.WriteString(m.wrappedDim("No partial result will be shown. Press ctrl+c to quit immediately."))
		b.WriteString("\n")
	}
	stage := strings.ToUpper(strings.TrimSpace(m.loading.Stage))
	if stage == "" {
		stage = "START"
	}
	// 指示字符只表示“正在等待”，不代表完成的百分比；真实进度另行显示。
	glyph := m.styles.accentText(m.spinnerGlyph())
	fmt.Fprintf(&b, "%s %s  %s\n", glyph, m.styles.accentText("["+stage+"]"), m.styles.boldText(sanitizeLine(m.loading.Message)))
	if m.loading.Total > 0 {
		// 只有引擎给出真实完成数与总数时才显示进度条。
		b.WriteString("\n")
		b.WriteString(m.styles.accentText(progressBar(m.loading.Completed, m.loading.Total, m.progressBarWidth())))
		b.WriteString("\n")
	} else {
		b.WriteString("\n")
		b.WriteString(m.wrappedDim("Work in progress; this request does not report a countable total."))
		b.WriteString("\n")
	}
	if len(m.progressLog) > 0 {
		b.WriteString("\n")
		b.WriteString(m.styles.dimText("Recent stages"))
		b.WriteString("\n")
		for _, item := range m.progressLog {
			line := sanitizeLine(item.Stage + " · " + item.Message)
			for _, wrapped := range wrapPlain(line, m.contentWidth()-4) {
				b.WriteString("  " + m.styles.dimText(wrapped) + "\n")
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("Press esc to cancel. Completed requests are read-only and no partial result is presented as a scan."))
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) reportContent() string {
	if m.document == nil {
		return "No report is loaded."
	}
	theme := m.theme()
	switch m.reportView {
	case viewOverview:
		return m.document.Overview(theme, m.findingIndex)
	case viewFinding:
		if len(m.document.Findings) == 0 {
			return m.document.Overview(theme, 0)
		}
		return m.document.FindingDetail(m.findingIndex, theme)
	case viewChecks:
		return m.document.ChecksSection(theme)
	case viewDependencies:
		return m.document.DependenciesSection(theme)
	case viewDetails:
		return m.document.DetailsSection(theme)
	case viewSources:
		return m.document.SourcesSection(theme) + "\n\n" + m.document.LimitationsSection(theme)
	case viewSummary:
		return m.document.CompactSummary(theme)
	default:
		return m.document.Overview(theme, m.findingIndex)
	}
}

func (m *Model) comparisonView() string {
	if m.comparison == nil {
		return "No comparison is loaded."
	}
	return render.Comparison(
		m.comparison.LeftRequest.String(),
		m.comparison.RightRequest.String(),
		m.comparison.Left,
		m.comparison.Right,
		m.comparison.LeftErr,
		m.comparison.RightErr,
		m.theme(),
	)
}

func (m *Model) errorView() string {
	var b strings.Builder
	b.WriteString(m.styles.dangerText("SCAN ERROR"))
	b.WriteString("\n")
	for _, line := range wrapPlain(m.errorText, m.contentWidth()) {
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("No packages were installed, upgraded, or executed, and no partial result is presented as a scan."))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("Return to the menu with esc and try again, or check the package name and network access."))
	return strings.TrimRight(b.String(), "\n")
}

// helpView 说明当前画面可用的按键，并列出全局快捷键。
func (m *Model) helpView() string {
	var b strings.Builder
	b.WriteString(m.styles.boldText("KEYBOARD"))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("The section marked CURRENT applies to the screen you came from."))
	b.WriteString("\n\n")

	current := m.helpContext()
	currentTitle := m.screenName(m.helpReturn)
	b.WriteString(m.wrappedStyled("Current · "+currentTitle, m.styles.accentText))
	b.WriteString("\n")
	for _, line := range wrapPlain(current, m.contentWidth()-2) {
		b.WriteString("  " + m.styles.boldText(line) + "\n")
	}

	sections := [][2]string{
		{"Menus", "↑/↓ or j/k move · enter select · 1-6 jump · esc back · q quit"},
		{"Forms", "tab / shift+tab switch fields · enter continue · esc back · ctrl+u clear field"},
		{"Reports", "↑/↓ select or scroll · enter opens the selected finding · esc overview"},
		{"Report views", "o overview · c checks · d dependencies · i details · s summary · v sources · tab cycles"},
		{"Scrolling", "pgup/pgdn page · home/end jump"},
		{"Any time", "? opens this help · ctrl+c cancels and quits"},
	}
	for _, section := range sections {
		b.WriteString("\n")
		b.WriteString(m.styles.accentText(section[0]))
		b.WriteString("\n")
		for _, line := range wrapPlain(section[1], m.contentWidth()-2) {
			b.WriteString("  " + line + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// helpContext 返回来源画面可用的按键说明。
func (m *Model) helpContext() string {
	switch m.helpReturn {
	case screenPackageForm:
		return "type a package · enter scans · tab is not needed for a single field"
	case screenProjectForm:
		return "type a directory · enter scans · the default is the current directory"
	case screenCompareForm:
		return "tab moves between left and right · enter advances to the empty field, then scans"
	case screenInventoryConfirm:
		return "enter starts the read-only inventory scan"
	case screenDemoPicker:
		return "↑/↓ choose a scenario · enter runs it offline"
	case screenSettings:
		return "↑/↓ choose a row · ←/→ or enter changes the value"
	case screenLoading:
		return "esc requests cancellation · ctrl+c quits"
	case screenReport:
		return "↑/↓ moves through findings · enter opens detail · esc returns to the menu from the overview"
	case screenComparison:
		return "↑/↓ scroll · esc returns to the menu"
	default:
		return "↑/↓ move · enter selects the highlighted action · 1-6 open a workflow directly"
	}
}

// screenName 返回画面的人类可读名称，用于帮助与页眉。
func (m *Model) screenName(value screen) string {
	switch value {
	case screenMenu:
		return "Start"
	case screenPackageForm:
		return "Check a single package"
	case screenProjectForm:
		return "Scan my project"
	case screenCompareForm:
		return "Compare two packages"
	case screenInventoryConfirm:
		return "Check my global installs"
	case screenDemoPicker:
		return "Offline demo"
	case screenSettings:
		return "Settings"
	case screenLoading:
		return "Working"
	case screenReport:
		return "Report"
	case screenComparison:
		return "Comparison"
	case screenError:
		return "Error"
	default:
		return "Help"
	}
}

func (m *Model) fieldBlock(label string, index int) string {
	if index >= len(m.formFields) {
		return ""
	}
	focused := m.formIndex == index
	marker := "  "
	if focused {
		marker = m.styles.accentText("› ")
		label = m.styles.selectedText(label)
	} else {
		label = m.styles.subtleText(label)
	}
	if len(m.formFields) > 1 {
		label = fmt.Sprintf("%s %d/%d", label, index+1, len(m.formFields))
	}
	if focused {
		label += m.styles.subtleText("  editing")
	}
	return marker + label + "\n" + "  " + m.fieldView(m.formFields[index], focused) + "\n"
}

func (m *Model) fieldView(field textField, focused bool) string {
	inner := m.contentWidth() - 7
	if inner < 8 {
		inner = 8
	}
	var content string
	if len(field.value) == 0 {
		placeholder := truncatePlain(field.placeholder, inner-1)
		if focused {
			content = m.styles.accentText("▏") + m.styles.dimText(placeholder)
		} else {
			content = m.styles.dimText(placeholder)
		}
		return "[ " + content + " ]"
	}
	visible, cursor, _ := visibleField(field.value, field.cursor, inner-1)
	runes := []rune(visible)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	before := string(runes[:cursor])
	after := string(runes[cursor:])
	if focused {
		content = m.styles.boldText(before) + m.styles.accentText("▏") + m.styles.boldText(after)
	} else {
		content = visible
	}
	return "[ " + content + " ]"
}

func (m *Model) formErrorView() string {
	if strings.TrimSpace(m.formError) == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n")
	for _, line := range wrapPlain("! "+m.formError, m.contentWidth()-2) {
		b.WriteString(m.styles.dangerText(line))
		b.WriteString("\n")
	}
	return b.String()
}

// wrappedStyled 按当前内容宽度换行后用指定样式渲染每一行，
// 避免带颜色的长句在窄终端中被终端自动折行而破坏布局。
func (m *Model) wrappedStyled(text string, style func(string) string) string {
	var b strings.Builder
	for index, line := range wrapPlain(text, m.contentWidth()) {
		if index > 0 {
			b.WriteString("\n")
		}
		b.WriteString(style(line))
	}
	return b.String()
}

func (m *Model) wrappedDim(text string) string {
	var b strings.Builder
	for index, line := range wrapPlain(text, m.contentWidth()) {
		if index > 0 {
			b.WriteString("\n")
		}
		b.WriteString(m.styles.dimText(line))
	}
	return b.String()
}

func (m *Model) progressBarWidth() int {
	width := m.contentWidth() - 16
	if width > 28 {
		width = 28
	}
	if width < 10 {
		width = 10
	}
	return width
}

func progressBar(completed, total, width int) string {
	if total <= 0 {
		return ""
	}
	if width < 4 {
		width = 4
	}
	filled := completed * width / total
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	if completed > total {
		completed = total
	}
	return fmt.Sprintf("[%s%s] %d/%d", strings.Repeat("█", filled), strings.Repeat("░", width-filled), completed, total)
}

// sanitizeLine 清理单行状态文本，避免远程消息注入终端控制序列。
func sanitizeLine(value string) string {
	return render.Sanitize(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " "))
}
