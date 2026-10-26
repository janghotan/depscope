package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janghotan/depscope/internal/app"
	"github.com/janghotan/depscope/internal/i18n"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/render"
)

// iconOptions 返回强制指定图标模式的测试选项，避免依赖运行环境的 locale。
func iconOptions(mode string) Options {
	options := testOptions()
	options.Animations = "off"
	options.Icons = mode
	return options
}

func iconModel(t *testing.T, mode string, width, height int, language i18n.Language) *Model {
	t.Helper()
	options := iconOptions(mode)
	options.Language = language
	model := New(&fakeService{}, options)
	model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return model
}

// TestIconTokenTableIsComplete 验证每个语义图标都同时提供 Unicode 与 ASCII
// 字形，并且显示宽度可测量、不含空白或控制字符。
func TestIconTokenTableIsComplete(t *testing.T) {
	if len(iconGlyphs) != int(iconCount) {
		t.Fatalf("icon table has %d entries, want %d", len(iconGlyphs), iconCount)
	}
	for token := iconToken(0); token < iconCount; token++ {
		pair := iconGlyphs[token]
		if strings.TrimSpace(pair.unicode) == "" || strings.TrimSpace(pair.ascii) == "" {
			t.Fatalf("icon %d is missing a glyph: %+v", token, pair)
		}
		if strings.ContainsAny(pair.unicode+pair.ascii, " \t\r\n\x00\x1b") {
			t.Fatalf("icon %d contains whitespace or control characters: %+v", token, pair)
		}
		if width := displayWidth(pair.unicode); width < 1 || width > 2 {
			t.Fatalf("icon %d Unicode glyph %q is %d cells wide", token, pair.unicode, width)
		}
		if width := displayWidth(pair.ascii); width < 1 || width > 2 {
			t.Fatalf("icon %d ASCII glyph %q is %d cells wide", token, pair.ascii, width)
		}
	}
	for token, want := range map[iconToken]string{
		iconPackage:   "◈",
		iconProject:   "▤",
		iconCompare:   "⇄",
		iconInventory: "☰",
		iconDemo:      "▷",
		iconSettings:  "⚙",
		iconWarning:   "!",
		iconConfirmed: "●",
	} {
		if got := (iconMode{enabled: true}).glyph(token); got != want {
			t.Fatalf("icon %d Unicode glyph = %q, want %q", token, got, want)
		}
	}
}

// TestIconModeResolution 验证显式设置优先于环境变量，并覆盖所有自动回退条件。
func TestIconModeResolution(t *testing.T) {
	lookup := func(env map[string]string) func(string) string {
		return func(name string) string { return env[name] }
	}
	cases := []struct {
		name     string
		explicit string
		env      map[string]string
		want     iconMode
	}{
		{name: "default unicode", want: iconMode{enabled: true}},
		{name: "explicit off", explicit: "off", env: map[string]string{"DEPSCOPE_ASCII_ICONS": "1"}, want: iconMode{}},
		{name: "explicit ascii", explicit: "ascii", want: iconMode{enabled: true, ascii: true}},
		{name: "explicit on wins", explicit: "on", env: map[string]string{"DEPSCOPE_NO_ICONS": "1"}, want: iconMode{enabled: true}},
		{name: "environment disables", env: map[string]string{"DEPSCOPE_NO_ICONS": "1"}, want: iconMode{}},
		{name: "environment forces ascii", env: map[string]string{"DEPSCOPE_ASCII_ICONS": "1"}, want: iconMode{enabled: true, ascii: true}},
		{name: "dumb terminal", env: map[string]string{"TERM": "dumb"}, want: iconMode{enabled: true, ascii: true}},
		{name: "non utf8 locale", env: map[string]string{"LANG": "fr_FR.ISO-8859-1"}, want: iconMode{enabled: true, ascii: true}},
		{name: "c locale", env: map[string]string{"LANG": "C"}, want: iconMode{enabled: true, ascii: true}},
		{name: "utf8 locale", env: map[string]string{"LANG": "zh_CN.UTF-8"}, want: iconMode{enabled: true}},
		{name: "lc_all overrides lang", env: map[string]string{"LC_ALL": "C", "LANG": "en_US.UTF-8"}, want: iconMode{enabled: true, ascii: true}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveIconMode(test.explicit, lookup(test.env)); got != test.want {
				t.Fatalf("resolveIconMode(%q, %v) = %+v, want %+v", test.explicit, test.env, got, test.want)
			}
		})
	}
}

// TestStartScreenShowsSixWorkflowIcons 验证六个工作流都有独立的语义图标，
// 并且选中项同时保留箭头、图标与文字标签。
func TestStartScreenShowsSixWorkflowIcons(t *testing.T) {
	if len(menuEntries) != 6 {
		t.Fatalf("menu has %d workflows, want 6", len(menuEntries))
	}
	model := iconModel(t, "on", 100, 32, i18n.English)
	body := model.bodyText()
	seen := map[iconToken]bool{}
	for _, entry := range menuEntries {
		if seen[entry.icon] {
			t.Fatalf("workflow %q reuses icon token %d", model.text(entry.titleKey), entry.icon)
		}
		seen[entry.icon] = true
		want := model.icons.prefix(entry.icon) + model.text(entry.titleKey)
		if !strings.Contains(body, want) {
			t.Fatalf("workflow row missing %q:\n%s", want, body)
		}
	}
	selected := "› 1  " + model.icons.prefix(menuEntries[0].icon) + model.text(menuEntries[0].titleKey)
	if !strings.Contains(body, selected) {
		t.Fatalf("selected workflow is not marked by arrow, icon, and text:\n%s", body)
	}
}

// TestIconsDisabledKeepsOriginalNavigationText 验证关闭图标后菜单与设置行
// 回到原来的纯文本布局，不留下多余空格。
func TestIconsDisabledKeepsOriginalNavigationText(t *testing.T) {
	model := iconModel(t, "off", 100, 32, i18n.English)
	body := model.bodyText()
	if !strings.Contains(body, "› 1  "+model.text(menuEntries[0].titleKey)) {
		t.Fatalf("disabled icons changed the selected navigation row:\n%s", body)
	}
	for token := iconToken(0); token < iconCount; token++ {
		if glyph := iconGlyphs[token].unicode; strings.Contains(body, glyph) {
			t.Fatalf("disabled icons still render %q:\n%s", glyph, body)
		}
	}

	model.openScreen(screenSettings)
	settings := model.bodyText()
	want := "› " + padPlain(model.text(i18n.KeySettingsLanguageLabel), 20) + model.language.NativeName()
	if !strings.Contains(settings, want) {
		t.Fatalf("disabled icons changed the settings row:\n%s", settings)
	}
}

// TestASCIIFallbackAvoidsUnicodeIconGlyphs 验证 ASCII 模式只输出可移植字符。
func TestASCIIFallbackAvoidsUnicodeIconGlyphs(t *testing.T) {
	model := iconModel(t, "ascii", 100, 32, i18n.English)
	body := model.bodyText()
	for token := iconToken(0); token < iconCount; token++ {
		pair := iconGlyphs[token]
		if pair.unicode == pair.ascii {
			continue
		}
		if strings.Contains(body, pair.unicode) {
			t.Fatalf("ASCII mode still renders Unicode icon %q:\n%s", pair.unicode, body)
		}
	}
	if !strings.Contains(body, "› 1  o "+model.text(menuEntries[0].titleKey)) {
		t.Fatalf("ASCII workflow icon is missing:\n%s", body)
	}
	model.openScreen(screenSettings)
	label := model.text(i18n.KeySettingsLanguageLabel)
	if !strings.Contains(model.bodyText(), padPlain("A", 2)+" "+label) {
		t.Fatalf("ASCII language icon is missing:\n%s", model.bodyText())
	}
}

// TestReportIconsPreserveLabelsEvidenceAndValues 验证报告图标不改变状态、
// 严重性标签、证据、标识符或来源，并且在关闭图标时完全移除。
func TestReportIconsPreserveLabelsEvidenceAndValues(t *testing.T) {
	report := demoReport(t, "vulnerable")
	report.Checks = append(append([]model.CheckResult(nil), report.Checks...), model.CheckResult{
		ID: "icon:unavailable", Name: "Icon source", Status: model.CheckUnavailable,
		Message: "Source unavailable.", SourceIDs: []string{},
	})
	if len(report.Findings) == 0 || len(report.Findings[0].Evidence) == 0 {
		t.Fatal("demo fixture has no finding evidence")
	}
	finding := report.Findings[0]
	evidence := finding.Evidence[0]
	sourceID := report.Sources[0].ID
	packageName := finding.Package.String()

	onModel := iconModel(t, "on", 100, 64, i18n.English)
	onModel.showDocument(render.FromReport(report), screenMenu)
	onBody := onModel.bodyText()
	for _, want := range []string{
		"[FINDINGS]", "[CRITICAL]", "[CONFIRMED]",
		onModel.icons.glyph(iconCritical) + " [CRITICAL]",
		onModel.icons.glyph(iconConfirmed) + " [CONFIRMED]",
		finding.ID, packageName,
	} {
		if !strings.Contains(onBody, want) {
			t.Fatalf("icon report missing %q:\n%s", want, onBody)
		}
	}
	onModel.reportView = viewChecks
	if checks := onModel.bodyText(); !strings.Contains(checks, onModel.icons.glyph(iconCheckUnavailable)+" [UNAVAILABLE]") {
		t.Fatalf("unavailable check lost its icon or label:\n%s", checks)
	}
	onModel.reportView = viewSources
	if sources := onModel.bodyText(); !strings.Contains(sources, sourceID) {
		t.Fatalf("source identifier changed with icons enabled:\n%s", sources)
	}
	onModel.reportView = viewFinding
	if detail := onModel.bodyText(); !strings.Contains(detail, evidence.SourceID) {
		t.Fatalf("evidence source changed with icons enabled:\n%s", detail)
	}
	onModel.reportView = viewSummary
	if summary := onModel.bodyText(); !strings.Contains(summary, "[CRITICAL]") || !strings.Contains(summary, "[CONFIRMED]") {
		t.Fatalf("summary lost bracket labels:\n%s", summary)
	}

	offModel := iconModel(t, "off", 100, 64, i18n.English)
	offModel.showDocument(render.FromReport(report), screenMenu)
	offBody := offModel.bodyText()
	for _, want := range []string{"[FINDINGS]", "[CRITICAL]", "[CONFIRMED]", finding.ID, packageName} {
		if !strings.Contains(offBody, want) {
			t.Fatalf("plain report missing %q:\n%s", want, offBody)
		}
	}
	for token := iconToken(0); token < iconCount; token++ {
		if glyph := iconGlyphs[token].unicode; strings.Contains(offBody, glyph) {
			t.Fatalf("disabled icons still appear in the report as %q:\n%s", glyph, offBody)
		}
	}
}

// TestBilingualIconLayoutsFitSupportedSizes 覆盖六种工作流、报告与对比，
// 在支持的最小到最大尺寸、两种语言和三种图标模式下检查显示宽度。
func TestBilingualIconLayoutsFitSupportedSizes(t *testing.T) {
	report := demoReport(t, "vulnerable")
	for _, language := range []i18n.Language{i18n.English, i18n.SimplifiedChinese} {
		for _, size := range [][2]int{{46, 14}, {60, 30}, {80, 24}, {100, 32}, {140, 40}} {
			for _, mode := range []string{"on", "ascii", "off"} {
				screenModel := iconModel(t, mode, size[0], size[1], language)
				assertBodyFits(t, screenModel, size[0])

				for _, target := range []screen{screenPackageForm, screenProjectForm, screenCompareForm, screenInventoryConfirm, screenDemoPicker, screenSettings} {
					screenModel.openScreen(target)
					assertBodyFits(t, screenModel, size[0])
				}

				screenModel.showDocument(render.FromReport(report, language), screenMenu)
				for _, view := range reportViews {
					screenModel.reportView = view
					assertBodyFits(t, screenModel, size[0])
				}

				left := report
				screenModel.comparison = &app.Comparison{
					LeftRequest:  app.PackageRequest{Ecosystem: model.EcosystemNPM, Name: "left"},
					RightRequest: app.PackageRequest{Ecosystem: model.EcosystemNPM, Name: "right"},
					Left:         &left,
					RightErr:     nil,
				}
				screenModel.screen = screenComparison
				screenModel.scroll = 0
				assertBodyFits(t, screenModel, size[0])
			}
		}
	}
}

// TestMonochromeIconsIntroduceNoANSI 验证关闭颜色时，图标仍然是纯文本。
func TestMonochromeIconsIntroduceNoANSI(t *testing.T) {
	for _, mode := range []string{"on", "ascii"} {
		model := iconModel(t, mode, 100, 32, i18n.English)
		assertNoANSI(t, model.View())
		model.openScreen(screenSettings)
		assertNoANSI(t, model.bodyText())
	}
}

// TestOnlySpinnerAnimates 验证加载页的图标保持静态，只有现有 spinner 逐帧变化。
func TestOnlySpinnerAnimates(t *testing.T) {
	model := iconModel(t, "on", 100, 32, i18n.English)
	model.anim.enabled = true
	model.screen = screenLoading
	model.loadingName = "CHECKING PACKAGE"
	model.loading = app.Progress{Stage: "scan", Message: "Checking."}
	model.anim.spinner = 1
	first := model.View()
	model.anim.spinner = 4
	second := model.View()
	if first == second {
		t.Fatal("spinner frames did not change")
	}
	strip := func(value string) string {
		return strings.Map(func(r rune) rune {
			if strings.ContainsRune(spinnerFrames, r) {
				return -1
			}
			return r
		}, value)
	}
	if strip(first) != strip(second) {
		t.Fatalf("non-spinner content changed between frames:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if !strings.Contains(first, model.icons.glyph(iconLoading)) {
		t.Fatalf("static loading icon is missing:\n%s", first)
	}
}

// TestSettingsIconNoteIsLocalized 验证设置页说明图标控制方式，并且中文文案
// 没有回退成英语。
func TestSettingsIconNoteIsLocalized(t *testing.T) {
	english := i18n.For(i18n.English).Text(i18n.KeySettingsIconNote)
	chinese := i18n.For(i18n.SimplifiedChinese).Text(i18n.KeySettingsIconNote)
	if strings.TrimSpace(english) == "" || strings.TrimSpace(chinese) == "" {
		t.Fatal("icon note is not present in both catalogs")
	}
	if english == chinese {
		t.Fatal("Chinese icon note still uses English text")
	}
	for _, language := range []i18n.Language{i18n.English, i18n.SimplifiedChinese} {
		model := iconModel(t, "on", 100, 40, language)
		model.openScreen(screenSettings)
		note := i18n.For(language).Text(i18n.KeySettingsIconNote)
		if compactText(model.bodyText()) == "" || !strings.Contains(compactText(model.bodyText()), compactText(note)) {
			t.Fatalf("%s settings screen is missing the icon note:\n%s", language, model.bodyText())
		}
	}
}

// TestSettingsIconColumnAlignsLabels 验证 CJK 宽图标不会把设置标签推离列。
func TestSettingsIconColumnAlignsLabels(t *testing.T) {
	for _, language := range []i18n.Language{i18n.English, i18n.SimplifiedChinese} {
		for _, mode := range []string{"on", "ascii"} {
			model := iconModel(t, mode, 100, 40, language)
			model.openScreen(screenSettings)
			body := model.bodyText()
			column := -1
			for _, key := range []i18n.Key{
				i18n.KeySettingsLanguageLabel,
				i18n.KeySettingsEcosystemLabel,
				i18n.KeySettingsColorLabel,
				i18n.KeySettingsAnimationLabel,
			} {
				label := model.text(key)
				index := strings.Index(body, label)
				if index < 0 {
					t.Fatalf("%s %s: settings label %q is missing:\n%s", language, mode, label, body)
				}
				lineStart := strings.LastIndex(body[:index], "\n") + 1
				got := displayWidth(body[lineStart:index])
				if column < 0 {
					column = got
					continue
				}
				if got != column {
					t.Fatalf("%s %s: label %q starts at column %d, want %d:\n%s", language, mode, label, got, column, body)
				}
			}
			if column != marginWidth+5 {
				t.Fatalf("%s %s: label column = %d, want %d:\n%s", language, mode, column, marginWidth+5, body)
			}
		}
	}
}

func assertBodyFits(t *testing.T, model *Model, width int) {
	t.Helper()
	limit := width - marginWidth
	for index, line := range strings.Split(model.bodyText(), "\n") {
		if got := displayWidth(line); got > limit {
			t.Fatalf("%dx%d %s: body line %d is %d columns, want <= %d: %q\nglyph=%s mode=%+v",
				model.width, model.height, model.language, index, got, limit, line, model.icons.glyph(iconPackage), model.icons)
		}
	}
}
