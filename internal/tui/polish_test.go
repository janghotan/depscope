package tui

import (
	"context"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/janghotan/depscope/internal/app"
	"github.com/janghotan/depscope/internal/demo"
	domain "github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/render"
)

var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")

// noAnimationOptions 返回关闭动画的测试选项，避免 tick 影响断言。
func noAnimationOptions() Options {
	opts := testOptions()
	opts.Animations = "off"
	return opts
}

func newSizedModel(t *testing.T, service app.Service, width, height int) *Model {
	t.Helper()
	model := New(service, noAnimationOptions())
	model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return model
}

// assertLinesFit 断言每一行都不超过给定列数，防止布局折行。
func assertLinesFit(t *testing.T, view string, width int) {
	t.Helper()
	for index, line := range strings.Split(view, "\n") {
		if got := displayWidth(line); got > width {
			t.Fatalf("line %d is %d columns wide, want <= %d: %q", index, got, width, line)
		}
	}
}

// assertNoANSI 断言输出不含任何 ANSI 转义序列。
func assertNoANSI(t *testing.T, view string) {
	t.Helper()
	if match := ansiPattern.FindString(view); match != "" {
		t.Fatalf("view contains ANSI styling %q despite color being disabled", match)
	}
}

func TestLogoRendererSizesAndFallback(t *testing.T) {
	full := newStyles(true)
	plain := newStyles(false)

	// 尺寸选择：宽度不足时逐级降级，绝不会选择超过可用宽度的档位。
	// 期望值来自实际渲染验证：大标识需要宽度与高度同时满足，
	// 单行标识需要为页眉其他信息留出余量，极小宽度退化为纯文字。
	cases := []struct {
		width, height int
		want          logoSize
	}{
		{140, 40, logoFull},
		{80, 24, logoFull},
		{100, 12, logoFull},
		{100, 8, logoInline},
		{60, 30, logoInline},
		{46, 14, logoInline},
		{20, 20, logoInline},
		{15, 20, logoCompact},
		{9, 10, logoNone},
		{6, 10, logoNone},
	}
	for _, test := range cases {
		if got := selectLogoSize(test.width, test.height); got != test.want {
			t.Fatalf("selectLogoSize(%d,%d) = %v, want %v", test.width, test.height, got, test.want)
		}
	}

	// 每种档位的渲染结果都不得超出其声明宽度。
	for _, size := range []logoSize{logoFull, logoInline, logoCompact, logoNone} {
		rendered := renderLogo(plain, size)
		assertNoANSI(t, rendered)
		if size != logoNone {
			assertLinesFit(t, rendered, logoWidth(size))
		}
		// 每种档位都必须保留可辨识的品牌元素：盾牌标记、词标或块状字形。
		if size != logoNone {
			hasShield := strings.Contains(rendered, "⬢")
			hasWord := strings.Contains(rendered, "DS")
			hasBlocks := strings.Contains(rendered, "█")
			if !hasShield && !hasWord && !hasBlocks {
				t.Fatalf("size %v lost its brand glyph: %q", size, rendered)
			}
		}
	}

	// 大尺寸标识包含盾牌轮廓与词标；颜色版本带 ANSI，无色版本不带。
	if !strings.Contains(renderLogo(plain, logoFull), "DepScope") &&
		!strings.Contains(renderLogo(plain, logoFull), "██") {
		t.Fatal("full logo is missing its wordmark")
	}
	if ansiPattern.MatchString(renderLogo(plain, logoFull)) {
		t.Fatal("monochrome logo contains ANSI styling")
	}
	// 纯文字档位本来就只是文本，其余图形档位在启用颜色时必须带样式。
	// lipgloss 在非终端环境会自动去除颜色，因此这里显式强制色彩配置，
	// 否则测试无法区分“样式未应用”和“输出被重定向”。
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(previousProfile)
	for _, size := range []logoSize{logoFull, logoInline, logoCompact} {
		if !ansiPattern.MatchString(renderLogo(full, size)) {
			t.Fatalf("colored logo (size %v) produced no styling", size)
		}
	}
}

// goldenWordmark 是完整词标的独立基准位图，字面量拼写为 DEPSCOPE。
// 它不通过 letterGlyphs 推导，因此漏字、错序或替换字形都会导致测试失败。
var goldenWordmark = []string{
	"██████╗ ███████╗██████╗ ███████╗ ██████╗ ██████╗ ██████╗ ███████╗",
	"██╔══██╗██╔════╝██╔══██╗██╔════╝██╔════╝██╔═══██╗██╔══██╗██╔════╝",
	"██║  ██║█████╗  ██████╔╝███████╗██║     ██║   ██║██████╔╝█████╗  ",
	"██║  ██║██╔══╝  ██╔═══╝ ╚════██║██║     ██║   ██║██╔═══╝ ██╔══╝  ",
	"██████╔╝███████╗██║     ███████║╚██████╗╚██████╔╝██║     ███████╗",
	"╚═════╝ ╚══════╝╚═╝     ╚══════╝ ╚═════╝ ╚═════╝ ╚═╝     ╚══════╝",
}

func TestWordmarkSpellsBrandInEveryVariant(t *testing.T) {
	const want = "DEPSCOPE"
	if brandName != want {
		t.Fatalf("brandName = %q, want %q", brandName, want)
	}
	if compactName != "DS" {
		t.Fatalf("compactName = %q, want %q", compactName, "DS")
	}
	if !wordmarkComplete(brandName) {
		t.Fatalf("%q has missing or malformed glyphs", brandName)
	}

	// 逐行比对独立基准位图，验证字母顺序与字形本身。
	rows := renderWordmark(brandName)
	if len(rows) != len(goldenWordmark) {
		t.Fatalf("wordmark has %d rows, want %d", len(rows), len(goldenWordmark))
	}
	if got := wordmarkWidth(brandName); got != 65 {
		t.Fatalf("wordmark width = %d, want 65", got)
	}
	for row := range goldenWordmark {
		if rows[row] != goldenWordmark[row] {
			t.Fatalf("wordmark row %d = %q, want %q", row, rows[row], goldenWordmark[row])
		}
		if got := displayWidth(rows[row]); got != 65 {
			t.Fatalf("wordmark row %d width = %d, want 65", row, got)
		}
	}
	if got := shieldWidth(); got != 11 {
		t.Fatalf("shield width = %d, want 11", got)
	}
	if got := logoWidth(logoFull); got != 76 {
		t.Fatalf("full logo width = %d, want 76 (11 shield + 65 wordmark)", got)
	}

	// 大标识的每一行都必须以完整词标结尾，而不是缩写或残缺字母。
	plain := newStyles(false)
	fullLines := strings.Split(renderLogo(plain, logoFull), "\n")
	if len(fullLines) < len(goldenWordmark) {
		t.Fatalf("full logo has %d lines, want at least %d", len(fullLines), len(goldenWordmark))
	}
	for row := range goldenWordmark {
		if !strings.HasSuffix(fullLines[row], goldenWordmark[row]) {
			t.Fatalf("full logo row %d does not carry the wordmark:\n got %q\nwant suffix %q", row, fullLines[row], goldenWordmark[row])
		}
	}

	// 其余档位逐一核对实际渲染文本；缩写档位只能使用约定的 DS。
	for _, test := range []struct {
		name string
		size logoSize
		want string
	}{
		{name: "inline", size: logoInline, want: shieldMark + " " + want},
		{name: "compact", size: logoCompact, want: shieldMark + " " + compactName},
		{name: "text", size: logoNone, want: want},
	} {
		if got := renderLogo(plain, test.size); got != test.want {
			t.Fatalf("%s logo = %q, want %q", test.name, got, test.want)
		}
	}
	if got := logoAccessibleText(logoCompact); got != want {
		t.Fatalf("accessible text = %q, want %q", got, want)
	}

	// 入场动画的每一帧只能是完整词标的前缀，最后一帧必须完整显示。
	model := newSizedModel(t, &fakeService{}, 100, 32)
	model.anim.enabled = true
	for frame := brandRevealFrames; frame >= 0; frame-- {
		model.anim.brand = frame
		got := strings.TrimPrefix(model.brandedLogo(logoInline), shieldMark+" ")
		if got == "" || !strings.HasPrefix(want, got) {
			t.Fatalf("reveal frame %d = %q, want a non-empty prefix of %q", frame, got, want)
		}
		if frame == 0 && got != want {
			t.Fatalf("final frame = %q, want %q", got, want)
		}
	}

	// 各档位不得超过声明宽度；代表性窗口尺寸下页面也不得折行。
	for _, size := range []logoSize{logoFull, logoInline, logoCompact, logoNone} {
		rendered := renderLogo(plain, size)
		if size != logoNone {
			assertLinesFit(t, rendered, logoWidth(size))
		}
	}
	for _, dimensions := range [][2]int{{46, 14}, {80, 24}, {100, 32}, {140, 40}} {
		view := newSizedModel(t, &fakeService{}, dimensions[0], dimensions[1]).View()
		assertLinesFit(t, view, dimensions[0])
		if !strings.Contains(view, brandName) && !strings.Contains(view, compactName) {
			t.Fatalf("%dx%d: brand is missing from the view", dimensions[0], dimensions[1])
		}
		if dimensions[0] >= 80 && !strings.Contains(view, goldenWordmark[0]) {
			t.Fatalf("%dx%d: full wordmark is missing from the start screen", dimensions[0], dimensions[1])
		}
	}
}

// stripANSI 返回去除 ANSI 序列后的可见文本，用于核对彩色布局。
func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}

func TestColorLayoutKeepsHeaderAndNavigationIntact(t *testing.T) {
	// lipgloss 在非终端环境会自动去除颜色，这里显式强制色彩配置。
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(previousProfile)

	options := testOptions()
	options.ColorMode = "always"
	options.Animations = "off"

	for _, dimensions := range [][2]int{{46, 14}, {80, 24}, {100, 32}, {140, 40}} {
		model := New(&fakeService{}, options)
		model.Update(tea.WindowSizeMsg{Width: dimensions[0], Height: dimensions[1]})
		view := model.View()
		if !ansiPattern.MatchString(view) {
			t.Fatalf("%dx%d: expected ANSI styling in color mode", dimensions[0], dimensions[1])
		}

		plain := stripANSI(view)
		assertLinesFit(t, plain, dimensions[0])
		lines := strings.Split(plain, "\n")
		if !strings.HasSuffix(lines[0], "Start") {
			t.Fatalf("%dx%d: header title is not right-aligned: %q", dimensions[0], dimensions[1], lines[0])
		}
		for _, entry := range menuEntries {
			title := model.text(entry.titleKey)
			if !strings.Contains(plain, title) {
				t.Fatalf("%dx%d: navigation entry %q is clipped in color mode", dimensions[0], dimensions[1], title)
			}
		}
		selectedPrefix := "› 1  "
		if icon := model.icons.glyph(menuEntries[0].icon); icon != "" {
			selectedPrefix += icon + " "
		}
		if !strings.Contains(plain, selectedPrefix+model.text(menuEntries[0].titleKey)) {
			t.Fatalf("%dx%d: selected navigation row is clipped in color mode:\n%s", dimensions[0], dimensions[1], plain)
		}
	}
}

func TestStartScreenShowsBrandAndAllWorkflows(t *testing.T) {
	for _, size := range [][2]int{{46, 14}, {80, 24}, {100, 32}, {140, 40}} {
		model := newSizedModel(t, &fakeService{}, size[0], size[1])
		view := model.View()
		if !strings.Contains(view, "DEPSCOPE") && !strings.Contains(view, "DS") {
			t.Fatalf("%dx%d: start screen is missing the brand: \n%s", size[0], size[1], view)
		}
		if !strings.Contains(view, "Dependency risk, explained.") {
			t.Fatalf("%dx%d: start screen is missing the purpose line", size[0], size[1])
		}
		for _, entry := range menuEntries {
			title := model.text(entry.titleKey)
			if !strings.Contains(view, title) {
				t.Fatalf("%dx%d: workflow %q is missing", size[0], size[1], title)
			}
		}
		assertLinesFit(t, view, size[0])
	}
}

func TestStartScreenSelectionState(t *testing.T) {
	model := newSizedModel(t, &fakeService{}, 100, 32)
	if !strings.Contains(model.View(), "› 1") {
		t.Fatal("selected item is not marked with an arrow")
	}
	send(model, tea.KeyMsg{Type: tea.KeyDown})
	view := model.View()
	if !strings.Contains(view, "› 2") || strings.Contains(view, "› 1") {
		t.Fatalf("selection did not move:\n%s", view)
	}
	// 双列布局下右侧描述必须跟随选择。
	description := []rune(model.text(menuEntries[1].descriptionKey))
	if len(description) > 20 {
		description = description[:20]
	}
	if !strings.Contains(view, string(description)) {
		t.Fatalf("context panel did not follow the selection:\n%s", view)
	}
}

func TestMenuSelectionUnmistakableWithoutColor(t *testing.T) {
	model := newSizedModel(t, &fakeService{}, 100, 32)
	send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	// 数字键会直接进入画面；返回后确认箭头标记存在且无颜色。
	send(model, tea.KeyMsg{Type: tea.KeyEsc})
	view := model.View()
	assertNoANSI(t, view)
	if strings.Count(view, "›") != 1 {
		t.Fatalf("expected exactly one selection marker without color:\n%s", view)
	}
}

func TestAnimationsDisabledByEnvironment(t *testing.T) {
	env := func(pairs map[string]string) func(string) string {
		return func(key string) string { return pairs[key] }
	}
	cases := []struct {
		name   string
		mode   string
		env    map[string]string
		expect bool
	}{
		{name: "default", expect: true},
		{name: "explicit off", mode: "off", expect: false},
		{name: "explicit on overrides env", mode: "on", env: map[string]string{"DEPSCOPE_NO_ANIMATION": "1"}, expect: true},
		{name: "env disables", env: map[string]string{"DEPSCOPE_NO_ANIMATION": "1"}, expect: false},
		{name: "term dumb", env: map[string]string{"TERM": "dumb"}, expect: false},
		{name: "ci", env: map[string]string{"CI": "true"}, expect: false},
		{name: "github actions", env: map[string]string{"GITHUB_ACTIONS": "true"}, expect: false},
	}
	for _, test := range cases {
		if got := animationsEnabled(test.mode, env(test.env)); got != test.expect {
			t.Fatalf("%s: animationsEnabled = %v, want %v", test.name, got, test.expect)
		}
	}
}

func TestAnimationTicksStopAndIgnoreStaleFrames(t *testing.T) {
	model := newSizedModel(t, &fakeService{}, 100, 32)
	model.anim.enabled = true
	model.anim.brand = brandRevealFrames

	// 逐步推进直到品牌动画结束。
	for frames := 0; model.anim.brand > 0 && frames < 100; frames++ {
		if cmd := model.handleTick(tickMsg{generation: model.anim.generation}); cmd == nil && model.anim.brand > 1 {
			t.Fatal("animation stopped before it finished")
		}
	}
	if model.anim.brand != 0 {
		t.Fatalf("brand animation did not settle: %d", model.anim.brand)
	}
	if model.anim.ticking {
		t.Fatal("tick flag left set after the animation settled")
	}
	if model.anim.active() {
		t.Fatal("animation still reported active after settling")
	}

	// 过期帧必须被忽略，不能推进任何动画。
	model.anim.generation++
	if cmd := model.handleTick(tickMsg{generation: model.anim.generation - 1}); cmd != nil {
		t.Fatal("stale tick scheduled another frame")
	}
}

func TestAnimationsStopOnScreenTransition(t *testing.T) {
	model := newSizedModel(t, &fakeService{}, 100, 32)
	model.anim.enabled = true
	model.anim.brand = brandRevealFrames
	before := model.anim.generation

	send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	// 离开主菜单后不应再有品牌入场动画在跑。
	if model.anim.brand != 0 {
		t.Fatalf("brand animation survived the transition: %d", model.anim.brand)
	}
	if model.anim.generation == before {
		t.Fatal("generation did not advance, so stale frames would still apply")
	}
	// 过期帧现在必须无效。
	if cmd := model.handleTick(tickMsg{generation: before}); cmd != nil {
		t.Fatal("frame from the previous screen was not discarded")
	}
}

func TestLoadingShowsSpinnerOnlyWhenAnimationsEnabled(t *testing.T) {
	model := newSizedModel(t, &fakeService{}, 100, 32)
	model.anim.enabled = true
	// 进入加载页，指示字符只在加载状态显示。
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model.anim.spinner = 1
	animated := model.View()
	if !strings.Contains(animated, "⠋") {
		t.Fatalf("animated loading state is missing the spinner:\n%s", animated)
	}
	model.anim.spinner = 5
	if model.View() == animated {
		t.Fatal("spinner glyph does not advance between frames")
	}

	static := newSizedModel(t, &fakeService{}, 100, 32)
	static.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	static.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view := static.View()
	if strings.ContainsAny(view, "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏") {
		t.Fatalf("static loading state must not animate:\n%s", view)
	}
	if !strings.Contains(view, "does not report a countable total") {
		t.Fatalf("unknown-progress state is not explained:\n%s", view)
	}
}

func TestProgressBarOnlyWithRealTotals(t *testing.T) {
	model := newSizedModel(t, &fakeService{}, 100, 32)
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// 没有总数时不得显示进度条。
	if strings.Contains(model.View(), "█") {
		t.Fatal("progress bar rendered without engine-reported totals")
	}
	// 引擎给出真实完成数后才显示。
	model.loading = app.Progress{Stage: "scan", Message: "Checking batch.", Completed: 2, Total: 5}
	view := model.View()
	if !strings.Contains(view, "2/5") {
		t.Fatalf("real progress is not shown:\n%s", view)
	}
}

func TestCancellationStatesAreDistinguishable(t *testing.T) {
	service := &fakeService{blockPackage: true, started: make(chan struct{}, 1)}
	model := New(service, noAnimationOptions())
	model.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	request, err := app.NewPackageRequest(domain.EcosystemNPM, "fixture-package", "1.0.0")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	model.beginPackageScan(request)
	model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !model.cancelling {
		t.Fatal("esc did not mark the scan as cancelling")
	}
	if !strings.Contains(model.View(), "Cancellation requested") {
		t.Fatalf("cancellation request is not visible:\n%s", model.View())
	}

	// 取消完成后回到主菜单并给出明确提示。
	message := scanResultMsg{id: model.scanID, kind: scanPackage, err: context.Canceled}
	send(model, message)
	if model.screen != screenMenu {
		t.Fatalf("screen = %v, want menu", model.screen)
	}
	if !strings.Contains(model.View(), "Scan cancelled") {
		t.Fatalf("completed cancellation is not reported:\n%s", model.View())
	}
}

func TestCompletionTransitionKeepsResultImmediatelyVisible(t *testing.T) {
	model := newSizedModel(t, &fakeService{}, 100, 32)
	model.anim.enabled = true
	report, err := demo.Scan(context.Background(), "vulnerable")
	if err != nil {
		t.Fatalf("demo scan: %v", err)
	}
	model.showDocument(render.FromReport(report), screenMenu)
	// 结果必须在第一帧就可见，动画不应延迟内容。
	view := model.View()
	if !strings.Contains(view, "PACKAGE REPORT") {
		t.Fatalf("report is not visible during the completion transition:\n%s", view)
	}
	if model.anim.complete == 0 {
		t.Fatal("completion transition did not start")
	}
	// 过渡结束后完全静态。
	for frames := 0; model.anim.complete > 0 && frames < 20; frames++ {
		model.handleTick(tickMsg{generation: model.anim.generation})
	}
	if model.anim.complete != 0 {
		t.Fatalf("completion transition did not settle: %d", model.anim.complete)
	}
}

func TestMonochromeRenderingIsFullyUnderstandable(t *testing.T) {
	for _, size := range [][2]int{{46, 14}, {80, 24}, {100, 32}} {
		model := newSizedModel(t, &fakeService{}, size[0], size[1])
		view := model.View()
		assertNoANSI(t, view)
		// 关键信息通过文字表达，不依赖颜色。
		for _, want := range []string{"DEPSCOPE", "ACTIONS"} {
			if !strings.Contains(view, want) {
				t.Fatalf("%dx%d: monochrome view missing %q", size[0], size[1], want)
			}
		}
	}
}

func TestResponsiveLayoutFitsAtSupportedSizes(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	for _, size := range [][2]int{{46, 14}, {80, 24}, {100, 32}, {140, 40}} {
		model := newSizedModel(t, &fakeService{}, size[0], size[1])
		assertLinesFit(t, model.View(), size[0])

		// 各工作流画面同样不得溢出。
		for _, key := range []string{"1", "2", "3", "4", "5", "6"} {
			send(model, tea.KeyMsg{Type: tea.KeyEsc})
			send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
			assertLinesFit(t, model.View(), size[0])
		}
	}
}

func TestTwoColumnLayoutCollapsesBelowThreshold(t *testing.T) {
	narrow := newSizedModel(t, &fakeService{}, 46, 14)
	narrowView := narrow.View()
	if strings.Contains(narrowView, "HOW IT WORKS") {
		t.Fatalf("narrow layout should not show the second column:\n%s", narrowView)
	}

	wide := newSizedModel(t, &fakeService{}, 100, 32)
	wideView := wide.View()
	for _, want := range []string{"ACTIONS", "SELECTED", "HOW IT WORKS"} {
		if !strings.Contains(wideView, want) {
			t.Fatalf("two-column layout missing %q:\n%s", want, wideView)
		}
	}
}

func TestHelpDocumentsCurrentScreen(t *testing.T) {
	// 表单页上的 ? 属于输入内容，因此帮助从主菜单打开并描述主菜单操作。
	model := newSizedModel(t, &fakeService{}, 100, 32)
	send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	view := model.View()
	if !strings.Contains(view, "Current · Start") {
		t.Fatalf("help does not identify the current screen:\n%s", view)
	}
	if !strings.Contains(view, "enter selects the highlighted action") {
		t.Fatalf("help does not explain the menu actions:\n%s", view)
	}
	// 帮助页必须可返回来源画面。
	send(model, tea.KeyMsg{Type: tea.KeyEsc})
	if model.screen != screenMenu {
		t.Fatalf("screen after closing help = %v, want menu", model.screen)
	}

	// 报告页的帮助应说明查找项导航。
	report, err := demo.Scan(context.Background(), "vulnerable")
	if err != nil {
		t.Fatalf("demo scan: %v", err)
	}
	model.showDocument(render.FromReport(report), screenMenu)
	send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !strings.Contains(model.View(), "Current · Report") {
		t.Fatalf("help from the report does not name the report screen:\n%s", model.View())
	}
}

func TestSettingsTogglesAnimation(t *testing.T) {
	model := newSizedModel(t, &fakeService{}, 100, 32)
	model.anim.enabled = true
	send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("6")})
	send(model, tea.KeyMsg{Type: tea.KeyDown})
	send(model, tea.KeyMsg{Type: tea.KeyDown})
	send(model, tea.KeyMsg{Type: tea.KeyDown})
	if model.settingsAt != 3 {
		t.Fatalf("settings row = %d, want the animation row", model.settingsAt)
	}
	send(model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.anim.enabled {
		t.Fatal("animation setting did not turn off")
	}
	if !strings.Contains(model.View(), "Animation") {
		t.Fatalf("settings screen lost the animation row:\n%s", model.View())
	}
}
