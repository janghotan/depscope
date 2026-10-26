// Package tui 提供 DepScope 的交互式终端界面。
// 界面只负责收集输入、展示共享引擎结果与取消真实扫描，不自行判断风险。
package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janghotan/depscope/internal/app"
	"github.com/janghotan/depscope/internal/model"
	"github.com/janghotan/depscope/internal/render"
)

const (
	defaultWidth  = 80
	defaultHeight = 24
	minWidth      = 46
	minHeight     = 14
)

type screen int

const (
	screenMenu screen = iota
	screenPackageForm
	screenProjectForm
	screenCompareForm
	screenInventoryConfirm
	screenDemoPicker
	screenSettings
	screenLoading
	screenReport
	screenComparison
	screenError
	screenHelp
)

type reportView int

const (
	viewOverview reportView = iota
	viewFinding
	viewChecks
	viewDependencies
	viewDetails
	viewSources
	viewSummary
)

type menuEntry struct {
	title       string
	description string
	target      screen
}

var menuEntries = []menuEntry{
	{
		title:       "Check a single package",
		description: "Registry metadata, advisories, lifecycle scripts, and name signals.",
		target:      screenPackageForm,
	},
	{
		title:       "Scan my project",
		description: "Read supported npm and Python manifests without executing code.",
		target:      screenProjectForm,
	},
	{
		title:       "Compare two packages",
		description: "Side-by-side versions, source coverage, and findings.",
		target:      screenCompareForm,
	},
	{
		title:       "Check my global installs",
		description: "Read-only npm and pip inventory of this machine.",
		target:      screenInventoryConfirm,
	},
	{
		title:       "Run the offline demo",
		description: "Deterministic synthetic reports; no network access.",
		target:      screenDemoPicker,
	},
	{
		title:       "Settings",
		description: "Session defaults and color behavior.",
		target:      screenSettings,
	},
}

var reportViews = []reportView{
	viewOverview,
	viewChecks,
	viewDependencies,
	viewDetails,
	viewSources,
	viewSummary,
}

// Options 配置交互界面的默认值与环境。
type Options struct {
	ColorMode        string
	DefaultEcosystem model.Ecosystem
	LookupEnv        func(string) string
	Context          context.Context
	// Animations 显式控制动画；留空时由 DEPSCOPE_NO_ANIMATION、TERM 与 CI 决定。
	Animations string
}

// Model 是 Bubble Tea 的界面状态；所有扫描通过 app.Service 完成。
type Model struct {
	service    app.Service
	width      int
	height     int
	screen     screen
	styles     styles
	lookupEnv  func(string) string
	baseCtx    context.Context
	colorMode  render.ColorMode
	color      bool
	ecosystem  model.Ecosystem
	menuIndex  int
	formFields []textField
	formIndex  int
	formError  string
	settingsAt int
	demoAt     int
	demos      []app.DemoScenario
	helpReturn screen

	progressCh  chan progressMsg
	scanID      int
	cancel      context.CancelFunc
	loading     app.Progress
	loadingName string
	progressLog []app.Progress
	cancelling  bool

	document     *render.Document
	reportView   reportView
	findingIndex int
	returnScreen screen
	comparison   *app.Comparison

	scroll    int
	errorText string
	notice    string

	// anim 保存动画进度；没有动画时所有字段为 0，界面保持静态。
	anim animation
	// motionMode 是构造时的动画设置，保留用于设置页展示。
	motionMode string
}

// New 创建界面模型；构造过程不会访问网络或执行包管理器命令。
func New(service app.Service, opts Options) *Model {
	width := defaultWidth
	height := defaultHeight
	mode, err := render.ParseColorMode(opts.ColorMode)
	if err != nil {
		mode = render.ColorAuto
	}
	lookupEnv := opts.LookupEnv
	if lookupEnv == nil {
		lookupEnv = os.Getenv
	}
	ecosystem := opts.DefaultEcosystem
	if !ecosystem.IsValid() {
		ecosystem = model.EcosystemNPM
	}
	baseCtx := opts.Context
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	m := &Model{
		service:    service,
		width:      width,
		height:     height,
		screen:     screenMenu,
		lookupEnv:  lookupEnv,
		baseCtx:    baseCtx,
		colorMode:  mode,
		ecosystem:  ecosystem,
		progressCh: make(chan progressMsg, 32),
	}
	m.color = render.ColorEnabled(mode, true, lookupEnv)
	m.styles = newStyles(m.color)
	m.motionMode = opts.Animations
	m.anim.enabled = animationsEnabled(opts.Animations, lookupEnv)
	if service != nil {
		m.demos = service.DemoScenarios()
	}
	return m
}

// Init 只启动品牌入场动画，不执行任何后台扫描工作。
func (m *Model) Init() tea.Cmd { return m.startBrand() }

// Update 处理窗口、键盘、进度与扫描结果消息。
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampScroll()
		return m, nil
	case tea.InterruptMsg:
		if m.cancel != nil {
			m.cancel()
		}
		return m, tea.Quit
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tickMsg:
		return m, m.handleTick(msg)
	case progressMsg:
		if msg.id != m.scanID || m.screen != screenLoading {
			return m, nil
		}
		m.loading = msg.progress
		m.progressLog = appendProgress(m.progressLog, msg.progress)
		return m, m.waitProgress(m.scanID)
	case scanResultMsg:
		return m.handleScanResult(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		if m.cancel != nil {
			m.cancel()
		}
		return m, tea.Quit
	}
	if m.screen == screenLoading {
		if msg.Type == tea.KeyEsc {
			if m.cancel != nil {
				m.cancel()
			}
			m.cancelling = true
			m.loading.Message = "Cancellation requested. Waiting for the current read-only request to stop."
			return m, nil
		}
		return m, nil
	}
	if msg.String() == "?" && m.helpAllowed() {
		m.helpReturn = m.screen
		m.screen = screenHelp
		m.scroll = 0
		return m, nil
	}
	switch m.screen {
	case screenMenu:
		return m.handleMenuKey(msg)
	case screenPackageForm, screenProjectForm, screenCompareForm:
		return m.handleFormKey(msg)
	case screenInventoryConfirm:
		return m.handleInventoryKey(msg)
	case screenDemoPicker:
		return m.handleDemoKey(msg)
	case screenSettings:
		return m.handleSettingsKey(msg)
	case screenReport:
		return m.handleReportKey(msg)
	case screenComparison:
		return m.handleComparisonKey(msg)
	case screenError:
		return m.handleErrorKey(msg)
	case screenHelp:
		return m.handleHelpKey(msg)
	}
	return m, nil
}

func (m *Model) helpAllowed() bool {
	switch m.screen {
	case screenPackageForm, screenProjectForm, screenCompareForm, screenHelp:
		return false
	default:
		return true
	}
}

func (m *Model) handleMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.menuIndex = wrapIndex(m.menuIndex-1, len(menuEntries))
	case "down", "j":
		m.menuIndex = wrapIndex(m.menuIndex+1, len(menuEntries))
	case "enter":
		return m, m.openScreen(menuEntries[m.menuIndex].target)
	case "1", "2", "3", "4", "5", "6":
		index := int(msg.String()[0] - '1')
		if index >= 0 && index < len(menuEntries) {
			m.menuIndex = index
			return m, m.openScreen(menuEntries[index].target)
		}
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) openScreen(target screen) tea.Cmd {
	m.notice = ""
	m.formError = ""
	m.scroll = 0
	// 切换画面时让在途动画帧失效，随后开始新的导航强调。
	m.stopAnimations()
	m.advanceGeneration()
	m.screen = target
	m.formIndex = 0
	switch target {
	case screenPackageForm:
		m.formFields = []textField{newTextField("lodash@4.17.21 · pypi:requests==2.31.0")}
	case screenProjectForm:
		field := newTextField("project directory")
		field.SetValue(".")
		m.formFields = []textField{field}
	case screenCompareForm:
		m.formFields = []textField{
			newTextField("first package"),
			newTextField("second package"),
		}
	case screenDemoPicker:
		m.demoAt = 0
	case screenSettings:
		m.settingsAt = 0
	}
	return m.startFocus()
}

// returnToMenu 回到主菜单；返回的返回值供调用方启动品牌入场动画。
func (m *Model) returnToMenu() tea.Cmd {
	m.stopAnimations()
	m.advanceGeneration()
	m.screen = screenMenu
	m.formError = ""
	m.errorText = ""
	m.scroll = 0
	m.cancelling = false
	m.cancel = nil
	return m.startBrand()
}

func (m *Model) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		return m, m.returnToMenu()
	case tea.KeyTab:
		m.formIndex = wrapIndex(m.formIndex+1, len(m.formFields))
		m.formError = ""
		return m, nil
	case tea.KeyShiftTab:
		m.formIndex = wrapIndex(m.formIndex-1, len(m.formFields))
		m.formError = ""
		return m, nil
	case tea.KeyEnter:
		return m.submitForm()
	}
	m.editField(msg)
	return m, nil
}

func (m *Model) editField(msg tea.KeyMsg) {
	if len(m.formFields) == 0 {
		return
	}
	field := &m.formFields[m.formIndex]
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace:
		field.Insert(msg.Runes)
	case tea.KeyBackspace:
		field.Backspace()
	case tea.KeyDelete:
		field.Delete()
	case tea.KeyLeft:
		field.Left()
	case tea.KeyRight:
		field.Right()
	case tea.KeyHome:
		field.Home()
	case tea.KeyEnd:
		field.End()
	case tea.KeyCtrlU:
		field.Clear()
	}
}

func (m *Model) submitForm() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenPackageForm:
		value := strings.TrimSpace(m.formFields[0].String())
		if value == "" {
			m.formError = "Enter a package name, for example lodash@4.17.21 or pypi:requests==2.31.0."
			return m, nil
		}
		request, err := app.ParsePackageSpec(value, m.ecosystem)
		if err != nil {
			m.formError = "Cannot parse package: " + render.Sanitize(err.Error())
			return m, nil
		}
		m.formError = ""
		return m, m.startScan(m.beginPackageScan(request))
	case screenProjectForm:
		value := strings.TrimSpace(m.formFields[0].String())
		if value == "" {
			m.formError = "Enter a project directory, or use . for the current directory."
			return m, nil
		}
		request := app.ProjectRequest{Path: value}
		if err := request.Validate(); err != nil {
			m.formError = render.Sanitize(err.Error())
			return m, nil
		}
		m.formError = ""
		return m, m.startScan(m.beginProjectScan(request))
	case screenCompareForm:
		if m.formIndex == 0 && len(m.formFields) > 1 && strings.TrimSpace(m.formFields[1].String()) == "" {
			if strings.TrimSpace(m.formFields[0].String()) == "" {
				m.formError = "Enter the first package."
				return m, nil
			}
			m.formIndex = 1
			m.formError = ""
			return m, nil
		}
		left, err := app.ParsePackageSpec(strings.TrimSpace(m.formFields[0].String()), m.ecosystem)
		if err != nil {
			m.formError = "Left package: " + render.Sanitize(err.Error())
			return m, nil
		}
		right, err := app.ParsePackageSpec(strings.TrimSpace(m.formFields[1].String()), m.ecosystem)
		if err != nil {
			m.formError = "Right package: " + render.Sanitize(err.Error())
			return m, nil
		}
		request := app.CompareRequest{Left: left, Right: right}
		if err := request.Validate(); err != nil {
			m.formError = render.Sanitize(err.Error())
			return m, nil
		}
		m.formError = ""
		return m, m.startScan(m.beginCompareScan(request))
	}
	return m, nil
}

func (m *Model) handleInventoryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		return m, m.returnToMenu()
	case tea.KeyEnter:
		return m, m.startScan(m.beginInventoryScan())
	}
	return m, nil
}

func (m *Model) handleDemoKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, m.returnToMenu()
	case "up", "k":
		if len(m.demos) > 0 {
			m.demoAt = wrapIndex(m.demoAt-1, len(m.demos))
		}
	case "down", "j":
		if len(m.demos) > 0 {
			m.demoAt = wrapIndex(m.demoAt+1, len(m.demos))
		}
	case "enter":
		if len(m.demos) == 0 {
			m.formError = "No offline demo scenarios are available."
			return m, nil
		}
		scenario := m.demos[m.demoAt]
		return m, m.startScan(m.beginDemoScan(scenario.ID))
	}
	return m, nil
}

func (m *Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	const rowCount = 4
	switch msg.String() {
	case "esc":
		return m, m.returnToMenu()
	case "up", "k":
		m.settingsAt = wrapIndex(m.settingsAt-1, rowCount)
	case "down", "j":
		m.settingsAt = wrapIndex(m.settingsAt+1, rowCount)
	case "enter", " ":
		return m, m.changeSetting(false)
	case "left", "h":
		return m, m.changeSetting(true)
	case "right", "l":
		return m, m.changeSetting(false)
	}
	return m, nil
}

// changeSetting 修改当前设置项；返回的命令用于在主菜单重新播放品牌动画。
func (m *Model) changeSetting(backward bool) tea.Cmd {
	switch m.settingsAt {
	case 0:
		if m.ecosystem == model.EcosystemNPM {
			m.ecosystem = model.EcosystemPyPI
		} else {
			m.ecosystem = model.EcosystemNPM
		}
	case 1:
		modes := []render.ColorMode{render.ColorAuto, render.ColorAlways, render.ColorNever}
		index := 0
		for current, mode := range modes {
			if mode == m.colorMode {
				index = current
				break
			}
		}
		if backward {
			index = (index + len(modes) - 1) % len(modes)
		} else {
			index = (index + 1) % len(modes)
		}
		m.colorMode = modes[index]
		m.color = render.ColorEnabled(m.colorMode, true, m.lookupEnv)
		m.styles = newStyles(m.color)
	case 2:
		// 动画开关切换后立即生效：关闭时清除所有进行中的动画。
		m.anim.enabled = !m.anim.enabled
		if !m.anim.enabled {
			m.stopAnimations()
			m.advanceGeneration()
		} else {
			return m.startFocus()
		}
	case 3:
		return m.returnToMenu()
	}
	return nil
}

func (m *Model) handleReportKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		if m.reportView != viewOverview {
			m.reportView = viewOverview
			m.scroll = 0
			return m, nil
		}
		return m, m.returnToMenu()
	case "up", "k":
		if m.reportView == viewOverview && m.document != nil && len(m.document.Findings) > 0 {
			m.findingIndex = wrapIndex(m.findingIndex-1, len(m.document.Findings))
			m.ensureFindingVisible()
			return m, nil
		}
		if m.reportView == viewFinding && m.document != nil && len(m.document.Findings) > 0 {
			m.findingIndex = wrapIndex(m.findingIndex-1, len(m.document.Findings))
			m.scroll = 0
			return m, nil
		}
		m.scrollBy(-1)
	case "down", "j":
		if m.reportView == viewOverview && m.document != nil && len(m.document.Findings) > 0 {
			m.findingIndex = wrapIndex(m.findingIndex+1, len(m.document.Findings))
			m.ensureFindingVisible()
			return m, nil
		}
		if m.reportView == viewFinding && m.document != nil && len(m.document.Findings) > 0 {
			m.findingIndex = wrapIndex(m.findingIndex+1, len(m.document.Findings))
			m.scroll = 0
			return m, nil
		}
		m.scrollBy(1)
	case "pgup":
		m.scrollBy(-m.pageSize())
	case "pgdown":
		m.scrollBy(m.pageSize())
	case "home":
		m.scroll = 0
	case "end":
		m.scroll = m.maxScroll()
	case "enter":
		if m.reportView == viewOverview && m.document != nil && len(m.document.Findings) > 0 {
			m.reportView = viewFinding
			m.scroll = 0
		}
	case "tab":
		m.cycleReportView(1)
	case "shift+tab":
		m.cycleReportView(-1)
	case "o":
		m.reportView = viewOverview
		m.scroll = 0
	case "c":
		m.setReportView(viewChecks)
	case "d":
		m.setReportView(viewDependencies)
	case "i":
		m.setReportView(viewDetails)
	case "s":
		m.setReportView(viewSummary)
	case "v":
		m.setReportView(viewSources)
	}
	return m, nil
}

func (m *Model) cycleReportView(direction int) {
	current := 0
	for index, view := range reportViews {
		if view == m.reportView {
			current = index
			break
		}
	}
	current = wrapIndex(current+direction, len(reportViews))
	m.setReportView(reportViews[current])
}

func (m *Model) setReportView(view reportView) {
	m.reportView = view
	m.scroll = 0
}

func (m *Model) handleComparisonKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		return m, m.returnToMenu()
	case "up", "k":
		m.scrollBy(-1)
	case "down", "j":
		m.scrollBy(1)
	case "pgup":
		m.scrollBy(-m.pageSize())
	case "pgdown":
		m.scrollBy(m.pageSize())
	case "home":
		m.scroll = 0
	case "end":
		m.scroll = m.maxScroll()
	}
	return m, nil
}

func (m *Model) handleErrorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		return m, m.returnToMenu()
	}
	return m, nil
}

func (m *Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc", "?":
		m.screen = m.helpReturn
		m.scroll = 0
	}
	return m, nil
}

func (m *Model) handleScanResult(msg scanResultMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.scanID || m.screen != screenLoading {
		return m, nil
	}
	m.cancel = nil
	m.cancelling = false
	if msg.err != nil {
		if errors.Is(msg.err, context.Canceled) {
			cmd := m.returnToMenu()
			m.notice = "Scan cancelled. No results were kept."
			return m, cmd
		}
		m.screen = screenError
		m.errorText = render.SanitizeMultiline(msg.err.Error())
		return m, nil
	}
	switch msg.kind {
	case scanPackage, scanDemo:
		if msg.report.Status == model.StatusCancelled {
			cmd := m.returnToMenu()
			m.notice = "Scan cancelled. No results were kept."
			return m, cmd
		}
		document := render.FromReport(msg.report)
		return m, m.showDocument(document, screenMenu)
	case scanProject:
		if msg.project.Status == model.StatusCancelled {
			cmd := m.returnToMenu()
			m.notice = "Scan cancelled. No results were kept."
			return m, cmd
		}
		document := render.FromProject(msg.project)
		return m, m.showDocument(document, screenMenu)
	case scanInventory:
		if msg.inventory.Status == model.StatusCancelled {
			cmd := m.returnToMenu()
			m.notice = "Scan cancelled. No results were kept."
			return m, cmd
		}
		document := render.FromInventory(msg.inventory)
		return m, m.showDocument(document, screenMenu)
	case scanCompare:
		comparison := msg.comparison
		m.comparison = &comparison
		m.stopAnimations()
		m.advanceGeneration()
		m.screen = screenComparison
		m.scroll = 0
		m.notice = ""
		return m, m.startComplete()
	}
	return m, nil
}

// showDocument 展示报告；结果立即可见，完成过渡只是短暂的高亮提示。
func (m *Model) showDocument(document render.Document, returnScreen screen) tea.Cmd {
	m.document = &document
	m.stopAnimations()
	m.advanceGeneration()
	m.screen = screenReport
	m.reportView = viewOverview
	m.findingIndex = 0
	m.scroll = 0
	m.returnScreen = returnScreen
	m.notice = ""
	return m.startComplete()
}

func (m *Model) scrollBy(delta int) {
	m.scroll += delta
	m.clampScroll()
}

func (m *Model) clampScroll() {
	max := m.maxScroll()
	if m.scroll > max {
		m.scroll = max
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m *Model) maxScroll() int {
	lines := strings.Count(m.bodyText(), "\n") + 1
	max := lines - m.contentHeight()
	if max < 0 {
		return 0
	}
	return max
}

func (m *Model) pageSize() int {
	size := m.contentHeight() - 2
	if size < 1 {
		return 1
	}
	return size
}

func (m *Model) ensureFindingVisible() {
	if m.document == nil || m.findingIndex < 0 || m.findingIndex >= len(m.document.Findings) {
		return
	}
	content := m.document.Overview(m.theme(), m.findingIndex)
	lines := strings.Split(content, "\n")
	needle := m.document.Findings[m.findingIndex].ID
	lineNumber := -1
	for index, line := range lines {
		if strings.Contains(line, needle) {
			lineNumber = index
			break
		}
	}
	if lineNumber < 0 {
		return
	}
	if lineNumber < m.scroll {
		m.scroll = lineNumber
	}
	height := m.contentHeight()
	if lineNumber >= m.scroll+height {
		m.scroll = lineNumber - height + 1
	}
	m.clampScroll()
}

func (m *Model) startScan(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return tea.Batch(cmd, m.waitProgress(m.scanID))
}

func (m *Model) theme() render.Theme {
	return render.Theme{Color: m.color, Width: m.contentWidth()}
}

func (m *Model) contentWidth() int {
	width := m.width - marginWidth*2
	if width < 20 {
		width = 20
	}
	return width
}

func (m *Model) contentHeight() int {
	height := m.height - 3
	if height < 1 {
		height = 1
	}
	return height
}

func appendProgress(log []app.Progress, item app.Progress) []app.Progress {
	log = append(log, item)
	const limit = 6
	if len(log) > limit {
		log = log[len(log)-limit:]
	}
	return log
}

func wrapIndex(value, count int) int {
	if count <= 0 {
		return 0
	}
	value %= count
	if value < 0 {
		value += count
	}
	return value
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// animationDescription 描述动画当前状态与决定方式。
func (m *Model) animationDescription() string {
	state := "off"
	if m.anim.enabled {
		state = "on"
	}
	description := state
	if strings.TrimSpace(m.motionMode) != "" {
		description += " (" + strings.TrimSpace(m.motionMode) + ")"
	}
	if strings.EqualFold(strings.TrimSpace(m.lookupEnv("TERM")), "dumb") {
		description += " · TERM=dumb"
	}
	if strings.TrimSpace(m.lookupEnv("CI")) != "" {
		description += " · CI"
	}
	return description
}

func (m *Model) colorDescription() string {
	state := "off"
	if m.color {
		state = "on"
	}
	description := fmt.Sprintf("%s (%s)", m.colorMode, state)
	if m.lookupEnv("NO_COLOR") != "" {
		description += " · NO_COLOR"
	}
	if strings.EqualFold(m.lookupEnv("TERM"), "dumb") {
		description += " · TERM=dumb"
	}
	return description
}
