package tui

import (
	"fmt"
	"strings"

	"depscope/internal/model"
	"depscope/internal/render"
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
	brand := m.styles.brandText("DEPSCOPE")
	brandWidth := runeLen("DEPSCOPE")
	if m.color {
		brandWidth += 2
	}
	available := m.width - brandWidth - 2
	if available < 1 {
		available = 1
	}
	title = truncatePlain(title, available)
	gap := m.width - brandWidth - runeLen(title) - 1
	if gap < 1 {
		gap = 1
	}
	line := brand + strings.Repeat(" ", gap) + m.styles.dimText(title)
	rule := m.styles.dimText(strings.Repeat("─", m.width))
	return line + "\n" + rule
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

func (m *Model) menuView() string {
	var b strings.Builder
	b.WriteString(m.styles.boldText("Check dependency risk before you install."))
	b.WriteString("\n")
	b.WriteString(m.styles.dimText("Read-only metadata. Evidence-first results."))
	b.WriteString("\n\n")
	compact := m.contentHeight() < 18
	for index, entry := range menuEntries {
		marker := "  "
		title := entry.title
		if index == m.menuIndex {
			marker = m.styles.accentText("› ")
			title = m.styles.selectedText(title)
		}
		fmt.Fprintf(&b, "%s%d  %s\n", marker, index+1, title)
		if compact {
			continue
		}
		width := m.contentWidth() - 6
		if width < 10 {
			width = 10
		}
		for _, line := range wrapPlain(entry.description, width) {
			b.WriteString("      " + m.styles.dimText(line) + "\n")
		}
	}
	if m.notice != "" {
		b.WriteString("\n" + m.styles.successText(m.notice) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
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
	b.WriteString(m.styles.warningText("It will not install, upgrade, import, or execute any package."))
	b.WriteString("\n\n")
	b.WriteString(m.wrappedDim("Top-level npm global packages and the selected Python interpreter's default environment are covered. Other environments are not."))
	b.WriteString("\n\n")
	b.WriteString(m.styles.accentText("Press enter to start the read-only scan."))
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
		"Default ecosystem   " + string(m.ecosystem),
		"Color               " + m.colorDescription(),
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
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) loadingView() string {
	var b strings.Builder
	b.WriteString(m.styles.boldText(m.loadingName))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim("Real read-only requests are in progress. DepScope does not estimate or fabricate progress."))
	b.WriteString("\n\n")
	if m.cancelling {
		b.WriteString(m.styles.warningText("Cancelling..."))
		b.WriteString("\n")
	}
	stage := strings.ToUpper(strings.TrimSpace(m.loading.Stage))
	if stage == "" {
		stage = "START"
	}
	fmt.Fprintf(&b, "%s  %s\n", m.styles.accentText("["+stage+"]"), m.styles.boldText(sanitizeLine(m.loading.Message)))
	if m.loading.Total > 0 {
		b.WriteString("\n")
		b.WriteString(progressBar(m.loading.Completed, m.loading.Total, m.progressBarWidth()))
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

func (m *Model) helpView() string {
	var b strings.Builder
	b.WriteString(m.styles.boldText("KEYBOARD"))
	b.WriteString("\n")
	sections := [][2]string{
		{"Menus", "↑/↓ or j/k move · enter select · 1-6 jump · q quit"},
		{"Forms", "tab next · enter continue · esc back · ctrl+u clear field"},
		{"Reports", "↑/↓ select findings or scroll · enter opens finding detail"},
		{"Report views", "c checks · d dependencies · i details · s summary · v sources · o overview"},
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
		label = m.styles.dimText(label)
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
