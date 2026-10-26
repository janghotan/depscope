package tui

import (
	"fmt"
	"strings"

	"github.com/janghotan/depscope/internal/i18n"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/render"
	"github.com/janghotan/depscope/internal/termtext"
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
		m.text(i18n.KeySmallTerminalTitle),
		"",
		m.format(i18n.KeySmallTerminalNeeds, minWidth, minHeight),
		m.format(i18n.KeySmallTerminalCurrent, m.width, m.height),
		"",
		m.text(i18n.KeySmallTerminalResize),
	}, "\n")
}

func (m *Model) header() string {
	title := m.screenTitle()
	brand := m.headerBrand()
	brandWidth := visibleWidth(brand)
	available := m.width - brandWidth - 2
	if available < 1 {
		available = 1
	}
	title = truncatePlain(title, available)
	gap := m.width - brandWidth - displayWidth(title) - 1
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
		return m.text(i18n.KeyScreenStart)
	case screenPackageForm:
		return m.text(i18n.KeyScreenPackageForm)
	case screenProjectForm:
		return m.text(i18n.KeyScreenProjectForm)
	case screenCompareForm:
		return m.text(i18n.KeyScreenCompareForm)
	case screenInventoryConfirm:
		return m.text(i18n.KeyScreenInventory)
	case screenDemoPicker:
		return m.text(i18n.KeyScreenDemo)
	case screenSettings:
		return m.text(i18n.KeyScreenSettings)
	case screenLoading:
		return m.text(i18n.KeyScreenWorking)
	case screenReport:
		if m.document != nil {
			title := m.document.Title
			if m.document.DataMode == model.DataModeDemo {
				title += m.text(i18n.KeyScreenDemoSuffix)
			}
			return title
		}
		return m.text(i18n.KeyScreenReport)
	case screenComparison:
		return m.text(i18n.KeyScreenComparison)
	case screenError:
		return m.text(i18n.KeyScreenError)
	case screenHelp:
		return m.text(i18n.KeyScreenHelp)
	default:
		return ""
	}
}

func (m *Model) footerHints() string {
	switch m.screen {
	case screenMenu:
		return m.text(i18n.KeyFooterMenu)
	case screenPackageForm, screenProjectForm:
		return m.text(i18n.KeyFooterForm)
	case screenCompareForm:
		return m.text(i18n.KeyFooterCompareForm)
	case screenInventoryConfirm:
		return m.text(i18n.KeyFooterInventory)
	case screenDemoPicker:
		return m.text(i18n.KeyFooterDemo)
	case screenSettings:
		return m.text(i18n.KeyFooterSettings)
	case screenLoading:
		return m.text(i18n.KeyFooterLoading)
	case screenReport:
		switch m.reportView {
		case viewOverview:
			return m.text(i18n.KeyFooterReportOverview)
		case viewFinding:
			return m.text(i18n.KeyFooterReportFinding)
		default:
			return m.text(i18n.KeyFooterReportScroll)
		}
	case screenComparison:
		return m.text(i18n.KeyFooterComparison)
	case screenError:
		return m.text(i18n.KeyFooterError)
	case screenHelp:
		return m.text(i18n.KeyFooterHelp)
	default:
		return m.text(i18n.KeyFooterDefault)
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
	// 本地化标签行统一放在标识下方，所有尺寸都保留产品定位说明。
	lines = append(lines, strings.Split(m.brandedLogo(size), "\n")...)
	lines = append(lines, m.styles.subtleText(m.text(i18n.KeyBrandTagline)), "")

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
		lines = append(lines, wrapBlock(m.text(menuEntries[m.menuIndex].descriptionKey), width, 2)...)
		if height >= 22 {
			lines = append(lines, "")
			lines = append(lines, m.safetyLines(width)...)
		}
	}

	if notice := m.noticeText(); notice != "" {
		lines = append(lines, "")
		prefix := "  " + m.icons.prefix(iconError)
		for _, line := range wrapPlain(notice, width-displayWidth(prefix)) {
			lines = append(lines, m.styles.warningText(prefix+line))
		}
	}
	return strings.Join(lines, "\n")
}

// brandedLogo 渲染带入场动画的品牌标识；动画结束后完全静态。
func (m *Model) brandedLogo(size logoSize) string {
	if size != logoFull && m.brandVisible() && size == logoInline {
		return m.styles.brandAccentText(shieldMark) + " " + m.styles.brandWordText(m.revealWord(brandName))
	}
	return renderLogo(m.styles, size)
}

// menuNavLines 生成导航列表。选中项同时使用箭头与高亮，无颜色时箭头仍可辨识。
func (m *Model) menuNavLines(width int) []string {
	lines := []string{m.sectionLabel(m.text(i18n.KeyMenuActions)), ""}
	for index, entry := range menuEntries {
		selected := index == m.menuIndex
		marker := "  "
		if selected {
			marker = "› "
		}
		shortcut := fmt.Sprintf("%d", index+1)
		icon := m.icons.prefix(entry.icon)
		focus := ""
		if selected && m.anim.focus > 0 && m.anim.enabled {
			focus = " ◂"
		}
		// 先按可见文本截断标题，再应用样式；否则 ANSI 序列会被计入宽度并截掉标题。
		titleWidth := width - displayWidth(marker) - displayWidth(shortcut) - 2 - displayWidth(icon) - displayWidth(focus)
		title := truncatePlain(m.text(entry.titleKey), titleWidth)
		styledIcon := ""
		if icon != "" {
			if selected {
				styledIcon = m.styles.accentText(icon)
			} else {
				styledIcon = m.styles.subtleText(icon)
			}
		}
		if selected {
			marker = m.styles.accentText(marker)
			title = m.styles.selectedText(title)
		} else {
			title = m.styles.boldText(title)
		}
		line := marker + m.styles.subtleText(shortcut) + "  " + styledIcon + title
		// 选中行附带焦点指示，保证在无色环境下也一眼可辨。
		if focus != "" {
			line += m.styles.accentText(focus)
		}
		lines = append(lines, line)
	}
	return lines
}

// menuContextLines 生成右侧的上下文说明与安全信息。
func (m *Model) menuContextLines(width int) []string {
	lines := []string{m.sectionLabel(m.text(i18n.KeyMenuSelected)), ""}
	title := m.styles.boldText(m.text(menuEntries[m.menuIndex].titleKey))
	if icon := m.icons.glyph(menuEntries[m.menuIndex].icon); icon != "" {
		title = m.styles.accentText(icon) + " " + title
	}
	lines = append(lines, title)
	lines = append(lines, "")
	lines = append(lines, wrapBlock(m.text(menuEntries[m.menuIndex].descriptionKey), width, 0)...)
	lines = append(lines, "")
	lines = append(lines, m.safetyLines(width)...)
	return lines
}

// safetyLines 是只读、证据优先的简短说明；不包含任何统计或健康指标。
func (m *Model) safetyLines(width int) []string {
	lines := []string{m.sectionLabel(m.text(i18n.KeyMenuHowItWorks)), ""}
	for _, text := range []string{
		m.text(i18n.KeyMenuSafetyRegistry),
		m.text(i18n.KeyMenuSafetySeparate),
		m.text(i18n.KeyMenuSafetyIncomplete),
	} {
		// 项目符号与续行缩进都占两列，先扣除后再换行，避免右边缘被裁切。
		wrapped := wrapBlock(text, width-2, 0)
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

// titleWithIcon 在粗体标题前附加可选强调图标；关闭图标时只返回原标题。
func (m *Model) titleWithIcon(token iconToken, text string) string {
	title := m.styles.boldText(text)
	if icon := m.icons.glyph(token); icon != "" {
		title = m.styles.accentText(icon) + " " + title
	}
	return title
}

// dangerTitleWithIcon 用于错误标题，图标与文字共享危险语义色。
func (m *Model) dangerTitleWithIcon(token iconToken, text string) string {
	title := m.styles.dangerText(text)
	if icon := m.icons.glyph(token); icon != "" {
		title = m.styles.dangerText(icon) + " " + title
	}
	return title
}

func (m *Model) packageFormView() string {
	var b strings.Builder
	b.WriteString(m.titleWithIcon(iconPackage, m.text(i18n.KeyFormPackageTitle)))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeyFormPackageIntro)))
	b.WriteString("\n\n")
	b.WriteString(m.fieldBlock(m.text(i18n.KeyFormPackageLabel), 0))
	b.WriteString(m.wrappedDim(m.text(i18n.KeyFormPackageExamples)))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.format(i18n.KeyFormDefaultEcosystem, m.ecosystem)))
	b.WriteString(m.formErrorView())
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) projectFormView() string {
	var b strings.Builder
	b.WriteString(m.titleWithIcon(iconProject, m.text(i18n.KeyFormProjectTitle)))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeyFormProjectIntro)))
	b.WriteString("\n\n")
	b.WriteString(m.fieldBlock(m.text(i18n.KeyFormProjectLabel), 0))
	b.WriteString(m.wrappedDim(m.text(i18n.KeyFormProjectHint)))
	b.WriteString(m.formErrorView())
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) compareFormView() string {
	var b strings.Builder
	b.WriteString(m.titleWithIcon(iconCompare, m.text(i18n.KeyFormCompareTitle)))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeyFormCompareIntro)))
	b.WriteString("\n\n")
	b.WriteString(m.fieldBlock(m.text(i18n.KeyFormCompareLeft), 0))
	b.WriteString("\n")
	b.WriteString(m.fieldBlock(m.text(i18n.KeyFormCompareRight), 1))
	b.WriteString(m.wrappedDim(m.text(i18n.KeyFormCompareHint)))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeyFormCompareEcosystem)))
	b.WriteString(m.formErrorView())
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) inventoryView() string {
	var b strings.Builder
	b.WriteString(m.titleWithIcon(iconInventory, m.text(i18n.KeyInventoryTitle)))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeyInventoryIntro)))
	b.WriteString("\n\n")
	b.WriteString(m.wrappedPrefixed(iconWarning, m.text(i18n.KeyInventoryNoExecute), m.styles.warningText))
	b.WriteString("\n\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeyInventoryScope)))
	b.WriteString("\n\n")
	b.WriteString(m.wrappedStyled(m.text(i18n.KeyInventoryPressEnter), m.styles.accentText))
	b.WriteString("\n")
	return b.String()
}

func (m *Model) demoView() string {
	var b strings.Builder
	b.WriteString(m.titleWithIcon(iconDemo, m.text(i18n.KeyDemoTitle)))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeyDemoIntro)))
	b.WriteString("\n\n")
	if len(m.demos) == 0 {
		b.WriteString(m.styles.dangerText(m.text(i18n.KeyDemoEmpty)))
		return b.String()
	}
	for index, scenario := range m.demos {
		marker := "  "
		title := m.demoTitle(scenario)
		if index == m.demoAt {
			marker = m.styles.accentText("› ")
			title = m.styles.selectedText(title)
		}
		iconText := m.demoIconText(scenario.ID)
		// 窄终端下按显示宽度截断标题，避免图标把标题推出右边缘。
		titleWidth := m.contentWidth() - displayWidth(marker) - 1 - 2 - displayWidth(iconText)
		if titleWidth < 1 {
			titleWidth = 1
		}
		fmt.Fprintf(&b, "%s%d  %s%s\n", marker, index+1, iconText, truncatePlain(title, titleWidth))
		indent := strings.Repeat(" ", 6+displayWidth(iconText))
		for _, line := range wrapPlain(m.demoDescription(scenario), m.contentWidth()-displayWidth(indent)) {
			b.WriteString(indent + m.styles.dimText(line) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeyDemoNote)))
	b.WriteString(m.formErrorView())
	return strings.TrimRight(b.String(), "\n")
}

// demoScenarioIcon 把演示场景映射为稳定语义图标；未知场景按风险场景处理。
func demoScenarioIcon(id string) iconToken {
	switch id {
	case "clean":
		return iconClean
	case "install-scripts":
		return iconHeuristic
	default:
		return iconCritical
	}
}

// demoIconText 返回带语义色的场景图标；关闭图标时为空串。
func (m *Model) demoIconText(id string) string {
	glyph := m.icons.glyph(demoScenarioIcon(id))
	if glyph == "" {
		return ""
	}
	text := glyph + " "
	switch demoScenarioIcon(id) {
	case iconClean:
		return m.styles.successText(text)
	case iconHeuristic:
		return m.styles.warningText(text)
	default:
		return m.styles.dangerText(text)
	}
}

func (m *Model) settingsView() string {
	var b strings.Builder
	b.WriteString(m.titleWithIcon(iconSettings, m.text(i18n.KeySettingsTitle)))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeySettingsIntro)))
	b.WriteString("\n\n")
	rows := []struct {
		icon  iconToken
		label string
		value string
	}{
		{iconLanguage, m.text(i18n.KeySettingsLanguageLabel), m.language.NativeName()},
		{iconPackage, m.text(i18n.KeySettingsEcosystemLabel), string(m.ecosystem)},
		{iconColor, m.text(i18n.KeySettingsColorLabel), m.colorDescription()},
		{iconAnimation, m.text(i18n.KeySettingsAnimationLabel), m.animationDescription()},
		{iconHome, "", m.text(i18n.KeySettingsBack)},
	}
	for index, row := range rows {
		marker := "  "
		selected := index == m.settingsAt
		if selected {
			marker = m.styles.accentText("› ")
		}
		iconText := ""
		if glyph := m.icons.glyph(row.icon); glyph != "" {
			// 语言图标“文”占两列，其余设置图标补齐到同一列宽，保证标签对齐。
			iconText = padPlain(glyph, 2) + " "
			if selected {
				iconText = m.styles.accentText(iconText)
			} else {
				iconText = m.styles.subtleText(iconText)
			}
		}
		if row.label == "" {
			text := row.value
			if selected {
				text = m.styles.selectedText(text)
			}
			b.WriteString(marker + iconText + text + "\n")
			continue
		}
		// 窄终端下把超长取值换行到下一行，避免隐藏颜色或动画的决定信息。
		labelText := padPlain(row.label, 20)
		if selected {
			labelText = m.styles.selectedText(labelText)
		}
		prefix := marker + iconText + labelText
		width := m.contentWidth() - displayWidth(prefix)
		if width < 1 {
			width = 1
		}
		for lineIndex, line := range wrapPlain(row.value, width) {
			if lineIndex > 0 {
				prefix = strings.Repeat(" ", displayWidth(prefix))
			}
			if selected {
				line = m.styles.selectedText(line)
			}
			b.WriteString(prefix + line + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeySettingsLanguageNote)))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeySettingsColorNote)))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeySettingsAnimationNote)))
	// 最小终端高度不足以显示全部说明时省略图标控制提示，避免裁掉更重要的决定信息。
	if m.contentHeight() >= 20 {
		b.WriteString("\n")
		b.WriteString(m.wrappedDim(m.text(i18n.KeySettingsIconNote)))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) loadingView() string {
	var b strings.Builder
	b.WriteString(m.titleWithIcon(iconLoading, m.loadingName))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeyLoadingIntro)))
	b.WriteString("\n\n")
	if m.cancelling {
		b.WriteString(m.wrappedPrefixed(iconWarning, m.text(i18n.KeyLoadingCancelling), m.styles.warningText))
		b.WriteString("\n")
		b.WriteString(m.wrappedDim(m.text(i18n.KeyLoadingNoPartial)))
		b.WriteString("\n")
	}
	stage := m.stageLabel(m.loading.Stage)
	if stage == "" {
		stage = m.text(i18n.KeyLoadingStageStart)
	}
	// 指示字符只表示“正在等待”，不代表完成的百分比；真实进度另行显示。
	glyph := m.styles.accentText(m.spinnerGlyph())
	stageLabel := "[" + stage + "]"
	// 阶段前缀固定宽度，正文按剩余显示宽度换行，避免长消息越过右边缘。
	messageWidth := m.contentWidth() - displayWidth(glyph) - 1 - displayWidth(stageLabel) - 2
	if messageWidth < 1 {
		messageWidth = 1
	}
	indent := strings.Repeat(" ", displayWidth(glyph)+1+displayWidth(stageLabel)+2)
	for index, line := range wrapPlain(sanitizeLine(m.loading.Message), messageWidth) {
		if index == 0 {
			fmt.Fprintf(&b, "%s %s  %s\n", glyph, m.styles.accentText(stageLabel), m.styles.boldText(line))
			continue
		}
		b.WriteString(indent + m.styles.boldText(line) + "\n")
	}
	if m.loading.Total > 0 {
		// 只有引擎给出真实完成数与总数时才显示进度条。
		b.WriteString("\n")
		b.WriteString(m.styles.accentText(progressBar(m.loading.Completed, m.loading.Total, m.progressBarWidth())))
		b.WriteString("\n")
	} else {
		b.WriteString("\n")
		b.WriteString(m.wrappedDim(m.text(i18n.KeyLoadingNoCountableTotal)))
		b.WriteString("\n")
	}
	if len(m.progressLog) > 0 {
		b.WriteString("\n")
		b.WriteString(m.styles.dimText(m.text(i18n.KeyLoadingRecentStages)))
		b.WriteString("\n")
		for _, item := range m.progressLog {
			stage := m.stageLabel(item.Stage)
			if stage == "" {
				stage = m.text(i18n.KeyLoadingStageStart)
			}
			line := sanitizeLine(stage + " · " + item.Message)
			for _, wrapped := range wrapPlain(line, m.contentWidth()-4) {
				b.WriteString("  " + m.styles.dimText(wrapped) + "\n")
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeyLoadingPressEsc)))
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) reportContent() string {
	if m.document == nil {
		return m.text(i18n.KeyReportNoDocument)
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
		return m.text(i18n.KeyComparisonNoDocument)
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
	b.WriteString(m.dangerTitleWithIcon(iconError, m.text(i18n.KeyErrorTitle)))
	b.WriteString("\n")
	for _, line := range wrapPlain(m.errorText, m.contentWidth()) {
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeyErrorNoSideEffects)))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeyErrorRetry)))
	return strings.TrimRight(b.String(), "\n")
}

// helpView 说明当前画面可用的按键，并列出全局快捷键。
func (m *Model) helpView() string {
	var b strings.Builder
	b.WriteString(m.titleWithIcon(iconKeyboard, m.text(i18n.KeyHelpTitle)))
	b.WriteString("\n")
	b.WriteString(m.wrappedDim(m.text(i18n.KeyHelpIntro)))
	b.WriteString("\n\n")

	current := m.helpContext()
	currentTitle := m.screenName(m.helpReturn)
	b.WriteString(m.wrappedStyled(m.format(i18n.KeyHelpCurrent, currentTitle), m.styles.accentText))
	b.WriteString("\n")
	for _, line := range wrapPlain(current, m.contentWidth()-2) {
		b.WriteString("  " + m.styles.boldText(line) + "\n")
	}

	sections := [][2]string{
		{m.text(i18n.KeyHelpSectionMenus), m.text(i18n.KeyHelpMenusBody)},
		{m.text(i18n.KeyHelpSectionForms), m.text(i18n.KeyHelpFormsBody)},
		{m.text(i18n.KeyHelpSectionReports), m.text(i18n.KeyHelpReportsBody)},
		{m.text(i18n.KeyHelpSectionViews), m.text(i18n.KeyHelpViewsBody)},
		{m.text(i18n.KeyHelpSectionScrolling), m.text(i18n.KeyHelpScrollingBody)},
		{m.text(i18n.KeyHelpSectionAnyTime), m.text(i18n.KeyHelpAnyTimeBody)},
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
		return m.text(i18n.KeyHelpContextPackageForm)
	case screenProjectForm:
		return m.text(i18n.KeyHelpContextProjectForm)
	case screenCompareForm:
		return m.text(i18n.KeyHelpContextCompareForm)
	case screenInventoryConfirm:
		return m.text(i18n.KeyHelpContextInventory)
	case screenDemoPicker:
		return m.text(i18n.KeyHelpContextDemo)
	case screenSettings:
		return m.text(i18n.KeyHelpContextSettings)
	case screenLoading:
		return m.text(i18n.KeyHelpContextLoading)
	case screenReport:
		return m.text(i18n.KeyHelpContextReport)
	case screenComparison:
		return m.text(i18n.KeyHelpContextComparison)
	default:
		return m.text(i18n.KeyHelpContextMenu)
	}
}

// screenName 返回画面的人类可读名称，用于帮助与页眉。
func (m *Model) screenName(value screen) string {
	switch value {
	case screenMenu:
		return m.text(i18n.KeyScreenStart)
	case screenPackageForm:
		return m.text(i18n.KeyScreenPackageForm)
	case screenProjectForm:
		return m.text(i18n.KeyScreenProjectForm)
	case screenCompareForm:
		return m.text(i18n.KeyScreenCompareForm)
	case screenInventoryConfirm:
		return m.text(i18n.KeyScreenInventory)
	case screenDemoPicker:
		return m.text(i18n.KeyScreenDemo)
	case screenSettings:
		return m.text(i18n.KeyScreenSettings)
	case screenLoading:
		return m.text(i18n.KeyScreenWorking)
	case screenReport:
		return m.text(i18n.KeyScreenReport)
	case screenComparison:
		return m.text(i18n.KeyScreenComparison)
	case screenError:
		return m.text(i18n.KeyScreenError)
	default:
		return m.text(i18n.KeyScreenHelp)
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
		label += m.styles.subtleText("  " + m.text(i18n.KeyFormEditing))
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
	visible, cursorCell, _ := visibleField(field.value, field.cursor, inner-1)
	before, after := splitAtCell(visible, cursorCell)
	if focused {
		content = m.styles.boldText(before) + m.styles.accentText("▏") + m.styles.boldText(after)
	} else {
		content = visible
	}
	return "[ " + content + " ]"
}

// splitAtCell 按终端显示列把文本切成光标前后两段。
// 中文输入时光标列不等于 rune 下标，必须按显示宽度切分。
func splitAtCell(value string, cell int) (string, string) {
	if cell <= 0 {
		return "", value
	}
	used := 0
	for index, r := range value {
		width := termtext.Width(string(r))
		if used+width > cell {
			return value[:index], value[index:]
		}
		used += width
	}
	return value, ""
}

func (m *Model) formErrorView() string {
	if strings.TrimSpace(m.formError) == "" {
		return ""
	}
	prefix := "! "
	if glyph := m.icons.glyph(iconError); glyph != "" {
		prefix = glyph + " "
	}
	var b strings.Builder
	b.WriteString("\n")
	for _, line := range wrapPlain(prefix+m.formError, m.contentWidth()) {
		b.WriteString(m.styles.dangerText(line))
		b.WriteString("\n")
	}
	return b.String()
}

// wrappedPrefixed 在首行前附加图标并按剩余显示宽度换行；
// 关闭图标时输出与无前缀版本一致，不会改变原有文案。
func (m *Model) wrappedPrefixed(token iconToken, text string, style func(string) string) string {
	prefix := m.icons.prefix(token)
	indent := strings.Repeat(" ", displayWidth(prefix))
	var b strings.Builder
	for index, line := range wrapPlain(text, m.contentWidth()-displayWidth(prefix)) {
		if index > 0 {
			b.WriteString("\n")
			b.WriteString(style(indent + line))
			continue
		}
		b.WriteString(style(prefix + line))
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
