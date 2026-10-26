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
		if got := runeLen(line); got > width {
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
			if !strings.Contains(view, entry.title) {
				t.Fatalf("%dx%d: workflow %q is missing", size[0], size[1], entry.title)
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
	if !strings.Contains(view, menuEntries[1].description[:20]) {
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
	if model.settingsAt != 2 {
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
