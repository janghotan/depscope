package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/janghotan/depscope/internal/i18n"
	"github.com/janghotan/depscope/internal/model"
)

const (
	compareLabelWidth = 17
	compareMinColumns = 76
)

// Comparison 并排展示两个包扫描结果，并在窄终端中自动改为上下排列。
// 标签与界面文案随主题语言本地化；包名、版本、公告与来源保持原始证据。
func Comparison(leftRequest, rightRequest string, left, right *model.ScanReport, leftErr, rightErr error, theme Theme) string {
	leftRequest = Sanitize(leftRequest)
	rightRequest = Sanitize(rightRequest)
	left = sanitizeReportPointer(left)
	right = sanitizeReportPointer(right)
	w := newWriter(theme)
	w.heading(theme.text(i18n.KeyComparisonTitle))
	w.paragraph(leftRequest+theme.text(i18n.KeyComparisonVs)+rightRequest, "")
	if isDemoReport(left) || isDemoReport(right) {
		w.blank()
		badge := "[" + theme.text(i18n.KeyReportDemoBadge) + "] "
		w.prefixedStyled(
			theme.paint(badge, ansiMedium, ansiBold),
			theme.text(i18n.KeyComparisonDemoWarning),
			func(value string) string { return theme.paint(value, ansiMedium) },
		)
	}
	w.blank()

	leftData := summarizeSide(theme, i18n.KeyComparisonLeft, leftRequest, left, leftErr)
	rightData := summarizeSide(theme, i18n.KeyComparisonRight, rightRequest, right, rightErr)
	rows := comparisonRows(theme, leftData, rightData)
	if theme.WidthForContent() >= compareMinColumns {
		columnWidth := compareColumnWidth(theme)
		w.line(theme.dim(padRight("", compareLabelWidth)) + "  " + theme.bold(padRight(leftData.label, columnWidth)) + "  " + theme.bold(rightData.label))
		for _, row := range rows {
			leftCell := padRight(truncate(row.left, columnWidth), columnWidth)
			rightCell := truncate(row.right, columnWidth)
			w.line(theme.dim(padRight(row.label, compareLabelWidth)) + "  " + leftCell + "  " + rightCell)
		}
	} else {
		writeStackedSide(w, leftData)
		w.blank()
		writeStackedSide(w, rightData)
	}

	w.blank()
	w.heading(theme.text(i18n.KeyComparisonFindingsLeft))
	writeSideFindings(w, leftData)
	w.blank()
	w.heading(theme.text(i18n.KeyComparisonFindingsRight))
	writeSideFindings(w, rightData)
	w.blank()
	w.heading(theme.text(i18n.KeyComparisonSourceCoverage))
	w.paragraph(padRight(leftData.label, 7)+sourceCoverage(theme, leftData), "  ")
	w.paragraph(padRight(rightData.label, 7)+sourceCoverage(theme, rightData), "  ")
	return strings.TrimRight(w.b.String(), "\n")
}

type sideQuality int

const (
	qualityUnavailable sideQuality = iota
	qualityPartial
	qualityComplete
)

type sideSummary struct {
	label         string
	request       string
	report        *model.ScanReport
	err           error
	quality       sideQuality
	packageName   string
	version       string
	status        string
	checks        string
	confirmed     string
	heuristic     string
	highest       string
	topFinding    string
	lifecycle     string
	typosquat     string
	sources       string
	dataMode      string
	findings      []model.Finding
	uniqueSources []string
}

func summarizeSide(theme Theme, labelKey i18n.Key, request string, report *model.ScanReport, err error) sideSummary {
	dash := "—"
	side := sideSummary{
		label:       theme.text(labelKey),
		request:     request,
		report:      report,
		err:         err,
		packageName: dash,
		version:     dash,
		status:      theme.text(i18n.KeyComparisonStatusUnavailable),
		checks:      dash,
		confirmed:   dash,
		heuristic:   dash,
		highest:     dash,
		topFinding:  dash,
		lifecycle:   dash,
		typosquat:   dash,
		sources:     dash,
		dataMode:    dash,
		quality:     qualityUnavailable,
	}
	if err != nil || report == nil {
		return side
	}

	side.quality = sideDataQuality(*report)
	side.packageName = report.Target.Package.String()
	side.version = theme.text(i18n.KeyComparisonNotResolved)
	if report.Target.ResolvedVersion != nil && *report.Target.ResolvedVersion != "" {
		side.version = *report.Target.ResolvedVersion
	}
	side.status = theme.statusLabel(report.Status)
	succeeded, total := checkCoverage(report.Checks)
	side.checks = theme.format(i18n.KeyComparisonChecks, succeeded, total)
	side.confirmed = fmt.Sprint(report.Summary.ConfirmedFindings)
	side.heuristic = fmt.Sprint(report.Summary.HeuristicWarnings)
	if report.Summary.HighestSeverity != nil {
		side.highest = theme.severityLabel(*report.Summary.HighestSeverity)
	}
	side.findings = report.Findings
	if len(report.Findings) > 0 {
		top := report.Findings[0]
		side.topFinding = fmt.Sprintf("[%s] %s — %s", theme.severityLabel(top.Severity), theme.kindLabel(top.Kind), top.Title)
	}
	lifecycle := findingsWithPrefix(report.Findings, "LIFECYCLE-")
	typosquat := findingsWithPrefix(report.Findings, "TYPOSQUAT-")
	side.lifecycle = theme.text(i18n.KeyComparisonNo)
	if lifecycle > 0 {
		side.lifecycle = theme.format(i18n.KeyComparisonYes, lifecycle)
	}
	side.typosquat = theme.text(i18n.KeyComparisonNo)
	if typosquat > 0 {
		side.typosquat = theme.format(i18n.KeyComparisonYes, typosquat)
	}
	side.uniqueSources = uniqueProviders(report.Sources)
	side.sources = theme.format(i18n.KeyComparisonSources, strings.Join(side.uniqueSources, ", "), len(report.Sources))
	if len(report.Sources) == 0 {
		side.sources = theme.text(i18n.KeyComparisonSourcesNone)
	}
	side.dataMode = theme.dataModeLabel(report.DataMode)
	return side
}

func qualityLabel(theme Theme, quality sideQuality) string {
	switch quality {
	case qualityPartial:
		return theme.text(i18n.KeyComparisonQualityPartial)
	case qualityComplete:
		return theme.text(i18n.KeyComparisonQualityComplete)
	default:
		return theme.text(i18n.KeyComparisonQualityUnavailable)
	}
}

type comparisonRow struct {
	label string
	left  string
	right string
}

func comparisonRows(theme Theme, left, right sideSummary) []comparisonRow {
	return []comparisonRow{
		{label: theme.text(i18n.KeyComparisonRowPackage), left: left.packageName, right: right.packageName},
		{label: theme.text(i18n.KeyComparisonRowVersion), left: left.version, right: right.version},
		{label: theme.text(i18n.KeyComparisonRowStatus), left: left.status, right: right.status},
		{label: theme.text(i18n.KeyComparisonRowQuality), left: qualityLabel(theme, left.quality), right: qualityLabel(theme, right.quality)},
		{label: theme.text(i18n.KeyComparisonRowChecks), left: left.checks, right: right.checks},
		{label: theme.text(i18n.KeyComparisonRowConfirmed), left: left.confirmed, right: right.confirmed},
		{label: theme.text(i18n.KeyComparisonRowHeuristic), left: left.heuristic, right: right.heuristic},
		{label: theme.text(i18n.KeyComparisonRowHighest), left: left.highest, right: right.highest},
		{label: theme.text(i18n.KeyComparisonRowTopFinding), left: left.topFinding, right: right.topFinding},
		{label: theme.text(i18n.KeyComparisonRowLifecycle), left: left.lifecycle, right: right.lifecycle},
		{label: theme.text(i18n.KeyComparisonRowTyposquat), left: left.typosquat, right: right.typosquat},
		{label: theme.text(i18n.KeyComparisonRowSources), left: left.sources, right: right.sources},
		{label: theme.text(i18n.KeyComparisonRowDataMode), left: left.dataMode, right: right.dataMode},
	}
}

func writeStackedSide(w *writer, side sideSummary) {
	w.prefixedStyled(w.theme.bold(side.label)+"  ", side.request, nil)
	for _, row := range comparisonRows(w.theme, side, sideSummary{}) {
		w.keyValue(row.label, row.left)
	}
}

func writeSideFindings(w *writer, side sideSummary) {
	theme := w.theme
	if side.err != nil {
		w.paragraph(theme.format(i18n.KeyComparisonDataUnavailable, Sanitize(side.err.Error())), "  ")
		return
	}
	if side.report == nil {
		w.paragraph(theme.text(i18n.KeyComparisonNoReport), "  ")
		return
	}
	if len(side.findings) == 0 {
		if side.quality == qualityPartial {
			w.paragraph(theme.text(i18n.KeyComparisonNoFindingsPartial), "  ")
		} else {
			w.paragraph(theme.text(i18n.KeyComparisonNoFindingsComplete), "  ")
		}
		return
	}
	for _, finding := range side.findings {
		w.prefixedStyled("  "+theme.severityBadge(finding.Severity)+" "+theme.kindBadge(finding.Kind)+" ", finding.ID, theme.dim)
		w.paragraph(finding.Title, "      ")
	}
}

func sideDataQuality(report model.ScanReport) sideQuality {
	if report.Status == model.StatusIncomplete || report.Status == model.StatusFailed || report.Status == model.StatusCancelled {
		return qualityPartial
	}
	succeeded, total := checkCoverage(report.Checks)
	if total == 0 || succeeded != total {
		return qualityPartial
	}
	return qualityComplete
}

func sourceCoverage(theme Theme, side sideSummary) string {
	if side.err != nil || side.report == nil {
		return theme.text(i18n.KeyComparisonUnavailable)
	}
	if len(side.report.Sources) == 0 {
		return theme.text(i18n.KeyComparisonNoSourceRefs)
	}
	return theme.format(i18n.KeyComparisonSourceRefs, strings.Join(side.uniqueSources, ", "), len(side.report.Sources))
}

func compareColumnWidth(theme Theme) int {
	width := (theme.WidthForContent() - compareLabelWidth - 4) / 2
	if width < 10 {
		return 10
	}
	return width
}

func findingsWithPrefix(findings []model.Finding, prefix string) int {
	count := 0
	for _, finding := range findings {
		if strings.HasPrefix(finding.ID, prefix) {
			count++
		}
	}
	return count
}

func uniqueProviders(sources []model.SourceReference) []string {
	seen := map[string]bool{}
	providers := []string{}
	for _, source := range sources {
		value := string(source.Provider)
		if !seen[value] {
			seen[value] = true
			providers = append(providers, value)
		}
	}
	sort.Strings(providers)
	return providers
}

func isDemoReport(report *model.ScanReport) bool {
	return report != nil && report.DataMode == model.DataModeDemo
}
