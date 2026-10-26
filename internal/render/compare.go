package render

import (
	"fmt"
	"sort"
	"strings"

	"depscope/internal/model"
)

const (
	compareLabelWidth = 17
	compareMinColumns = 76
)

// Comparison 并排展示两个包扫描结果，并在窄终端中自动改为上下排列。
func Comparison(leftRequest, rightRequest string, left, right *model.ScanReport, leftErr, rightErr error, theme Theme) string {
	leftRequest = Sanitize(leftRequest)
	rightRequest = Sanitize(rightRequest)
	left = sanitizeReportPointer(left)
	right = sanitizeReportPointer(right)
	w := newWriter(theme)
	w.heading("PACKAGE COMPARISON")
	w.paragraph(leftRequest+"   vs   "+rightRequest, "")
	if isDemoReport(left) || isDemoReport(right) {
		w.blank()
		w.line(w.theme.paint("[DEMO DATA] ", ansiMedium, ansiBold) + w.theme.paint("This comparison includes synthetic demo data; it is not a real security assessment.", ansiMedium))
	}
	w.blank()

	leftData := summarizeSide("LEFT", leftRequest, left, leftErr)
	rightData := summarizeSide("RIGHT", rightRequest, right, rightErr)
	rows := comparisonRows(leftData, rightData)
	if theme.WidthForContent() >= compareMinColumns {
		w.line(w.theme.dim(padRight("", compareLabelWidth)) + "  " + w.theme.bold(padRight("LEFT", compareColumnWidth(theme))) + "  " + w.theme.bold("RIGHT"))
		for _, row := range rows {
			leftCell := padRight(truncate(row.left, compareColumnWidth(theme)), compareColumnWidth(theme))
			rightCell := truncate(row.right, compareColumnWidth(theme))
			w.line(w.theme.dim(padRight(row.label, compareLabelWidth)) + "  " + leftCell + "  " + rightCell)
		}
	} else {
		writeStackedSide(w, leftData)
		w.blank()
		writeStackedSide(w, rightData)
	}

	w.blank()
	w.heading("FINDINGS — LEFT")
	writeSideFindings(w, leftData)
	w.blank()
	w.heading("FINDINGS — RIGHT")
	writeSideFindings(w, rightData)
	w.blank()
	w.heading("SOURCE COVERAGE")
	w.paragraph("LEFT   "+sourceCoverage(leftData), "  ")
	w.paragraph("RIGHT  "+sourceCoverage(rightData), "  ")
	return strings.TrimRight(w.b.String(), "\n")
}

type sideSummary struct {
	label         string
	request       string
	report        *model.ScanReport
	err           error
	quality       string
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

func summarizeSide(label, request string, report *model.ScanReport, err error) sideSummary {
	side := sideSummary{
		label:       label,
		request:     request,
		report:      report,
		err:         err,
		packageName: "—",
		version:     "—",
		status:      "UNAVAILABLE",
		checks:      "—",
		confirmed:   "—",
		heuristic:   "—",
		highest:     "—",
		topFinding:  "—",
		lifecycle:   "—",
		typosquat:   "—",
		sources:     "—",
		dataMode:    "—",
		quality:     "UNAVAILABLE",
	}
	if err != nil || report == nil {
		return side
	}

	side.quality = sideDataQuality(*report)
	side.packageName = report.Target.Package.String()
	side.version = "not resolved"
	if report.Target.ResolvedVersion != nil && *report.Target.ResolvedVersion != "" {
		side.version = *report.Target.ResolvedVersion
	}
	side.status = StatusLabel(report.Status)
	succeeded, total := checkCoverage(report.Checks)
	side.checks = fmt.Sprintf("%d/%d succeeded", succeeded, total)
	side.confirmed = fmt.Sprint(report.Summary.ConfirmedFindings)
	side.heuristic = fmt.Sprint(report.Summary.HeuristicWarnings)
	if report.Summary.HighestSeverity != nil {
		side.highest = SeverityLabel(*report.Summary.HighestSeverity)
	}
	side.findings = report.Findings
	if len(report.Findings) > 0 {
		top := report.Findings[0]
		side.topFinding = fmt.Sprintf("[%s] %s — %s", SeverityLabel(top.Severity), KindLabel(top.Kind), top.Title)
	}
	lifecycle := findingsWithPrefix(report.Findings, "LIFECYCLE-")
	typosquat := findingsWithPrefix(report.Findings, "TYPOSQUAT-")
	side.lifecycle = "No"
	if lifecycle > 0 {
		side.lifecycle = fmt.Sprintf("Yes (%d)", lifecycle)
	}
	side.typosquat = "No"
	if typosquat > 0 {
		side.typosquat = fmt.Sprintf("Yes (%d)", typosquat)
	}
	side.uniqueSources = uniqueProviders(report.Sources)
	side.sources = fmt.Sprintf("%s (%d)", strings.Join(side.uniqueSources, ", "), len(report.Sources))
	if len(report.Sources) == 0 {
		side.sources = "none"
	}
	side.dataMode = DataModeLabel(report.DataMode)
	return side
}

type comparisonRow struct {
	label string
	left  string
	right string
}

func comparisonRows(left, right sideSummary) []comparisonRow {
	rows := []comparisonRow{
		{label: "Package", left: left.packageName, right: right.packageName},
		{label: "Version", left: left.version, right: right.version},
		{label: "Status", left: left.status, right: right.status},
		{label: "Data quality", left: left.quality, right: right.quality},
		{label: "Checks", left: left.checks, right: right.checks},
		{label: "Confirmed", left: left.confirmed, right: right.confirmed},
		{label: "Heuristic", left: left.heuristic, right: right.heuristic},
		{label: "Highest severity", left: left.highest, right: right.highest},
		{label: "Top finding", left: left.topFinding, right: right.topFinding},
		{label: "Lifecycle scripts", left: left.lifecycle, right: right.lifecycle},
		{label: "Typosquat signal", left: left.typosquat, right: right.typosquat},
		{label: "Sources", left: left.sources, right: right.sources},
		{label: "Data mode", left: left.dataMode, right: right.dataMode},
	}
	return rows
}

func writeStackedSide(w *writer, side sideSummary) {
	w.line(w.theme.bold(side.label + "  " + side.request))
	for _, row := range comparisonRows(side, sideSummary{}) {
		value := row.left
		w.keyValue(row.label, value)
	}
}

func writeSideFindings(w *writer, side sideSummary) {
	if side.err != nil {
		w.paragraph("Data unavailable: "+Sanitize(side.err.Error()), "  ")
		return
	}
	if side.report == nil {
		w.paragraph("No report was produced.", "  ")
		return
	}
	if len(side.findings) == 0 {
		if side.quality == "PARTIAL" {
			w.paragraph("No findings in the available data; coverage is incomplete.", "  ")
		} else {
			w.paragraph("No findings in completed checks.", "  ")
		}
		return
	}
	for _, finding := range side.findings {
		w.line("  " + w.theme.severityBadge(finding.Severity) + " " + w.theme.kindBadge(finding.Kind) + " " + finding.ID)
		w.paragraph(finding.Title, "      ")
	}
}

func sideDataQuality(report model.ScanReport) string {
	if report.Status == model.StatusIncomplete || report.Status == model.StatusFailed || report.Status == model.StatusCancelled {
		return "PARTIAL"
	}
	succeeded, total := checkCoverage(report.Checks)
	if total == 0 || succeeded != total {
		return "PARTIAL"
	}
	return "COMPLETE"
}

func sourceCoverage(side sideSummary) string {
	if side.err != nil || side.report == nil {
		return "unavailable"
	}
	if len(side.report.Sources) == 0 {
		return "no source references recorded"
	}
	return fmt.Sprintf("%s · %d reference(s)", strings.Join(side.uniqueSources, ", "), len(side.report.Sources))
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
