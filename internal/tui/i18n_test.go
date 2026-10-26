package tui

import (
	"context"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janghotan/depscope/internal/app"
	"github.com/janghotan/depscope/internal/demo"
	"github.com/janghotan/depscope/internal/i18n"
	domain "github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/render"
)

func languageOptions(language i18n.Language) Options {
	options := testOptions()
	options.Language = language
	options.Animations = "off"
	return options
}

func sizedLanguageModel(t *testing.T, language i18n.Language, width, height int, color bool) *Model {
	t.Helper()
	options := languageOptions(language)
	if color {
		options.ColorMode = "always"
	}
	model := New(&fakeService{}, options)
	model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return model
}

func demoReport(t *testing.T, scenario string) domain.ScanReport {
	t.Helper()
	report, err := demo.Scan(context.Background(), scenario)
	if err != nil {
		t.Fatalf("demo scan %q: %v", scenario, err)
	}
	return report
}

// compactText 去掉换行与空白，便于断言被自动换行的本地化句子。
func compactText(value string) string {
	return strings.Join(strings.Fields(value), "")
}

func assertContains(t *testing.T, view string, language i18n.Language, keys ...i18n.Key) {
	t.Helper()
	catalog := i18n.For(language)
	compactView := compactText(view)
	for _, key := range keys {
		if text := catalog.Text(key); !strings.Contains(compactView, compactText(text)) {
			t.Fatalf("%s view is missing %q:\n%s", language, text, view)
		}
	}
}

func TestLanguageDefaultsToEnglish(t *testing.T) {
	model := New(&fakeService{}, Options{
		ColorMode: "never",
		LookupEnv: func(string) string { return "" },
	})
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	if model.language != i18n.English {
		t.Fatalf("language = %q, want English", model.language)
	}
	view := model.View()
	assertContains(t, view, i18n.English, i18n.KeyMenuPackageTitle, i18n.KeyMenuSettingsTitle)
	if strings.Contains(view, "检查单个包") {
		t.Fatalf("English default view contains Chinese chrome:\n%s", view)
	}
}

func TestUnknownLanguageFallsBackToEnglish(t *testing.T) {
	model := New(&fakeService{}, Options{
		ColorMode: "never",
		LookupEnv: func(string) string { return "" },
		Language:  i18n.Language("fr"),
	})
	if model.language != i18n.English {
		t.Fatalf("language = %q, want English fallback", model.language)
	}
}

func TestSettingsLanguageSwitchIsImmediateAndSessionOnly(t *testing.T) {
	model := sizedLanguageModel(t, i18n.English, 100, 32, false)
	model.menuIndex = 5
	model.ecosystem = domain.EcosystemPyPI
	send(model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.screen != screenSettings || model.settingsAt != 0 {
		t.Fatalf("screen = %v row = %d, want settings language row", model.screen, model.settingsAt)
	}
	assertContains(t, model.View(), i18n.English, i18n.KeySettingsLanguageLabel, i18n.KeyLanguageEnglish)

	send(model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.language != i18n.SimplifiedChinese {
		t.Fatalf("language = %q, want Simplified Chinese", model.language)
	}
	if model.screen != screenSettings || model.settingsAt != 0 || model.menuIndex != 5 {
		t.Fatalf("state changed unexpectedly: screen=%v row=%d menu=%d", model.screen, model.settingsAt, model.menuIndex)
	}
	if model.ecosystem != domain.EcosystemPyPI {
		t.Fatal("changing language reset another setting")
	}
	view := model.View()
	assertContains(t, view, i18n.SimplifiedChinese, i18n.KeySettingsTitle, i18n.KeySettingsLanguageLabel, i18n.KeyLanguageChinese)

	send(model, tea.KeyMsg{Type: tea.KeyRight})
	if model.language != i18n.English {
		t.Fatalf("right did not switch back: %q", model.language)
	}
	if !strings.Contains(model.View(), i18n.For(i18n.English).Text(i18n.KeyLanguageEnglish)) {
		t.Fatalf("language row does not display the English native name:\n%s", model.View())
	}
	send(model, tea.KeyMsg{Type: tea.KeyLeft})
	if model.language != i18n.SimplifiedChinese {
		t.Fatalf("left did not switch back to Chinese: %q", model.language)
	}

	send(model, tea.KeyMsg{Type: tea.KeyEsc})
	if model.screen != screenMenu || model.menuIndex != 5 {
		t.Fatalf("screen = %v menu = %d, want menu selection preserved", model.screen, model.menuIndex)
	}
	assertContains(t, model.View(), i18n.SimplifiedChinese, i18n.KeyMenuCompareTitle, i18n.KeyMenuSettingsTitle)
}

func TestLanguageSwitchPreservesFormInput(t *testing.T) {
	model := sizedLanguageModel(t, i18n.English, 100, 32, false)
	model.openScreen(screenPackageForm)
	typeText(t, model, "npm:@depscope-demo/vulnerable-package@1.0.0")
	before := model.formFields[0].String()

	model.applyLanguage(i18n.SimplifiedChinese)
	if model.screen != screenPackageForm {
		t.Fatalf("language switch changed screen to %v", model.screen)
	}
	if got := model.formFields[0].String(); got != before {
		t.Fatalf("language switch rewrote form input: %q -> %q", before, got)
	}
	if model.formFields[0].cursor != len([]rune(before)) {
		t.Fatalf("language switch moved the cursor to %d", model.formFields[0].cursor)
	}
	assertContains(t, model.View(), i18n.SimplifiedChinese, i18n.KeyFormPackageLabel)
}

func TestAllSixWorkflowsLocalizeInBothLanguages(t *testing.T) {
	for _, language := range []i18n.Language{i18n.English, i18n.SimplifiedChinese} {
		catalog := i18n.For(language)
		model := sizedLanguageModel(t, language, 100, 32, false)
		menu := model.View()
		for _, entry := range menuEntries {
			if text := catalog.Text(entry.titleKey); !strings.Contains(menu, text) {
				t.Fatalf("%s menu missing %q:\n%s", language, text, menu)
			}
		}
		assertContains(t, menu, language, i18n.KeyBrandTagline)

		forms := []struct {
			key     string
			screen  screen
			content []i18n.Key
		}{
			{key: "1", screen: screenPackageForm, content: []i18n.Key{i18n.KeyFormPackageTitle, i18n.KeyFormPackageLabel, i18n.KeyFormPackageExamples}},
			{key: "2", screen: screenProjectForm, content: []i18n.Key{i18n.KeyFormProjectTitle, i18n.KeyFormProjectLabel, i18n.KeyFormProjectHint}},
			{key: "3", screen: screenCompareForm, content: []i18n.Key{i18n.KeyFormCompareTitle, i18n.KeyFormCompareLeft, i18n.KeyFormCompareRight}},
			{key: "4", screen: screenInventoryConfirm, content: []i18n.Key{i18n.KeyInventoryTitle, i18n.KeyInventoryNoExecute, i18n.KeyInventoryPressEnter}},
			{key: "5", screen: screenDemoPicker, content: []i18n.Key{i18n.KeyDemoTitle, i18n.KeyDemoScenarioVulnerableTitle, i18n.KeyDemoNote}},
			{key: "6", screen: screenSettings, content: []i18n.Key{i18n.KeySettingsTitle, i18n.KeySettingsLanguageLabel, i18n.KeySettingsLanguageNote}},
		}
		for _, form := range forms {
			send(model, tea.KeyMsg{Type: tea.KeyEsc})
			send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(form.key)})
			if model.screen != form.screen {
				t.Fatalf("%s key %s opened %v, want %v", language, form.key, model.screen, form.screen)
			}
			assertContains(t, model.View(), language, form.content...)
		}
	}
}

func TestLoadingTitlesLocalize(t *testing.T) {
	cases := []struct {
		key       string
		titleKey  i18n.Key
		configure func(*Model)
	}{
		{key: "1", titleKey: i18n.KeyLoadingCheckingPackage, configure: func(m *Model) { m.formFields[0].SetValue("npm:fixture@1.0.0") }},
		{key: "2", titleKey: i18n.KeyLoadingScanningProject, configure: func(m *Model) { m.formFields[0].SetValue(".") }},
		{key: "3", titleKey: i18n.KeyLoadingComparing, configure: func(m *Model) {
			m.formFields[0].SetValue("npm:left@1.0.0")
			m.formFields[1].SetValue("npm:right@1.0.0")
			m.formIndex = 1
		}},
		{key: "4", titleKey: i18n.KeyLoadingReadingInventory},
		{key: "5", titleKey: i18n.KeyLoadingBuildingDemo},
	}
	for _, language := range []i18n.Language{i18n.English, i18n.SimplifiedChinese} {
		for _, test := range cases {
			model := sizedLanguageModel(t, language, 100, 32, false)
			send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(test.key)})
			if test.configure != nil {
				test.configure(model)
			}
			send(model, tea.KeyMsg{Type: tea.KeyEnter})
			if model.screen != screenLoading {
				t.Fatalf("%s key %s did not enter loading: %v", language, test.key, model.screen)
			}
			want := i18n.For(language).Text(test.titleKey)
			if model.loadingName != want {
				t.Fatalf("%s key %s loading title = %q, want %q", language, test.key, model.loadingName, want)
			}
			assertContains(t, model.View(), language, i18n.KeyLoadingIntro, i18n.KeyLoadingPressEsc)
			model.cancel()
		}
	}
}

func TestChineseChromeHasNoUntranslatedLabels(t *testing.T) {
	model := sizedLanguageModel(t, i18n.SimplifiedChinese, 120, 40, false)
	forbidden := []string{
		"ACTIONS", "SELECTED", "HOW IT WORKS", "PACKAGE", "PROJECT SCAN",
		"COMPARE PACKAGES", "GLOBAL INVENTORY", "OFFLINE DEMO", "SETTINGS",
		"KEYBOARD", "Start", "Settings", "Keyboard help",
	}
	views := []string{model.View()}
	for _, target := range []screen{
		screenPackageForm, screenProjectForm, screenCompareForm,
		screenInventoryConfirm, screenDemoPicker, screenSettings, screenHelp,
	} {
		send(model, tea.KeyMsg{Type: tea.KeyEsc})
		model.openScreen(target)
		views = append(views, model.View())
	}
	for _, view := range views {
		for _, word := range forbidden {
			if strings.Contains(view, word) {
				t.Fatalf("Chinese screen contains untranslated label %q:\n%s", word, view)
			}
		}
	}
}

func TestLanguageSwitchPreservesReportAndEvidence(t *testing.T) {
	report := demoReport(t, "vulnerable")
	model := sizedLanguageModel(t, i18n.English, 100, 32, false)
	model.documentSource = func() render.Document {
		return render.FromReport(report, model.language)
	}
	model.showDocument(model.documentSource(), screenMenu)
	model.scroll = 2
	model.findingIndex = 1
	model.reportView = viewChecks
	english := *model.document

	model.applyLanguage(i18n.SimplifiedChinese)
	if model.document == nil {
		t.Fatal("language switch discarded the report")
	}
	if model.reportView != viewChecks || model.findingIndex != 1 || model.scroll != 2 {
		t.Fatalf("view state changed: view=%v finding=%d scroll=%d", model.reportView, model.findingIndex, model.scroll)
	}
	model.scroll = 0
	assertContains(t, model.View(), i18n.SimplifiedChinese, i18n.KeyReportHeadingChecks)
	if model.document.Title == english.Title || !strings.Contains(model.document.Title, "包报告") {
		t.Fatalf("report title was not localized: %q", model.document.Title)
	}
	if len(model.document.Findings) != len(english.Findings) {
		t.Fatalf("findings changed after language switch: %d vs %d", len(model.document.Findings), len(english.Findings))
	}
	for index := range english.Findings {
		if model.document.Findings[index].ID != english.Findings[index].ID {
			t.Fatalf("finding %d ID changed", index)
		}
		if model.document.Findings[index].Title != english.Findings[index].Title {
			t.Fatalf("finding %d title changed", index)
		}
		if len(model.document.Findings[index].Evidence) != len(english.Findings[index].Evidence) {
			t.Fatalf("finding %d evidence changed", index)
		}
		for evidenceIndex, evidence := range english.Findings[index].Evidence {
			got := model.document.Findings[index].Evidence[evidenceIndex]
			if got.Summary != evidence.Summary || got.SourceID != evidence.SourceID || got.Reference != evidence.Reference {
				t.Fatalf("finding %d evidence %d changed", index, evidenceIndex)
			}
		}
	}
	if !reflect.DeepEqual(model.document.Sources, english.Sources) {
		t.Fatal("sources changed after language switch")
	}
	if !reflect.DeepEqual(model.document.Checks, english.Checks) {
		t.Fatal("check results changed after language switch")
	}
	if !reflect.DeepEqual(model.document.Limitations, english.Limitations) {
		t.Fatal("limitations changed after language switch")
	}
	if (model.document.Tree == nil) != (english.Tree == nil) {
		t.Fatal("dependency tree presence changed after language switch")
	}
	if english.Tree != nil {
		if model.document.Tree.RootID != english.Tree.RootID || len(model.document.Tree.Nodes) != len(english.Tree.Nodes) {
			t.Fatal("dependency tree structure changed after language switch")
		}
		for id, node := range english.Tree.Nodes {
			translated, ok := model.document.Tree.Nodes[id]
			if !ok || !reflect.DeepEqual(translated.Children, node.Children) {
				t.Fatalf("dependency node %q changed after language switch", id)
			}
		}
	}
}

func TestBilingualLayoutFitsAllSupportedSizes(t *testing.T) {
	report := demoReport(t, "vulnerable")
	left := report
	right := demoReport(t, "clean")
	request, err := app.NewPackageRequest(domain.EcosystemNPM, "demo-left", "1.0.0")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	sizes := [][2]int{{46, 14}, {60, 30}, {80, 24}, {100, 32}, {140, 40}}
	for _, language := range []i18n.Language{i18n.English, i18n.SimplifiedChinese} {
		for _, color := range []bool{false, true} {
			for _, size := range sizes {
				model := sizedLanguageModel(t, language, size[0], size[1], color)
				views := map[string]string{"menu": model.View()}

				model.openScreen(screenPackageForm)
				views["package_form"] = model.View()
				model.openScreen(screenProjectForm)
				views["project_form"] = model.View()
				model.openScreen(screenCompareForm)
				views["compare_form"] = model.View()
				model.openScreen(screenInventoryConfirm)
				views["inventory_confirm"] = model.View()
				model.openScreen(screenSettings)
				views["settings"] = model.View()
				model.openScreen(screenDemoPicker)
				views["demo"] = model.View()
				model.openScreen(screenHelp)
				views["help"] = model.View()

				model.openScreen(screenPackageForm)
				model.formFields[0].SetValue("npm:fixture@1.0.0")
				send(model, tea.KeyMsg{Type: tea.KeyEnter})
				views["loading"] = model.View()
				model.cancel()

				model.screen = screenError
				model.errorText = i18n.For(language).Text(i18n.KeyReportSourceLanguageNote)
				views["error"] = model.View()

				model.showDocument(render.FromReport(report, language), screenMenu)
				views["report_overview"] = model.View()
				model.reportView = viewFinding
				views["report_finding"] = model.View()
				model.reportView = viewChecks
				views["report_checks"] = model.View()
				model.reportView = viewDependencies
				views["report_dependencies"] = model.View()
				model.reportView = viewSources
				views["report_sources"] = model.View()
				model.reportView = viewSummary
				views["report_summary"] = model.View()

				model.comparison = &app.Comparison{
					LeftRequest:  request,
					RightRequest: request,
					Left:         &left,
					Right:        &right,
				}
				model.screen = screenComparison
				views["comparison"] = model.View()

				for name, view := range views {
					plain := stripANSI(view)
					for index, line := range strings.Split(plain, "\n") {
						if width := displayWidth(line); width > size[0] {
							t.Fatalf("%s/%s %dx%d %s line %d is %d columns, want <= %d: %q",
								language, name, size[0], size[1], name, index, width, size[0], line)
						}
					}
				}
			}
		}
	}
}

func TestBrandSpellingStaysExactInBothLanguages(t *testing.T) {
	for _, language := range []i18n.Language{i18n.English, i18n.SimplifiedChinese} {
		model := sizedLanguageModel(t, language, 100, 32, false)
		view := model.View()
		if !strings.Contains(view, brandName) {
			t.Fatalf("%s start screen lost the DEPSCOPE wordmark:\n%s", language, view)
		}
		if got := wordmarkWidth(brandName); got != 65 {
			t.Fatalf("wordmark width changed to %d", got)
		}
		if !wordmarkComplete(brandName) {
			t.Fatal("wordmark glyphs are incomplete")
		}
	}
}

func TestValidationErrorsLocalizeWithoutRewritingInput(t *testing.T) {
	model := sizedLanguageModel(t, i18n.SimplifiedChinese, 100, 32, false)
	send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	send(model, tea.KeyMsg{Type: tea.KeyEnter})
	if want := i18n.For(i18n.SimplifiedChinese).Text(i18n.KeyFormErrorPackageRequired); model.formError != want {
		t.Fatalf("empty package error = %q, want %q", model.formError, want)
	}
	typeText(t, model, "bad name")
	send(model, tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(model.formError, "npm 包") {
		t.Fatalf("validation error was not localized: %q", model.formError)
	}
	if !strings.Contains(model.formError, `"bad name"`) {
		t.Fatalf("validation error rewrote the user input: %q", model.formError)
	}
}

// TestStartingNewScanDropsStaleDocumentSource 确认为新扫描清除旧报告重建闭包，
// 避免取消或失败后按新语言重建上一份报告。
func TestStartingNewScanDropsStaleDocumentSource(t *testing.T) {
	model := sizedLanguageModel(t, i18n.SimplifiedChinese, 100, 32, false)
	model.documentSource = func() render.Document { return render.Document{} }
	model.beginScan(scanPackage, i18n.KeyLoadingCheckingPackage, func(context.Context, func(app.Progress)) scanResultMsg {
		return scanResultMsg{}
	})
	if model.documentSource != nil {
		t.Fatal("new scan retained the previous report rebuild source")
	}
}
