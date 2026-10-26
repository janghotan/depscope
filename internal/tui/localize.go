package tui

import (
	"fmt"
	"strings"

	"github.com/janghotan/depscope/internal/app"
	"github.com/janghotan/depscope/internal/i18n"
	"github.com/janghotan/depscope/internal/render"
)

// localizedValidationError 把共享应用层的固定校验错误映射为当前语言。
// 未识别的错误原样保留，避免丢失诊断信息。
func (m *Model) localizedValidationError(err error) string {
	return render.Sanitize(translateValidationError(m.language, err))
}

// translateValidationError 只翻译应用层自己生成的校验文本；
// 包名、版本号等参数仍按原样插入，不翻译用户或注册表内容。
func translateValidationError(language i18n.Language, err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	for _, wrapper := range []struct {
		prefix string
		key    i18n.Key
	}{
		{prefix: "left: ", key: i18n.KeyFormErrorLeftPrefix},
		{prefix: "right: ", key: i18n.KeyFormErrorRightPrefix},
	} {
		if strings.HasPrefix(message, wrapper.prefix) {
			inner := strings.TrimPrefix(message, wrapper.prefix)
			return i18n.For(language).Text(wrapper.key) + translateSimpleValidation(language, inner)
		}
	}
	return translateSimpleValidation(language, message)
}

func translateSimpleValidation(language i18n.Language, message string) string {
	catalog := i18n.For(language)
	for _, literal := range []struct {
		text string
		key  i18n.Key
	}{
		{text: "package must not be empty", key: i18n.KeyValidationPackageEmpty},
		{text: "package must include a name", key: i18n.KeyValidationPackageMissingName},
		{text: "package name must not be empty", key: i18n.KeyValidationPackageNameEmpty},
		{text: "version must not be empty after a separator", key: i18n.KeyValidationVersionEmpty},
		{text: "package name must not contain leading or trailing whitespace", key: i18n.KeyValidationWhitespace},
		{text: "version must not contain leading or trailing whitespace", key: i18n.KeyValidationVersionOuterWhitespace},
		{text: "project path must not be empty", key: i18n.KeyValidationProjectPathEmpty},
	} {
		if message == literal.text {
			return catalog.Text(literal.key)
		}
	}

	var first, second string
	for _, pattern := range []struct {
		format string
		key    i18n.Key
		args   []any
	}{
		{format: "package name %q is not canonical; use %q", key: i18n.KeyValidationNotCanonical, args: []any{&first, &second}},
		{format: "scoped npm package %q must contain exactly one slash", key: i18n.KeyValidationScopedSlash, args: []any{&first}},
		{format: "scoped npm package %q must include a scope and a name", key: i18n.KeyValidationScopedParts, args: []any{&first}},
		{format: "unscoped npm package %q must not contain a slash", key: i18n.KeyValidationNpmUnscopedSlash, args: []any{&first}},
		{format: "npm package %q contains invalid characters", key: i18n.KeyValidationNpmInvalid, args: []any{&first}},
		{format: "pypi package %q contains invalid characters", key: i18n.KeyValidationPyPIInvalid, args: []any{&first}},
		{format: "version %q must not contain whitespace", key: i18n.KeyValidationVersionWhitespace, args: []any{&first}},
		{format: "version %q must not contain control characters", key: i18n.KeyValidationVersionControl, args: []any{&first}},
		{format: "unsupported ecosystem %q", key: i18n.KeyValidationUnsupportedEcosystem, args: []any{&first}},
		{format: "ecosystem %q is not supported", key: i18n.KeyValidationUnsupportedEcosystem, args: []any{&first}},
	} {
		if scanned, _ := fmt.Sscanf(message, pattern.format, pattern.args...); scanned != len(pattern.args) {
			continue
		}
		values := make([]any, len(pattern.args))
		for index := range pattern.args {
			values[index] = *pattern.args[index].(*string)
		}
		return catalog.Format(pattern.key, values...)
	}
	return message
}

// demoTitle 与 demoDescription 优先使用按场景 ID 映射的本地化文案；
// 未知场景回退到服务提供的原始文本。
func (m *Model) demoTitle(scenario app.DemoScenario) string {
	if key, ok := demoTitleKeys[scenario.ID]; ok {
		return m.text(key)
	}
	return scenario.Title
}

func (m *Model) demoDescription(scenario app.DemoScenario) string {
	if key, ok := demoDescriptionKeys[scenario.ID]; ok {
		return m.text(key)
	}
	return scenario.Description
}

// stageLabel 翻译已知的引擎阶段标识；未知阶段保持原样，避免改写诊断信息。
func (m *Model) stageLabel(stage string) string {
	normalized := strings.ToLower(strings.TrimSpace(stage))
	key, ok := map[string]i18n.Key{
		"start":    i18n.KeyLoadingStageStart,
		"detect":   i18n.KeyStageDetect,
		"parse":    i18n.KeyStageParse,
		"resolve":  i18n.KeyStageResolve,
		"scan":     i18n.KeyStageScan,
		"assemble": i18n.KeyStageAssemble,
		"list":     i18n.KeyStageList,
	}[normalized]
	if !ok {
		return strings.ToUpper(stage)
	}
	return m.text(key)
}
