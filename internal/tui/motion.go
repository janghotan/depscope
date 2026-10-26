package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// 动画只用于提供反馈：品牌入场、导航过渡、加载指示与完成过渡。
// 所有动画都由显式的 tick 消息驱动，并且在同一时刻最多只有一个 tick 在途，
// 结束时自动停止，不会留下持续重绘的 goroutine。

const (
	// tickInterval 是渲染帧间隔；每秒约 10 帧足够平滑且开销很低。
	tickInterval = 100 * time.Millisecond
	// brandRevealFrames 是品牌入场动画的帧数（约 0.4 秒）。
	brandRevealFrames = 4
	// focusFrames 是导航与画面切换强调动画的帧数（约 0.2 秒）。
	focusFrames = 2
	// completeFrames 是扫描完成过渡的帧数（约 0.3 秒）。
	completeFrames = 3
	// spinnerFrames 是加载指示的循环字符。
	spinnerFrames = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"
)

// tickMsg 推进一帧动画。generation 用于丢弃已经过期的动画帧，
// 保证旧动画不会覆盖更新后的画面或结果。
type tickMsg struct {
	generation int
}

// animation 描述当前正在运行的动画状态。
type animation struct {
	enabled    bool
	generation int
	brand      int
	focus      int
	complete   int
	// spinner 只在加载页有意义，并且不推进数据，只推进图形。
	spinner int
	// ticking 防止同一时刻存在多个 tick 命令。
	ticking bool
}

// active 报告是否还有动画需要继续推进。
func (a animation) active() bool {
	return a.brand > 0 || a.focus > 0 || a.complete > 0 || a.spinner > 0
}

// startBrand 开始品牌入场动画，用于启动以及回到主菜单。
func (m *Model) startBrand() tea.Cmd {
	if !m.anim.enabled {
		return nil
	}
	m.anim.brand = brandRevealFrames
	return m.scheduleTick()
}

// startFocus 开始一次短暂的导航强调动画。
func (m *Model) startFocus() tea.Cmd {
	if !m.anim.enabled || m.anim.brand > 0 {
		// 品牌入场期间不需要额外的焦点动画，避免两套动效叠加。
		return nil
	}
	m.anim.focus = focusFrames
	return m.scheduleTick()
}

// startComplete 在真实结果到达时播放一次完成过渡。
func (m *Model) startComplete() tea.Cmd {
	if !m.anim.enabled {
		return nil
	}
	m.anim.complete = completeFrames
	return m.scheduleTick()
}

// startSpinner 在加载页启动不确定进度指示。
func (m *Model) startSpinner() tea.Cmd {
	if !m.anim.enabled {
		return nil
	}
	m.anim.spinner = 1
	return m.scheduleTick()
}

// stopAnimations 在离开需要动画的画面时立即归零，并让在途的 tick 自然失效。
func (m *Model) stopAnimations() {
	m.anim.brand = 0
	m.anim.focus = 0
	m.anim.complete = 0
	m.anim.spinner = 0
	// ticking 交给下一帧 tickMsg 清理：帧到达时 generation 已变化会被忽略。
}

// scheduleTick 安排下一帧；已有在途 tick 时不重复安排。
func (m *Model) scheduleTick() tea.Cmd {
	if !m.anim.enabled || m.anim.ticking {
		return nil
	}
	m.anim.ticking = true
	generation := m.anim.generation
	return tea.Tick(tickInterval, func(time.Time) tea.Msg {
		return tickMsg{generation: generation}
	})
}

// handleTick 推进一帧动画并决定是否继续。
func (m *Model) handleTick(msg tickMsg) tea.Cmd {
	// 过期帧（例如画面已切换并递增了 generation）直接丢弃。
	if msg.generation != m.anim.generation {
		return nil
	}
	m.anim.ticking = false
	if !m.anim.enabled || !m.anim.active() {
		// 动画已全部结束：没有继续安排的帧，保持 ticking 为假以便下次重新启动。
		m.anim.ticking = false
		return nil
	}
	if m.anim.brand > 0 {
		m.anim.brand--
	}
	if m.anim.focus > 0 {
		m.anim.focus--
	}
	if m.anim.complete > 0 {
		m.anim.complete--
	}
	if m.anim.spinner > 0 {
		m.anim.spinner++
	}
	if !m.anim.active() {
		// 最后一帧已应用，动画结束，不再安排新帧。
		return nil
	}
	return m.scheduleTick()
}

// advanceGeneration 让所有在途动画帧失效；每次切换画面时调用。
func (m *Model) advanceGeneration() {
	m.anim.generation++
	m.anim.ticking = false
}

// spinnerGlyph 返回当前加载指示字符。
func (m *Model) spinnerGlyph() string {
	if !m.anim.enabled || m.anim.spinner == 0 {
		return "·"
	}
	index := (m.anim.spinner - 1) % len([]rune(spinnerFrames))
	runes := []rune(spinnerFrames)
	return string(runes[index])
}

// brandVisible 报告品牌入场动画是否仍在进行，用于决定是否逐步显示词标。
func (m *Model) brandVisible() bool {
	return m.anim.brand > 0
}

// revealWord 根据入场进度截取词标前缀，产生“逐渐打出”的效果。
func (m *Model) revealWord(word string) string {
	if !m.anim.enabled || m.anim.brand <= 0 {
		return word
	}
	runes := []rune(word)
	total := len(runes)
	if total == 0 {
		return word
	}
	// brand 从 brandRevealFrames 递减到 1，显示比例相应增加。
	shown := total * (brandRevealFrames - m.anim.brand + 1) / (brandRevealFrames + 1)
	if shown < 1 {
		shown = 1
	}
	if shown > total {
		shown = total
	}
	return string(runes[:shown])
}

// animationsEnabled 解析是否启用动画。
// 显式关闭、TERM=dumb 以及常见的 CI 环境都会退化为静态界面。
func animationsEnabled(mode string, lookupEnv func(string) string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "off", "never", "false", "no", "0":
		return false
	case "on", "always", "true", "yes", "1":
		return true
	}
	if lookupEnv == nil {
		return true
	}
	// 变量存在即视为关闭，与 NO_COLOR 的约定一致。
	if _, present := envPresent(lookupEnv, "DEPSCOPE_NO_ANIMATION"); present {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(lookupEnv("TERM")), "dumb") {
		return false
	}
	for _, name := range []string{"CI", "GITHUB_ACTIONS", "BUILDKITE", "CONTINUOUS_INTEGRATION"} {
		if strings.TrimSpace(lookupEnv(name)) != "" {
			return false
		}
	}
	return true
}

// envPresent 区分“变量未设置”和“变量已设置但为空”，用于 DEPSCOPE_NO_ANIMATION。
func envPresent(lookupEnv func(string) string, name string) (string, bool) {
	value := lookupEnv(name)
	return value, value != ""
}
