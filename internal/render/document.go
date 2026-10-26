package render

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/janghotan/depscope/internal/inventory"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/project"
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
}

// Document 是包、项目与全局清单报告的统一展示模型。
type Document struct {
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
}

// FromReport 将共享扫描报告转换为展示文档。
func FromReport(report model.ScanReport) Document {
	version := "(not resolved)"
	if report.Target.ResolvedVersion != nil {
		version = *report.Target.ResolvedVersion
	}
	return sanitizeDocument(Document{
		Title:       "PACKAGE REPORT",
		Target:      report.Target.Package.String(),
		Subtitle:    "Resolved version " + version,
		DataMode:    report.DataMode,
		DemoNotice:  report.DemoNotice,
		ScannedAt:   report.ScannedAt,
		Status:      report.Status,
		Summary:     report.Summary,
		Findings:    report.Findings,
		Checks:      report.Checks,
		Tree:        treeFromGraph(report.Dependencies),
		Sources:     report.Sources,
		Limitations: report.Limitations,
	})
}

// FromProject 将项目扫描结果转换为展示文档。
func FromProject(result project.Result) Document {
	formats := make([]string, 0, len(result.Formats)*4)
	for _, format := range result.Formats {
		lockfile := format.LockfilePath
		if lockfile == "" {
			lockfile = "(missing)"
		}
		formats = append(formats,
			fmt.Sprintf("%s  %s", format.Ecosystem, format.Format),
			fmt.Sprintf("  manifest: %s", format.ManifestPath),
			fmt.Sprintf("  lockfile: %s", lockfile),
			fmt.Sprintf("  resolved packages: %d · unresolved: %d", format.ResolvedPackages, format.UnresolvedPackages),
		)
	}
	if len(formats) == 0 {
		formats = []string{"No supported project formats were detected."}
	}
	sections := []Section{{Title: "FORMATS", Lines: formats}}
	return sanitizeDocument(Document{
		Title:       "PROJECT REPORT",
		Target:      result.Root,
		Subtitle:    "Dependency files are read only; project code and install scripts are never executed.",
		DataMode:    model.DataModeLive,
		ScannedAt:   result.ScannedAt,
		Status:      result.Status,
		Summary:     result.Summary,
		Findings:    result.Findings,
		Checks:      result.Checks,
		Tree:        treeFromProject(result.Dependencies),
		Sources:     result.Sources,
		Limitations: result.Limitations,
		Sections:    sections,
	})
}

// FromInventory 将全局清单结果转换为展示文档。
func FromInventory(result inventory.Result) Document {
	lines := []string{}
	byEcosystem := map[model.Ecosystem]int{}
	for _, pkg := range result.Packages {
		byEcosystem[pkg.Ecosystem]++
	}
	ecosystems := make([]string, 0, len(byEcosystem))
	for ecosystem := range byEcosystem {
		ecosystems = append(ecosystems, string(ecosystem))
	}
	sort.Strings(ecosystems)
	for _, ecosystem := range ecosystems {
		lines = append(lines, fmt.Sprintf("%s  %d installed package(s)", ecosystem, byEcosystem[model.Ecosystem(ecosystem)]))
	}
	if len(lines) == 0 {
		lines = append(lines, "No installed packages were reported by the configured package managers.")
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
			lines = append(lines, fmt.Sprintf("  … %d more package(s) omitted from this view.", len(result.Packages)-limit))
		}
	}
	return sanitizeDocument(Document{
		Title:       "GLOBAL INVENTORY",
		Target:      "Local npm and PyPI installations",
		Subtitle:    "Read-only metadata inventory; packages and install scripts are never executed.",
		DataMode:    model.DataModeLive,
		ScannedAt:   result.ScannedAt,
		Status:      result.Status,
		Summary:     result.Summary,
		Findings:    result.Findings,
		Checks:      result.Checks,
		Sources:     result.Sources,
		Limitations: result.Limitations,
		Sections:    []Section{{Title: "INSTALLED PACKAGES", Lines: lines}},
	})
}

func treeFromGraph(graph model.DependencyGraph) *Tree {
	if graph.RootNodeID == "" || len(graph.Nodes) == 0 {
		return nil
	}
	nodes := make(map[string]TreeNode, len(graph.Nodes))
	for _, node := range graph.Nodes {
		label := node.Package.String()
		if node.Version != nil && *node.Version != "" {
			label += "@" + *node.Version
		} else if node.Relationship != model.DependencyRoot {
			label += " (unresolved)"
		}
		nodes[node.ID] = TreeNode{Label: label, Children: append([]string(nil), node.Dependencies...)}
	}
	return &Tree{RootID: graph.RootNodeID, Nodes: nodes}
}

func treeFromProject(graph project.Graph) *Tree {
	if graph.RootID == "" {
		return nil
	}
	nodes := make(map[string]TreeNode, len(graph.Nodes)+1)
	nodes[graph.RootID] = TreeNode{Label: "(project root)", Children: []string{}}
	for _, node := range graph.Nodes {
		label := node.Package.String()
		if node.Version != nil && *node.Version != "" {
			label += "@" + *node.Version
		} else {
			label += " (unresolved)"
		}
		if node.Constraint != "" {
			label += " · declared " + node.Constraint
		}
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
	w := newWriter(theme)
	d.writeHeader(w)
	d.writeAssessment(w)
	d.writeCoverageGaps(w, 3)
	d.writeFindingList(w, selected)
	d.writeActions(w)
	return strings.TrimRight(w.b.String(), "\n")
}

// Full 返回包含所有细节的完整纯文本报告。
func (d Document) Full(theme Theme) string {
	w := newWriter(theme)
	d.writeHeader(w)
	d.writeAssessment(w)
	d.writeCoverageGaps(w, 0)
	if len(d.Findings) == 0 {
		w.heading("FINDINGS")
		w.paragraph(d.emptyFindingMessage(), "  ")
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
			if strings.TrimSpace(line) == "" {
				w.blank()
				continue
			}
			w.raw(line)
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
	if index < 0 || index >= len(d.Findings) {
		return "No finding is selected."
	}
	w := newWriter(theme)
	w.heading(fmt.Sprintf("FINDING %d OF %d", index+1, len(d.Findings)))
	w.blank()
	d.writeFinding(w, index, d.Findings[index])
	w.blank()
	w.heading("NEXT ACTIONS")
	actions := actionsForFinding(d.Findings[index])
	if len(actions) == 0 {
		actions = []string{"Review the evidence and package context before making a decision."}
	}
	for _, action := range actions {
		w.bullet(action)
	}
	return strings.TrimRight(w.b.String(), "\n")
}

// ChecksSection 返回所有检查及其完整消息。
func (d Document) ChecksSection(theme Theme) string {
	w := newWriter(theme)
	d.writeChecks(w)
	return strings.TrimRight(w.b.String(), "\n")
}

// DependenciesSection 返回依赖树与图覆盖说明。
func (d Document) DependenciesSection(theme Theme) string {
	w := newWriter(theme)
	if d.Tree == nil {
		w.heading("DEPENDENCIES")
		w.paragraph("No dependency graph was recorded.", "  ")
		return strings.TrimRight(w.b.String(), "\n")
	}
	d.writeDependencies(w)
	return strings.TrimRight(w.b.String(), "\n")
}

// SourcesSection 返回报告引用的全部来源。
func (d Document) SourcesSection(theme Theme) string {
	w := newWriter(theme)
	d.writeSources(w)
	return strings.TrimRight(w.b.String(), "\n")
}

// DetailsSection 返回项目格式、已安装包等附加信息块。
func (d Document) DetailsSection(theme Theme) string {
	w := newWriter(theme)
	if len(d.Sections) == 0 {
		w.heading("DETAILS")
		w.paragraph("No additional details were recorded for this report.", "  ")
		return strings.TrimRight(w.b.String(), "\n")
	}
	for index, section := range d.Sections {
		if index > 0 {
			w.blank()
		}
		w.heading(section.Title)
		for _, line := range section.Lines {
			if strings.TrimSpace(line) == "" {
				w.blank()
				continue
			}
			w.raw(line)
		}
	}
	return strings.TrimRight(w.b.String(), "\n")
}

// LimitationsSection 返回报告中记录的全部覆盖限制。
func (d Document) LimitationsSection(theme Theme) string {
	w := newWriter(theme)
	d.writeLimitations(w)
	return strings.TrimRight(w.b.String(), "\n")
}

// CompactSummary 返回可复制到问题跟踪器或聊天中的短摘要。
func (d Document) CompactSummary(theme Theme) string {
	w := newWriter(theme)
	w.heading("DEPSCOPE SUMMARY")
	target := d.Target
	status := StatusLabel(d.Status)
	succeeded, total := checkCoverage(d.Checks)
	w.line(fmt.Sprintf("%s · %s · %d/%d checks", target, status, succeeded, total))
	highest := "none"
	if d.Summary.HighestSeverity != nil {
		highest = SeverityLabel(*d.Summary.HighestSeverity)
	}
	w.line(fmt.Sprintf("Highest %s · confirmed %d · heuristic %d · data %s",
		highest, d.Summary.ConfirmedFindings, d.Summary.HeuristicWarnings, DataModeLabel(d.DataMode)))
	if len(d.Findings) == 0 {
		w.line("Findings: none in available data.")
	} else {
		w.line("Key findings:")
		limit := len(d.Findings)
		if limit > 5 {
			limit = 5
		}
		for _, finding := range d.Findings[:limit] {
			w.line(fmt.Sprintf("- [%s] [%s] %s — %s", SeverityLabel(finding.Severity), KindLabel(finding.Kind), finding.ID, finding.Title))
		}
		if len(d.Findings) > limit {
			w.line(fmt.Sprintf("- … %d more finding(s)", len(d.Findings)-limit))
		}
	}
	w.line("Sources: " + sourceList(d.Sources))
	if d.DataMode == model.DataModeDemo && d.DemoNotice != nil {
		w.line("Demo: " + *d.DemoNotice)
	}
	if d.Status == model.StatusIncomplete || d.Status == model.StatusFailed {
		w.line("Coverage gaps: " + coverageGapList(d.Checks))
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

func (w *writer) raw(text string) {
	w.line(truncate(text, w.theme.WidthForContent()))
}

func (w *writer) blank() {
	w.b.WriteString("\n")
}

func (w *writer) heading(text string) {
	w.line(w.theme.paint(strings.ToUpper(text), ansiBold, ansiAccent))
}

func (w *writer) paragraph(text, prefix string) {
	width := w.theme.WidthForContent() - runeCount(prefix)
	for _, line := range wrapText(text, width) {
		w.line(prefix + line)
	}
}

func (w *writer) bullet(text string) {
	w.paragraph("• "+text, "  ")
}

func (w *writer) keyValue(key, value string) {
	const keyWidth = 19
	plainKey := padRight(key, keyWidth)
	w.line("  " + w.theme.dim(plainKey) + "  " + value)
}

func (d Document) writeHeader(w *writer) {
	w.line(w.theme.paint(strings.ToUpper(d.Title), ansiBold, ansiAccent))
	for _, line := range wrapText(d.Target, w.theme.WidthForContent()) {
		w.line(w.theme.bold(line))
	}
	if d.Subtitle != "" {
		w.paragraph(d.Subtitle, "")
	}
	w.blank()
	w.keyValue("Data mode", DataModeLabel(d.DataMode))
	if !d.ScannedAt.IsZero() {
		w.keyValue("Scanned at", d.ScannedAt.UTC().Format(time.RFC3339))
	}
	if len(d.Sources) > 0 {
		w.keyValue("Sources", fmt.Sprintf("%d recorded", len(d.Sources)))
	}
	if d.DataMode == model.DataModeDemo && d.DemoNotice != nil {
		w.blank()
		prefix := w.theme.paint("[DEMO DATA]", ansiMedium, ansiBold) + " "
		width := w.theme.WidthForContent() - runeCount("[DEMO DATA] ")
		for _, line := range wrapText(*d.DemoNotice, width) {
			w.line(prefix + w.theme.paint(line, ansiMedium))
			prefix = strings.Repeat(" ", runeCount("[DEMO DATA] "))
		}
	}
}

func (d Document) writeAssessment(w *writer) {
	w.blank()
	w.heading("ASSESSMENT")
	w.keyValue("Status", w.theme.statusBadge(d.Status))
	succeeded, total := checkCoverage(d.Checks)
	coverage := fmt.Sprintf("%d of %d succeeded", succeeded, total)
	if total == 0 {
		coverage = "no checks recorded"
	}
	w.keyValue("Completed checks", coverage)
	w.keyValue("Confirmed findings", fmt.Sprint(d.Summary.ConfirmedFindings))
	w.keyValue("Heuristic warnings", fmt.Sprint(d.Summary.HeuristicWarnings))
	highest := "none"
	if d.Summary.HighestSeverity != nil {
		highest = w.theme.severityBadge(*d.Summary.HighestSeverity)
	}
	w.keyValue("Highest severity", highest)
	if d.Status == model.StatusIncomplete || d.Status == model.StatusFailed {
		w.keyValue("Interpretation", "coverage is incomplete; this result is not clean")
	}
}

func (d Document) writeCoverageGaps(w *writer, limitationLimit int) {
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
	w.heading("COVERAGE GAPS")
	for _, check := range gaps {
		w.line("  " + w.theme.checkBadge(check.Status) + " " + check.Name + "  " + w.theme.dim("["+check.ID+"]"))
		w.paragraph(check.Message, "      ")
	}
	if len(d.Limitations) > 0 {
		limit := len(d.Limitations)
		if limitationLimit > 0 && limit > limitationLimit {
			limit = limitationLimit
		}
		for _, limitation := range d.Limitations[:limit] {
			w.line("  " + w.theme.dim(limitation.Code) + ": " + limitation.Message)
		}
		if limit < len(d.Limitations) {
			w.line(w.theme.dim(fmt.Sprintf("  … %d more limitation(s)", len(d.Limitations)-limit)))
		}
	}
}

func (d Document) emptyFindingMessage() string {
	if d.Status == model.StatusIncomplete || d.Status == model.StatusFailed {
		return "No confirmed findings or heuristic warnings were recorded in the available data, but coverage is incomplete. This is not a clean result."
	}
	return "No confirmed findings or heuristic warnings were recorded."
}

func (d Document) writeFindingList(w *writer, selected int) {
	w.blank()
	w.heading(fmt.Sprintf("FINDINGS (%d)", len(d.Findings)))
	if len(d.Findings) == 0 {
		w.paragraph(d.emptyFindingMessage(), "  ")
		return
	}
	for index, finding := range d.Findings {
		marker := "  "
		if index == selected {
			marker = w.theme.accent("› ")
		}
		head := fmt.Sprintf("%s%d  %s %s %s", marker, index+1, w.theme.severityBadge(finding.Severity), w.theme.kindBadge(finding.Kind), finding.ID)
		w.line(head)
		w.paragraph(finding.Title, "     ")
		line := finding.Package.String()
		if finding.Version != nil && *finding.Version != "" {
			line += "@" + *finding.Version
		}
		if finding.Risk != nil {
			line += fmt.Sprintf(" · risk %.0f/100", finding.Risk.Score)
		}
		w.paragraph(line, "     ")
	}
}

func (d Document) writeFinding(w *writer, index int, finding model.Finding) {
	w.heading(fmt.Sprintf("FINDING %d OF %d", index+1, len(d.Findings)))
	w.blank()
	w.line(w.theme.severityBadge(finding.Severity) + " " + w.theme.kindBadge(finding.Kind) + " " + w.theme.bold(finding.Title))
	w.keyValue("ID", finding.ID)
	pkg := finding.Package.String()
	if finding.Version != nil && *finding.Version != "" {
		pkg += "@" + *finding.Version
	}
	w.keyValue("Package", pkg)
	if finding.Risk != nil {
		w.keyValue("Risk", fmt.Sprintf("%.0f/100", finding.Risk.Score))
	} else {
		w.keyValue("Risk", "not computed")
	}
	w.blank()
	w.paragraph(finding.Description, "  ")
	if finding.Risk != nil && strings.TrimSpace(finding.Risk.Rationale) != "" {
		w.blank()
		w.heading("RISK RATIONALE")
		w.paragraph(finding.Risk.Rationale, "  ")
	}
	if len(finding.Evidence) > 0 {
		w.blank()
		w.heading("EVIDENCE")
		for _, evidence := range finding.Evidence {
			w.line("  " + w.theme.dim("["+string(evidence.Kind)+"]") + " " + evidence.Summary)
			details := "source: " + evidence.SourceID
			if evidence.Reference != "" {
				details += " · reference: " + evidence.Reference
			}
			w.paragraph(details, "      ")
		}
	}
	if finding.Remediation != nil && strings.TrimSpace(*finding.Remediation) != "" {
		w.blank()
		w.heading("REMEDIATION")
		w.paragraph(*finding.Remediation, "  ")
	}
}

func (d Document) writeChecks(w *writer) {
	w.heading("CHECKS")
	succeeded, total := checkCoverage(d.Checks)
	w.paragraph(fmt.Sprintf("%d of %d checks succeeded. A missing or unsuccessful check prevents a clean result.", succeeded, total), "  ")
	if len(d.Checks) == 0 {
		w.blank()
		w.paragraph("No checks were recorded. The result cannot be considered clean.", "  ")
		return
	}
	for _, check := range d.Checks {
		w.blank()
		w.line("  " + w.theme.checkBadge(check.Status) + " " + w.theme.bold(check.Name) + "  " + w.theme.dim("["+check.ID+"]"))
		w.paragraph(check.Message, "      ")
		if len(check.SourceIDs) > 0 {
			w.paragraph("sources: "+strings.Join(check.SourceIDs, ", "), "      ")
		}
	}
}

func (d Document) writeDependencies(w *writer) {
	w.heading("DEPENDENCIES")
	w.paragraph(fmt.Sprintf("%d node(s) recorded. Node versions are exact when shown; unresolved nodes are never approximated.", nodeCount(d.Tree)), "  ")
	w.blank()
	if d.Tree == nil {
		w.paragraph("No dependency graph was recorded.", "  ")
		return
	}
	root, exists := d.Tree.Nodes[d.Tree.RootID]
	if !exists {
		w.paragraph("The dependency graph has no root node.", "  ")
		return
	}
	w.line("  " + root.Label)
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
		w.paragraph(fmt.Sprintf("%d node(s) are not linked from the root and are listed separately.", len(orphans)), "  ")
		for _, id := range orphans {
			node := d.Tree.Nodes[id]
			w.line("  " + w.theme.dim("•") + " " + node.Label)
			d.writeTreeChildren(w, node.Children, "    ", map[string]bool{id: true}, rendered)
		}
	}
	graphStatus := ""
	for _, check := range d.Checks {
		if check.ID == "dependency-graph" || strings.HasSuffix(check.ID, ":dependency-resolution") {
			if check.Status != model.CheckSucceeded {
				graphStatus = strings.ToLower(CheckLabel(check.Status))
				break
			}
		}
	}
	if graphStatus != "" {
		w.blank()
		w.paragraph("Dependency graph coverage is incomplete. See COVERAGE GAPS before relying on this tree.", "  ")
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
			marker = " (cycle)"
		}
		w.raw(prefix + branch + node.Label + marker)
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
	w.heading("SOURCES")
	if len(d.Sources) == 0 {
		w.paragraph("No source references were recorded. Results without sources cannot be clean.", "  ")
		return
	}
	for _, source := range d.Sources {
		line := fmt.Sprintf("%s  [%s]  %s  · retrieved %s",
			source.ID, source.Provider, source.Identifier, source.RetrievedAt.UTC().Format(time.RFC3339))
		w.paragraph(line, "  ")
		if source.URL != nil {
			w.paragraph("url: "+*source.URL, "    ")
		}
	}
}

func (d Document) writeLimitations(w *writer) {
	w.heading("LIMITATIONS")
	if len(d.Limitations) == 0 {
		w.paragraph("No additional limitations were recorded.", "  ")
		return
	}
	for _, limitation := range d.Limitations {
		w.line("  " + w.theme.dim(limitation.Code))
		w.paragraph(limitation.Message, "    ")
	}
}

func (d Document) writeActions(w *writer) {
	w.blank()
	w.heading("NEXT ACTIONS")
	actions := d.actions()
	for _, action := range actions {
		w.bullet(action)
	}
}

func (d Document) actions() []string {
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
				add("Review confirmed finding " + finding.ID + " and apply the referenced advisory guidance.")
			}
		}
		if strings.HasPrefix(finding.ID, "LIFECYCLE-") {
			add("Review the declared install-script commands before installing; DepScope did not execute them.")
		}
		if strings.HasPrefix(finding.ID, "TYPOSQUAT-") {
			add("Verify the package name and publisher against the official project before installing.")
		}
	}
	if d.Status == model.StatusIncomplete {
		add("Treat this result as incomplete, not clean, and restore coverage for: " + coverageGapList(d.Checks) + ".")
	}
	if d.Status == model.StatusFailed {
		add("The scan failed; resolve the coverage gaps before relying on this result.")
	}
	if d.Status == model.StatusClean {
		add("No action is required by the completed checks; re-run after dependency changes.")
	}
	if len(actions) == 0 && len(d.Findings) > 0 {
		add("Review each heuristic warning and its evidence before making a decision.")
	}
	if len(actions) == 0 {
		add("Review the report evidence and source coverage before making a decision.")
	}
	return actions
}

func actionsForFinding(finding model.Finding) []string {
	actions := []string{}
	if finding.Remediation != nil && strings.TrimSpace(*finding.Remediation) != "" {
		actions = append(actions, *finding.Remediation)
	}
	if strings.HasPrefix(finding.ID, "LIFECYCLE-") {
		actions = append(actions, "Inspect the lifecycle command in the evidence; DepScope never executes package scripts.")
	}
	if strings.HasPrefix(finding.ID, "TYPOSQUAT-") {
		actions = append(actions, "Confirm the package name and publisher with the official project before installing.")
	}
	if len(actions) == 0 {
		actions = append(actions, "Compare the evidence with the package's published release information.")
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

func coverageGapList(checks []model.CheckResult) string {
	names := []string{}
	for _, check := range checks {
		if check.Status != model.CheckSucceeded {
			names = append(names, check.Name+" ("+strings.ToLower(CheckLabel(check.Status))+")")
		}
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

func sourceList(sources []model.SourceReference) string {
	if len(sources) == 0 {
		return "none"
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
