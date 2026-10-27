package render

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/janghotan/depscope/internal/i18n"
	"github.com/janghotan/depscope/internal/inventory"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
	"github.com/janghotan/depscope/internal/termtext"
)

const (
	inventoryListLimit = 200
	maxActions         = 5
)

// TreeNode 描述依赖树中的一个节点。
type TreeNode struct {
	Label    string
	Children []string
}

// Tree 描述以 RootID 为根的依赖树；节点标签已包含版本与未解析标记。
type Tree struct {
	RootID string
	Nodes  map[string]TreeNode
}

// Section 表示报告中的附加信息块，例如项目格式或已安装包列表。
type Section struct {
	Title string
	Lines []string
	// Overview 表示该信息块应出现在单屏概览中。
	Overview bool
}

// Document 是包、项目与全局清单报告的统一展示模型。
type Document struct {
	Language    i18n.Language
	Title       string
	Target      string
	Subtitle    string
	DataMode    model.DataMode
	DemoNotice  *string
	ScannedAt   time.Time
	Status      model.ScanStatus
	Summary     model.Summary
	Findings    []model.Finding
	Checks      []model.CheckResult
	Tree        *Tree
	Sources     []model.SourceReference
	Limitations []model.Limitation
	Sections    []Section
	// FindingContexts 保存项目扫描提供的依赖上下文；包与清单报告保持为空。
	FindingContexts map[string]project.DependencyContext
}

// presentationLanguage 解析可选语言；未提供时使用英语，保证 CLI 输出不变。
func presentationLanguage(languages []i18n.Language) i18n.Language {
	if len(languages) == 0 {
		return i18n.English
	}
	return i18n.Normalize(languages[0])
}

// themeWith 在调用方未显式指定语言时沿用文档语言。
func (d Document) themeWith(theme Theme) Theme {
	if theme.Language == "" {
		theme.Language = d.Language
	}
	return theme
}

// demoNoticeText 返回用于展示的演示声明；英语沿用报告原文，其他语言使用本地化文案。
func (d Document) demoNoticeText(theme Theme) string {
	if theme.Lang() == i18n.English && d.DemoNotice != nil {
		return *d.DemoNotice
	}
	return theme.text(i18n.KeyReportDemoNotice)
}

// FromReport 将共享扫描报告转换为展示文档。
func FromReport(report model.ScanReport, languages ...i18n.Language) Document {
	language := presentationLanguage(languages)
	catalog := i18n.For(language)
	version := catalog.Text(i18n.KeyReportNotResolved)
	if report.Target.ResolvedVersion != nil {
		version = *report.Target.ResolvedVersion
	}
	return sanitizeDocument(Document{
		Language:    language,
		Title:       catalog.Text(i18n.KeyReportPackageTitle),
		Target:      report.Target.Package.String(),
		Subtitle:    catalog.Format(i18n.KeyReportSubtitleResolved, version),
		DataMode:    report.DataMode,
		DemoNotice:  report.DemoNotice,
		ScannedAt:   report.ScannedAt,
		Status:      report.Status,
		Summary:     report.Summary,
		Findings:    report.Findings,
		Checks:      report.Checks,
		Tree:        treeFromGraph(report.Dependencies, language),
		Sources:     report.Sources,
		Limitations: report.Limitations,
	})
}

// FromProject 将项目扫描结果转换为展示文档。
func FromProject(result project.Result, languages ...i18n.Language) Document {
	language := presentationLanguage(languages)
	catalog := i18n.For(language)
	formats := make([]string, 0, len(result.Formats)*4)
	for _, format := range result.Formats {
		lockfile := format.LockfilePath
		if lockfile == "" {
			lockfile = catalog.Text(i18n.KeyReportLockfileMissing)
		}
		formats = append(formats,
			fmt.Sprintf("%s  %s", format.Ecosystem, format.Format),
			catalog.Format(i18n.KeyReportManifestLine, format.ManifestPath),
			catalog.Format(i18n.KeyReportLockfileLine, lockfile),
			catalog.Format(i18n.KeyReportResolvedLine, format.ResolvedPackages, format.UnresolvedPackages),
			catalog.Format(i18n.KeyReportFormatResolutionLine, resolutionLabel(catalog, format.Resolution)),
		)
	}
	if len(formats) == 0 {
		formats = []string{catalog.Text(i18n.KeyReportNoFormats)}
	}
	sections := []Section{{Title: catalog.Text(i18n.KeyReportFormatsTitle), Lines: formats}}
	return sanitizeDocument(Document{
		Language:        language,
		Title:           catalog.Text(i18n.KeyReportProjectTitle),
		Target:          result.Root,
		Subtitle:        catalog.Text(i18n.KeyReportSubtitleProject),
		DataMode:        model.DataModeLive,
		ScannedAt:       result.ScannedAt,
		Status:          result.Status,
		Summary:         result.Summary,
		Findings:        result.Findings,
		Checks:          result.Checks,
		Tree:            treeFromProject(result.Dependencies, language),
		Sources:         result.Sources,
		Limitations:     result.Limitations,
		Sections:        sections,
		FindingContexts: result.FindingContexts,
	})
}

// FromInventory 将全局清单结果转换为展示文档。
func FromInventory(result inventory.Result, languages ...i18n.Language) Document {
	language := presentationLanguage(languages)
	catalog := i18n.For(language)
	lines := []string{}
	byEcosystem := map[model.Ecosystem]int{}
	for _, pkg := range result.Packages {
		// InstalledPackage.Ecosystem 由包管理器填充，缺失时回退到包标识中的生态，
		// 避免在概览中出现“已安装 5 个包，但 npm 0、PyPI 0”的自相矛盾数字。
		ecosystem := pkg.Ecosystem
		if ecosystem == "" {
			ecosystem = pkg.Package.Ecosystem
		}
		byEcosystem[ecosystem]++
	}
	ecosystems := make([]string, 0, len(byEcosystem))
	for ecosystem := range byEcosystem {
		ecosystems = append(ecosystems, string(ecosystem))
	}
	sort.Strings(ecosystems)
	for _, ecosystem := range ecosystems {
		lines = append(lines, catalog.Format(i18n.KeyReportInstalledLine, ecosystem, byEcosystem[model.Ecosystem(ecosystem)]))
	}
	if len(lines) == 0 {
		lines = append(lines, catalog.Text(i18n.KeyReportNoInstalled))
	}
	if len(result.Packages) > 0 {
		lines = append(lines, "")
		limit := len(result.Packages)
		if limit > inventoryListLimit {
			limit = inventoryListLimit
		}
		for _, pkg := range result.Packages[:limit] {
			lines = append(lines, fmt.Sprintf("  %s@%s  [%s]", pkg.Package.String(), pkg.Version, pkg.Manager))
		}
		if len(result.Packages) > limit {
			lines = append(lines, catalog.Format(i18n.KeyReportMorePackages, len(result.Packages)-limit))
		}
	}
	sections := []Section{
		inventoryOverviewSection(result, byEcosystem, language),
		{Title: catalog.Text(i18n.KeyReportInstalledPackagesTitle), Lines: lines},
	}
	return sanitizeDocument(Document{
		Language:    language,
		Title:       catalog.Text(i18n.KeyReportInventoryTitle),
		Target:      catalog.Text(i18n.KeyReportInventoryTarget),
		Subtitle:    catalog.Text(i18n.KeyReportSubtitleInventory),
		DataMode:    model.DataModeLive,
		ScannedAt:   result.ScannedAt,
		Status:      result.Status,
		Summary:     result.Summary,
		Findings:    result.Findings,
		Checks:      result.Checks,
		Sources:     result.Sources,
		Limitations: result.Limitations,
		Sections:    sections,
	})
}

// inventoryOverviewSection 汇总三类不同含义的数量，避免把漏洞、配对与包数混为一谈。
// 行文本按语言中最长的标签对齐，中英文都使用显示宽度而不是 rune 数量。
func inventoryOverviewSection(result inventory.Result, byEcosystem map[model.Ecosystem]int, language i18n.Language) Section {
	catalog := i18n.For(language)
	theme := Theme{Language: language}
	overview := result.Overview
	if overview.InstalledPackages == 0 && overview.AdvisoryMatches == 0 {
		overview.InstalledPackages = len(result.Packages)
		overview.AffectedPackages = 0
		overview.AdvisoryMatches = len(result.Findings)
	}

	rows := []struct {
		label string
		value string
	}{
		{
			label: catalog.Text(i18n.KeyInventoryInstalledLabel),
			value: catalog.Format(i18n.KeyInventoryInstalledValue,
				overview.InstalledPackages,
				byEcosystem[model.EcosystemNPM],
				byEcosystem[model.EcosystemPyPI]),
		},
		{
			label: catalog.Text(i18n.KeyInventoryAffectedLabel),
			value: catalog.Format(i18n.KeyInventoryAffectedValue, overview.AffectedPackages),
		},
		{
			label: catalog.Text(i18n.KeyInventoryMatchesLabel),
			value: catalog.Format(i18n.KeyInventoryMatchesValue, overview.AdvisoryMatches),
		},
	}
	labelWidth := 0
	for _, row := range rows {
		if width := termtext.Width(row.label); width > labelWidth {
			labelWidth = width
		}
	}
	// 固定留出六个空格，保持英语输出与既有对齐方式一致。
	labelWidth += 6
	lines := make([]string, 0, len(rows)+6)
	for _, row := range rows {
		lines = append(lines, termtext.PadRight(row.label, labelWidth)+row.value)
	}

	if overview.AdvisoryMatches > 0 {
		order := []model.Severity{
			model.SeverityCritical,
			model.SeverityHigh,
			model.SeverityMedium,
			model.SeverityLow,
			model.SeverityUnknown,
		}
		parts := make([]string, 0, len(order))
		for _, severity := range order {
			if count := overview.FindingsBySeverity[severity]; count > 0 {
				// 英语沿用既有小写统计形式；中文标签不含大小写，ToLower 不改变文字。
				parts = append(parts, fmt.Sprintf("%s %d", strings.ToLower(theme.severityLabel(severity)), count))
			}
		}
		if len(parts) > 0 {
			label := catalog.Text(i18n.KeyInventorySeverityLabel)
			if width := termtext.Width(label) + 6; width > labelWidth {
				labelWidth = width
			}
			lines = append(lines, "", termtext.PadRight(label, labelWidth)+strings.Join(parts, " · "))
		}
	}

	lines = append(lines, "",
		catalog.Text(i18n.KeyInventoryPairsNote),
		catalog.Text(i18n.KeyInventoryPairsNote2),
		catalog.Text(i18n.KeyInventoryPairsNote3))

	return Section{Title: catalog.Text(i18n.KeyReportOverviewTitle), Lines: lines, Overview: true}
}

func treeFromGraph(graph model.DependencyGraph, language i18n.Language) *Tree {
	if graph.RootNodeID == "" || len(graph.Nodes) == 0 {
		return nil
	}
	catalog := i18n.For(language)
	nodes := make(map[string]TreeNode, len(graph.Nodes))
	for _, node := range graph.Nodes {
		label := node.Package.String()
		if node.Version != nil && *node.Version != "" {
			label += "@" + *node.Version
		} else if node.Relationship != model.DependencyRoot {
			label += catalog.Text(i18n.KeyReportTreeUnresolved)
		}
		nodes[node.ID] = TreeNode{Label: label, Children: append([]string(nil), node.Dependencies...)}
	}
	return &Tree{RootID: graph.RootNodeID, Nodes: nodes}
}

func treeFromProject(graph project.Graph, language i18n.Language) *Tree {
	if graph.RootID == "" {
		return nil
	}
	catalog := i18n.For(language)
	nodes := make(map[string]TreeNode, len(graph.Nodes)+1)
	nodes[graph.RootID] = TreeNode{Label: catalog.Text(i18n.KeyReportTreeRoot), Children: []string{}}
	for _, node := range graph.Nodes {
		label := node.Package.String()
		if node.Version != nil && *node.Version != "" {
			label += "@" + *node.Version
		} else {
			label += catalog.Text(i18n.KeyReportTreeUnresolved)
		}
		if node.Constraint != "" {
			label += catalog.Format(i18n.KeyReportTreeDeclared, node.Constraint)
		}
		label += treeScopeTag(catalog, node.Scopes)
		nodes[node.ID] = TreeNode{Label: label, Children: append([]string(nil), node.Dependencies...)}
	}
	for _, node := range graph.Nodes {
		if node.Relationship == model.DependencyDirect {
			root := nodes[graph.RootID]
			root.Children = append(root.Children, node.ID)
			nodes[graph.RootID] = root
		}
	}
	root := nodes[graph.RootID]
	sort.Strings(root.Children)
	nodes[graph.RootID] = root
	return &Tree{RootID: graph.RootID, Nodes: nodes}
}

// Overview 返回单屏概览，突出状态、覆盖缺口、发现与建议动作。
func (d Document) Overview(theme Theme, selected int) string {
	theme = d.themeWith(theme)
	w := newWriter(theme)
	d.writeHeader(w)
	d.writeAssessment(w)
	d.writeOverviewSections(w)
	d.writeCoverageGaps(w, 3)
	d.writeFindingList(w, selected)
	d.writeActions(w)
	return strings.TrimRight(w.b.String(), "\n")
}

// writeOverviewSections 在总览中呈现概览类信息块（例如全局清单的分类计数），
// 让最重要的数字出现在首屏，而不是埋在详细信息页里。
// 安装包列表等长列表仍保留在详细信息页，避免总览被撑满。
func (d Document) writeOverviewSections(w *writer) {
	for _, section := range d.Sections {
		if !section.Overview {
			continue
		}
		w.blank()
		w.heading(section.Title)
		for _, line := range section.Lines {
			w.blockLine(line)
		}
	}
}

// Full 返回包含所有细节的完整纯文本报告。
func (d Document) Full(theme Theme) string {
	theme = d.themeWith(theme)
	w := newWriter(theme)
	d.writeHeader(w)
	d.writeAssessment(w)
	d.writeCoverageGaps(w, 0)
	if len(d.Findings) == 0 {
		w.heading(theme.text(i18n.KeyReportHeadingFindingsShort))
		w.paragraph(d.emptyFindingMessage(theme), "  ")
	} else {
		for index, finding := range d.Findings {
			w.blank()
			d.writeFinding(w, index, finding)
		}
	}
	w.blank()
	d.writeChecks(w)
	if d.Tree != nil {
		w.blank()
		d.writeDependencies(w)
	}
	for _, section := range d.Sections {
		w.blank()
		w.heading(section.Title)
		for _, line := range section.Lines {
			w.blockLine(line)
		}
	}
	w.blank()
	d.writeSources(w)
	w.blank()
	d.writeLimitations(w)
	w.blank()
	d.writeActions(w)
	return strings.TrimRight(w.b.String(), "\n")
}

// FindingDetail 返回单条发现的完整细节。
func (d Document) FindingDetail(index int, theme Theme) string {
	theme = d.themeWith(theme)
	if index < 0 || index >= len(d.Findings) {
		return theme.text(i18n.KeyReportNoFindingSelected)
	}
	w := newWriter(theme)
	w.heading(theme.format(i18n.KeyReportHeadingFindingDetail, index+1, len(d.Findings)))
	w.blank()
	d.writeFinding(w, index, d.Findings[index])
	w.blank()
	w.headingWith(theme.Icons.Action, []string{ansiAccent}, theme.text(i18n.KeyReportHeadingNextActions))
	actions := actionsForFinding(theme, d.Findings[index])
	if len(actions) == 0 {
		actions = []string{theme.text(i18n.KeyActionReviewEvidenceContext)}
	}
	for _, action := range actions {
		w.bullet(action)
	}
	return strings.TrimRight(w.b.String(), "\n")
}

// ChecksSection 返回所有检查及其完整消息。
func (d Document) ChecksSection(theme Theme) string {
	theme = d.themeWith(theme)
	w := newWriter(theme)
	d.writeChecks(w)
	return strings.TrimRight(w.b.String(), "\n")
}

// DependenciesSection 返回依赖树与图覆盖说明。
func (d Document) DependenciesSection(theme Theme) string {
	theme = d.themeWith(theme)
	w := newWriter(theme)
	if d.Tree == nil {
		w.headingWith(theme.Icons.Dependency, []string{ansiAccent}, theme.text(i18n.KeyReportHeadingDependencies))
		w.paragraph(theme.text(i18n.KeyReportNoDependencyGraph), "  ")
		return strings.TrimRight(w.b.String(), "\n")
	}
	d.writeDependencies(w)
	return strings.TrimRight(w.b.String(), "\n")
}

// SourcesSection 返回报告引用的全部来源。
func (d Document) SourcesSection(theme Theme) string {
	theme = d.themeWith(theme)
	w := newWriter(theme)
	d.writeSources(w)
	return strings.TrimRight(w.b.String(), "\n")
}

// DetailsSection 返回项目格式、已安装包等附加信息块。
func (d Document) DetailsSection(theme Theme) string {
	theme = d.themeWith(theme)
	w := newWriter(theme)
	if len(d.Sections) == 0 {
		w.heading(theme.text(i18n.KeyReportHeadingDetails))
		w.paragraph(theme.text(i18n.KeyReportDetailsNone), "  ")
		return strings.TrimRight(w.b.String(), "\n")
	}
	for index, section := range d.Sections {
		if index > 0 {
			w.blank()
		}
		w.heading(section.Title)
		for _, line := range section.Lines {
			w.blockLine(line)
		}
	}
	return strings.TrimRight(w.b.String(), "\n")
}

// LimitationsSection 返回报告中记录的全部覆盖限制。
func (d Document) LimitationsSection(theme Theme) string {
	theme = d.themeWith(theme)
	w := newWriter(theme)
	d.writeLimitations(w)
	return strings.TrimRight(w.b.String(), "\n")
}

// CompactSummary 返回可复制到问题跟踪器或聊天中的短摘要。
func (d Document) CompactSummary(theme Theme) string {
	theme = d.themeWith(theme)
	w := newWriter(theme)
	w.heading(theme.text(i18n.KeyReportHeadingSummary))
	target := d.Target
	status := theme.statusText(d.Status)
	succeeded, total := checkCoverage(d.Checks)
	w.blockLine(theme.format(i18n.KeyReportSummaryLine, target, status, succeeded, total))
	highest := theme.text(i18n.KeyValueNone)
	if d.Summary.HighestSeverity != nil {
		highest = theme.severityText(*d.Summary.HighestSeverity)
	}
	w.blockLine(theme.format(i18n.KeyReportSummaryHighest,
		highest, d.Summary.ConfirmedFindings, d.Summary.HeuristicWarnings, theme.dataModeLabel(d.DataMode)))
	if len(d.Findings) == 0 {
		w.blockLine(theme.text(i18n.KeyReportSummaryNoFindings))
	} else {
		w.blockLine(theme.text(i18n.KeyReportSummaryKeyFindings))
		limit := len(d.Findings)
		if limit > 5 {
			limit = 5
		}
		for _, finding := range d.Findings[:limit] {
			severity := prependGlyph(theme.severityGlyph(finding.Severity), "["+theme.severityLabel(finding.Severity)+"]")
			kindGlyph := theme.Icons.Confirmed
			if finding.Kind == model.FindingHeuristic {
				kindGlyph = theme.Icons.Heuristic
			}
			kind := prependGlyph(kindGlyph, "["+theme.kindLabel(finding.Kind)+"]")
			w.blockLine(fmt.Sprintf("- %s %s %s — %s", severity, kind, finding.ID, finding.Title))
		}
		if len(d.Findings) > limit {
			w.blockLine(theme.format(i18n.KeyReportSummaryMoreFindings, len(d.Findings)-limit))
		}
	}
	w.blockLine(theme.format(i18n.KeyReportSummarySources, sourceList(theme, d.Sources)))
	if d.DataMode == model.DataModeDemo {
		w.blockLine(theme.format(i18n.KeyReportSummaryDemo, d.demoNoticeText(theme)))
	}
	if d.Status == model.StatusIncomplete || d.Status == model.StatusFailed {
		gaps := prependGlyph(theme.Icons.Warning, coverageGapList(theme, d.Checks))
		w.blockLine(theme.format(i18n.KeyReportSummaryCoverageGaps, gaps))
	}
	return strings.TrimRight(w.b.String(), "\n")
}

type writer struct {
	theme Theme
	b     strings.Builder
}

func newWriter(theme Theme) *writer {
	return &writer{theme: theme}
}

func (w *writer) line(text string) {
	w.b.WriteString(text)
	w.b.WriteString("\n")
}

// blockLine 输出信息块中的一行；超宽时按显示宽度换行并保留缩进，
// 避免中文长句在窄终端被直接裁掉。
func (w *writer) blockLine(text string) {
	width := w.theme.WidthForContent()
	if strings.TrimSpace(text) == "" {
		w.blank()
		return
	}
	if termtext.Width(text) <= width {
		w.line(text)
		return
	}
	indent := len(text) - len(strings.TrimLeft(text, " "))
	if indent >= width {
		indent = 0
	}
	for _, line := range termtext.Wrap(strings.TrimSpace(text), width-indent) {
		w.line(strings.Repeat(" ", indent) + line)
	}
}

func (w *writer) blank() {
	w.b.WriteString("\n")
}

func (w *writer) heading(text string) {
	w.headingWith("", nil, text)
}

// headingWith 在区块标题前附加可选图标；图标独立着色，标题本身保持统一层级。
// 图标为空时输出与既有标题完全一致。
func (w *writer) headingWith(icon string, codes []string, text string) {
	prefix := ""
	if icon != "" {
		prefix = icon + " "
	}
	w.line(w.theme.paint(prefix, codes...) + w.theme.paint(strings.ToUpper(text), ansiBold, ansiAccent))
}

func (w *writer) paragraph(text, prefix string) {
	width := w.theme.WidthForContent() - termtext.Width(prefix)
	for _, line := range wrapText(text, width) {
		w.line(prefix + line)
	}
}

// bullet 输出行动项。零值主题沿用既有圆点，启用图标时使用明确的方向图标。
func (w *writer) bullet(text string) {
	if w.theme.Icons.Action == "" {
		w.paragraph("• "+text, "  ")
		return
	}
	w.prefixedStyled("  "+w.theme.paint(w.theme.Icons.Action, ansiAccent)+" ", text, nil)
}

// prefixed 输出带固定前缀的纯文本行；超宽时按显示宽度换行，续行对齐前缀。
func (w *writer) prefixed(prefix, text string) {
	w.prefixedStyled(prefix, text, nil)
}

// prefixedStyled 与 prefixed 相同，但对每一行正文应用样式。
// text 必须是纯文本；样式在换行后应用，避免 ANSI 序列被切断。
func (w *writer) prefixedStyled(prefix, text string, style func(string) string) {
	width := w.theme.WidthForContent()
	indent := termtext.Width(prefix)
	if indent >= width {
		// 前缀本身接近整行时先单独输出，正文从下一行开始，保证不越过右边缘。
		w.line(prefix)
		prefix = ""
		indent = 0
	}
	apply := style
	if apply == nil {
		apply = func(value string) string { return value }
	}
	for _, line := range wrapText(text, width-indent) {
		w.line(prefix + apply(line))
		prefix = strings.Repeat(" ", indent)
	}
}

// checkLine 输出检查状态、名称与标识；窄终端下名称与标识分行，避免右侧被裁切。
func (w *writer) checkLine(prefix, name, id string) {
	width := w.theme.WidthForContent()
	indent := termtext.Width(prefix)
	plain := name + "  [" + id + "]"
	if indent+termtext.Width(plain) <= width {
		w.line(prefix + w.theme.bold(name) + "  " + w.theme.dim("["+id+"]"))
		return
	}
	w.prefixedStyled(prefix, name, w.theme.bold)
	pad := strings.Repeat(" ", indent)
	for _, line := range wrapText("["+id+"]", width-indent) {
		w.line(pad + w.theme.dim(line))
	}
}

// treeLine 输出依赖树的一行；长标签按显示宽度换行并延续树形缩进。
func (w *writer) treeLine(head, text string) {
	width := w.theme.WidthForContent()
	indent := termtext.Width(head)
	if indent >= width {
		w.line(head)
		head = ""
		indent = 0
	}
	for _, line := range wrapText(text, width-indent) {
		w.line(head + line)
		head = strings.Repeat(" ", indent)
	}
}

func (w *writer) keyValue(key, value string) {
	const keyWidth = 19
	plainKey := padRight(key, keyWidth)
	prefix := "  " + w.theme.dim(plainKey) + "  "
	if termtext.Width(prefix)+termtext.Width(value) > w.theme.WidthForContent() {
		// 超宽值按纯文本换行；带样式的短值不会走到这里。
		value = stripANSI(value)
	}
	w.prefixed(prefix, value)
}

func (d Document) writeHeader(w *writer) {
	theme := w.theme
	w.line(theme.paint(strings.ToUpper(d.Title), ansiBold, ansiAccent))
	for _, line := range wrapText(d.Target, theme.WidthForContent()) {
		w.line(theme.bold(line))
	}
	if d.Subtitle != "" {
		w.paragraph(d.Subtitle, "")
	}
	w.blank()
	w.keyValue(theme.text(i18n.KeyReportHeaderDataMode), theme.dataModeLabel(d.DataMode))
	if !d.ScannedAt.IsZero() {
		w.keyValue(theme.text(i18n.KeyReportHeaderScannedAt), d.ScannedAt.UTC().Format(time.RFC3339))
	}
	if len(d.Sources) > 0 {
		w.keyValue(theme.text(i18n.KeyReportHeaderSources), theme.format(i18n.KeyReportHeaderSourcesCount, len(d.Sources)))
	}
	if d.DataMode == model.DataModeDemo {
		w.blank()
		badge := theme.badgeText(theme.Icons.Demo, theme.text(i18n.KeyReportDemoBadge), ansiMedium, ansiBold)
		prefix := badge + " "
		width := theme.WidthForContent() - termtext.Width(badge) - 1
		for _, line := range wrapText(d.demoNoticeText(theme), width) {
			w.line(prefix + theme.paint(line, ansiMedium))
			prefix = strings.Repeat(" ", termtext.Width(badge)+1)
		}
	}
	// 非英语界面明确说明扫描器诊断与公告原文保持原样，避免把证据误认为未翻译。
	if theme.Lang() != i18n.English {
		w.blank()
		w.paragraph(theme.text(i18n.KeyReportSourceLanguageNote), "  ")
	}
}

func (d Document) writeAssessment(w *writer) {
	theme := w.theme
	w.blank()
	w.heading(theme.text(i18n.KeyReportHeadingAssessment))
	w.keyValue(theme.text(i18n.KeyReportLabelStatus), theme.statusBadge(d.Status))
	succeeded, total := checkCoverage(d.Checks)
	coverage := theme.format(i18n.KeyReportCoverageSucceeded, succeeded, total)
	if total == 0 {
		coverage = theme.text(i18n.KeyReportCoverageNoChecks)
	}
	w.keyValue(theme.text(i18n.KeyReportLabelCompletedChecks), coverage)
	w.keyValue(theme.text(i18n.KeyReportLabelConfirmed), fmt.Sprint(d.Summary.ConfirmedFindings))
	w.keyValue(theme.text(i18n.KeyReportLabelHeuristic), fmt.Sprint(d.Summary.HeuristicWarnings))
	highest := theme.text(i18n.KeyValueNone)
	if d.Summary.HighestSeverity != nil {
		highest = theme.severityBadge(*d.Summary.HighestSeverity)
	}
	w.keyValue(theme.text(i18n.KeyReportLabelHighest), highest)
	if d.Status == model.StatusIncomplete || d.Status == model.StatusFailed {
		w.keyValue(theme.text(i18n.KeyReportLabelInterpretation), theme.text(i18n.KeyReportInterpretationIncomplete))
	}
}

func (d Document) writeCoverageGaps(w *writer, limitationLimit int) {
	theme := w.theme
	gaps := make([]model.CheckResult, 0)
	for _, check := range d.Checks {
		if check.Status != model.CheckSucceeded {
			gaps = append(gaps, check)
		}
	}
	if len(gaps) == 0 && len(d.Limitations) == 0 {
		return
	}
	w.blank()
	w.headingWith(theme.Icons.Warning, []string{ansiMedium}, theme.text(i18n.KeyReportHeadingCoverageGaps))
	for _, check := range gaps {
		w.checkLine("  "+theme.checkBadge(check.Status)+" ", check.Name, check.ID)
		w.paragraph(check.Message, "      ")
	}
	if len(d.Limitations) > 0 {
		limit := len(d.Limitations)
		if limitationLimit > 0 && limit > limitationLimit {
			limit = limitationLimit
		}
		for _, limitation := range d.Limitations[:limit] {
			w.prefixed("  "+theme.dim(limitation.Code)+": ", limitation.Message)
		}
		if limit < len(d.Limitations) {
			w.line(theme.dim(theme.format(i18n.KeyReportMoreLimitations, len(d.Limitations)-limit)))
		}
	}
}

func (d Document) emptyFindingMessage(theme Theme) string {
	if d.Status == model.StatusIncomplete || d.Status == model.StatusFailed {
		return theme.text(i18n.KeyReportEmptyFindingsIncomplete)
	}
	return theme.text(i18n.KeyReportEmptyFindings)
}

func (d Document) writeFindingList(w *writer, selected int) {
	theme := w.theme
	w.blank()
	w.heading(theme.format(i18n.KeyReportHeadingFindings, len(d.Findings)))
	if len(d.Findings) == 0 {
		w.paragraph(d.emptyFindingMessage(theme), "  ")
		return
	}
	for index, finding := range d.Findings {
		marker := "  "
		if index == selected {
			marker = theme.accent("› ")
		}
		head := fmt.Sprintf("%s%d  %s %s ", marker, index+1, theme.severityBadge(finding.Severity), theme.kindBadge(finding.Kind))
		w.prefixedStyled(head, finding.ID, theme.dim)
		w.paragraph(finding.Title, "     ")
		line := finding.Package.String()
		if finding.Version != nil && *finding.Version != "" {
			line += "@" + *finding.Version
		}
		if finding.Risk != nil {
			line += theme.format(i18n.KeyReportRiskSuffix, finding.Risk.Score)
		}
		if context, ok := d.FindingContexts[finding.ID]; ok {
			line += " · " + compactContextLabel(i18n.For(theme.Lang()), context)
		}
		w.paragraph(line, "     ")
	}
}

func (d Document) writeFinding(w *writer, index int, finding model.Finding) {
	theme := w.theme
	w.heading(theme.format(i18n.KeyReportHeadingFindingDetail, index+1, len(d.Findings)))
	w.blank()
	w.prefixedStyled(theme.severityBadge(finding.Severity)+" "+theme.kindBadge(finding.Kind)+" ", finding.Title, theme.bold)
	w.keyValue(theme.text(i18n.KeyReportLabelID), finding.ID)
	pkg := finding.Package.String()
	if finding.Version != nil && *finding.Version != "" {
		pkg += "@" + *finding.Version
	}
	w.keyValue(theme.text(i18n.KeyReportLabelPackage), pkg)
	if finding.Risk != nil {
		w.keyValue(theme.text(i18n.KeyReportLabelRisk), fmt.Sprintf("%.0f/100", finding.Risk.Score))
	} else {
		w.keyValue(theme.text(i18n.KeyReportLabelRisk), theme.text(i18n.KeyReportRiskNotComputed))
	}
	if context, ok := d.FindingContexts[finding.ID]; ok {
		w.blank()
		d.writeDependencyContext(w, context)
	}
	w.blank()
	w.paragraph(finding.Description, "  ")
	if finding.Risk != nil && strings.TrimSpace(finding.Risk.Rationale) != "" {
		w.blank()
		w.heading(theme.text(i18n.KeyReportHeadingRiskRationale))
		w.paragraph(finding.Risk.Rationale, "  ")
	}
	if len(finding.Evidence) > 0 {
		w.blank()
		w.headingWith(theme.Icons.Source, []string{ansiDim}, theme.text(i18n.KeyReportHeadingEvidence))
		for _, evidence := range finding.Evidence {
			w.prefixed("  "+theme.dim("["+string(evidence.Kind)+"]")+" ", evidence.Summary)
			details := theme.format(i18n.KeyReportEvidenceSource, evidence.SourceID)
			if evidence.Reference != "" {
				details += theme.format(i18n.KeyReportEvidenceReference, evidence.Reference)
			}
			w.paragraph(details, "      ")
		}
	}
	if finding.Remediation != nil && strings.TrimSpace(*finding.Remediation) != "" {
		w.blank()
		w.headingWith(theme.Icons.Action, []string{ansiAccent}, theme.text(i18n.KeyReportHeadingRemediation))
		w.paragraph(*finding.Remediation, "  ")
	}
}

func (d Document) writeDependencyContext(w *writer, context project.DependencyContext) {
	theme := w.theme
	catalog := i18n.For(theme.Lang())
	w.heading(theme.text(i18n.KeyReportHeadingDependencyContext))
	w.keyValue(theme.text(i18n.KeyReportLabelRelationship), relationshipLabel(catalog, context.Relationship))
	w.keyValue(theme.text(i18n.KeyReportLabelScope), scopeLabels(catalog, context.Scopes))
	if hasUnknownScope(context.Scopes) {
		w.paragraph(theme.text(i18n.KeyReportScopeUnknownNote), "      ")
	}
	w.keyValue(theme.text(i18n.KeyReportLabelResolution), resolutionLabel(catalog, context.Resolution))
	switch context.Resolution {
	case project.DependencyResolutionPartial:
		w.paragraph(theme.text(i18n.KeyReportResolutionPartialNote), "      ")
	case project.DependencyResolutionUnknown:
		w.paragraph(theme.text(i18n.KeyReportResolutionUnknownNote), "      ")
	}
	if evidence := contextEvidenceLabels(context.Evidence); evidence != "" {
		w.keyValue(theme.text(i18n.KeyReportLabelContextEvidence), evidence)
	}
	w.paragraph(theme.text(i18n.KeyReportContextDisclaimer), "  ")
}

func (d Document) writeChecks(w *writer) {
	theme := w.theme
	w.heading(theme.text(i18n.KeyReportHeadingChecks))
	succeeded, total := checkCoverage(d.Checks)
	w.paragraph(theme.format(i18n.KeyReportChecksCoverage, succeeded, total), "  ")
	if len(d.Checks) == 0 {
		w.blank()
		w.paragraph(theme.text(i18n.KeyReportChecksNone), "  ")
		return
	}
	for _, check := range d.Checks {
		w.blank()
		w.checkLine("  "+theme.checkBadge(check.Status)+" ", check.Name, check.ID)
		w.paragraph(check.Message, "      ")
		if len(check.SourceIDs) > 0 {
			w.paragraph(theme.format(i18n.KeyReportChecksSources, strings.Join(check.SourceIDs, ", ")), "      ")
		}
	}
}

func (d Document) writeDependencies(w *writer) {
	theme := w.theme
	w.headingWith(theme.Icons.Dependency, []string{ansiAccent}, theme.text(i18n.KeyReportHeadingDependencies))
	w.paragraph(theme.format(i18n.KeyReportDependenciesCount, nodeCount(d.Tree)), "  ")
	w.blank()
	if d.Tree == nil {
		w.paragraph(theme.text(i18n.KeyReportNoDependencyGraph), "  ")
		return
	}
	root, exists := d.Tree.Nodes[d.Tree.RootID]
	if !exists {
		w.paragraph(theme.text(i18n.KeyReportDependencyNoRoot), "  ")
		return
	}
	w.treeLine("  ", root.Label)
	rendered := map[string]bool{d.Tree.RootID: true}
	d.writeTreeChildren(w, root.Children, "  ", map[string]bool{d.Tree.RootID: true}, rendered)
	orphans := make([]string, 0)
	for id := range d.Tree.Nodes {
		if !rendered[id] {
			orphans = append(orphans, id)
		}
	}
	if len(orphans) > 0 {
		sort.Strings(orphans)
		w.blank()
		w.paragraph(theme.format(i18n.KeyReportDependencyOrphans, len(orphans)), "  ")
		for _, id := range orphans {
			node := d.Tree.Nodes[id]
			w.treeLine("  "+theme.dim("•")+" ", node.Label)
			d.writeTreeChildren(w, node.Children, "    ", map[string]bool{id: true}, rendered)
		}
	}
	graphStatus := ""
	for _, check := range d.Checks {
		if check.ID == "dependency-graph" || strings.HasSuffix(check.ID, ":dependency-resolution") {
			if check.Status != model.CheckSucceeded {
				graphStatus = strings.ToLower(theme.checkLabel(check.Status))
				break
			}
		}
	}
	if graphStatus != "" {
		w.blank()
		w.paragraph(theme.text(i18n.KeyReportDependencyIncomplete), "  ")
	}
}

func (d Document) writeTreeChildren(w *writer, children []string, prefix string, path map[string]bool, rendered map[string]bool) {
	children = append([]string(nil), children...)
	sort.Strings(children)
	for index, id := range children {
		node, exists := d.Tree.Nodes[id]
		if !exists {
			continue
		}
		last := index == len(children)-1
		branch := "├─ "
		nextPrefix := prefix + "│  "
		if last {
			branch = "└─ "
			nextPrefix = prefix + "   "
		}
		marker := ""
		if path[id] {
			marker = w.theme.text(i18n.KeyReportDependencyCycle)
		}
		w.treeLine(prefix+branch, node.Label+marker)
		if path[id] {
			continue
		}
		rendered[id] = true
		nextPath := make(map[string]bool, len(path)+1)
		for key, value := range path {
			nextPath[key] = value
		}
		nextPath[id] = true
		d.writeTreeChildren(w, node.Children, nextPrefix, nextPath, rendered)
	}
}

func (d Document) writeSources(w *writer) {
	theme := w.theme
	w.headingWith(theme.Icons.Source, []string{ansiDim}, theme.text(i18n.KeyReportHeadingSources))
	if len(d.Sources) == 0 {
		w.paragraph(theme.text(i18n.KeyReportSourcesNone), "  ")
		return
	}
	prefix := "  "
	if theme.Icons.Source != "" {
		prefix += theme.dim(theme.Icons.Source) + " "
	}
	urlIndent := strings.Repeat(" ", termtext.Width(prefix)+2)
	for _, source := range d.Sources {
		line := theme.format(i18n.KeyReportSourceLine,
			source.ID, source.Provider, source.Identifier, source.RetrievedAt.UTC().Format(time.RFC3339))
		w.paragraph(line, prefix)
		if source.URL != nil {
			w.paragraph(theme.format(i18n.KeyReportSourceURL, *source.URL), urlIndent)
		}
	}
}

func (d Document) writeLimitations(w *writer) {
	theme := w.theme
	w.headingWith(theme.Icons.Warning, []string{ansiMedium}, theme.text(i18n.KeyReportHeadingLimitations))
	if len(d.Limitations) == 0 {
		w.paragraph(theme.text(i18n.KeyReportLimitationsNone), "  ")
		return
	}
	for _, limitation := range d.Limitations {
		w.prefixedStyled("  ", limitation.Code, theme.dim)
		w.paragraph(limitation.Message, "    ")
	}
}

func (d Document) writeActions(w *writer) {
	w.blank()
	w.headingWith(w.theme.Icons.Action, []string{ansiAccent}, w.theme.text(i18n.KeyReportHeadingNextActions))
	for _, action := range d.actions(w.theme) {
		w.bullet(action)
	}
}

func (d Document) actions(theme Theme) []string {
	actions := []string{}
	seen := map[string]bool{}
	add := func(action string) {
		action = strings.TrimSpace(action)
		if action == "" || seen[action] || len(actions) >= maxActions {
			return
		}
		seen[action] = true
		actions = append(actions, action)
	}

	for _, finding := range d.Findings {
		if finding.Kind == model.FindingConfirmed {
			if finding.Remediation != nil {
				add(*finding.Remediation)
			} else {
				add(theme.format(i18n.KeyActionReviewConfirmed, finding.ID))
			}
		}
		if strings.HasPrefix(finding.ID, "LIFECYCLE-") {
			add(theme.text(i18n.KeyActionLifecycle))
		}
		if strings.HasPrefix(finding.ID, "TYPOSQUAT-") {
			add(theme.text(i18n.KeyActionTyposquat))
		}
	}
	if d.Status == model.StatusIncomplete {
		add(theme.format(i18n.KeyActionIncomplete, coverageGapList(theme, d.Checks)))
	}
	if d.Status == model.StatusFailed {
		add(theme.text(i18n.KeyActionFailed))
	}
	if d.Status == model.StatusClean {
		add(theme.text(i18n.KeyActionClean))
	}
	if len(actions) == 0 && len(d.Findings) > 0 {
		add(theme.text(i18n.KeyActionReviewHeuristics))
	}
	if len(actions) == 0 {
		add(theme.text(i18n.KeyActionReviewEvidence))
	}
	return actions
}

func actionsForFinding(theme Theme, finding model.Finding) []string {
	actions := []string{}
	if finding.Remediation != nil && strings.TrimSpace(*finding.Remediation) != "" {
		actions = append(actions, *finding.Remediation)
	}
	if strings.HasPrefix(finding.ID, "LIFECYCLE-") {
		actions = append(actions, theme.text(i18n.KeyActionInspectLifecycle))
	}
	if strings.HasPrefix(finding.ID, "TYPOSQUAT-") {
		actions = append(actions, theme.text(i18n.KeyActionConfirmPublisher))
	}
	if len(actions) == 0 {
		actions = append(actions, theme.text(i18n.KeyActionCompareEvidence))
	}
	if len(actions) > maxActions {
		actions = actions[:maxActions]
	}
	return actions
}

func checkCoverage(checks []model.CheckResult) (int, int) {
	succeeded := 0
	for _, check := range checks {
		if check.Status == model.CheckSucceeded {
			succeeded++
		}
	}
	return succeeded, len(checks)
}

func coverageGapList(theme Theme, checks []model.CheckResult) string {
	names := []string{}
	for _, check := range checks {
		if check.Status != model.CheckSucceeded {
			names = append(names, check.Name+" ("+strings.ToLower(theme.checkLabel(check.Status))+")")
		}
	}
	if len(names) == 0 {
		return theme.text(i18n.KeyValueNone)
	}
	return strings.Join(names, ", ")
}

func sourceList(theme Theme, sources []model.SourceReference) string {
	if len(sources) == 0 {
		return theme.text(i18n.KeyValueNone)
	}
	ids := make([]string, 0, len(sources))
	for _, source := range sources {
		ids = append(ids, source.ID)
	}
	return strings.Join(ids, ", ")
}

func nodeCount(tree *Tree) int {
	if tree == nil {
		return 0
	}
	return len(tree.Nodes)
}
