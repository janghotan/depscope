package i18n

import (
	"strings"
	"testing"
)

// TestCatalogsAreComplete 确保两种语言都覆盖全部类型化消息键。
func TestCatalogsAreComplete(t *testing.T) {
	english := englishCatalog()
	chinese := chineseCatalog()
	keys := AllKeys()
	if len(keys) != int(KeyCount) {
		t.Fatalf("AllKeys returned %d keys, want %d", len(keys), KeyCount)
	}
	for _, key := range keys {
		en := english[key]
		zh := chinese[key]
		if strings.TrimSpace(en) == "" {
			t.Errorf("English catalog is missing key %d", key)
		}
		if strings.TrimSpace(zh) == "" {
			t.Errorf("Chinese catalog is missing key %d", key)
		}
	}
}

// TestFormatVerbsMatch 确保参数化消息在两种语言中使用相同的格式化占位符。
func TestFormatVerbsMatch(t *testing.T) {
	english := englishCatalog()
	chinese := chineseCatalog()
	for _, key := range AllKeys() {
		enVerbs := formatVerbs(english[key])
		zhVerbs := formatVerbs(chinese[key])
		if strings.Join(enVerbs, ",") != strings.Join(zhVerbs, ",") {
			t.Errorf("key %d format verbs differ: English %v, Chinese %v", key, enVerbs, zhVerbs)
		}
	}
}

// TestFallbackIsEnglish 确保未知语言和非简体中文都回退到英语。
func TestFallbackIsEnglish(t *testing.T) {
	for _, language := range []Language{"", "fr", "zh-Hant", Language("EN")} {
		catalog := For(language)
		if catalog.Language() != English {
			t.Fatalf("language %q normalized to %q, want English", language, catalog.Language())
		}
		if got := catalog.Text(KeyBrandTagline); got != "Dependency risk, explained." {
			t.Fatalf("language %q returned %q, want English fallback", language, got)
		}
	}
}

// TestParseAndToggle 验证设置界面使用的语言解析与切换语义。
func TestParseAndToggle(t *testing.T) {
	cases := map[string]Language{
		"":        English,
		"en":      English,
		"English": English,
		"zh":      SimplifiedChinese,
		"zh-CN":   SimplifiedChinese,
		"zh-Hans": SimplifiedChinese,
		"简体中文":    SimplifiedChinese,
		"fr":      Default,
	}
	for input, want := range cases {
		got, ok := Parse(input)
		if got != want {
			t.Errorf("Parse(%q) = %q, want %q", input, got, want)
		}
		if input == "fr" && ok {
			t.Errorf("Parse(%q) reported a supported language", input)
		}
	}
	if English.Toggle() != SimplifiedChinese {
		t.Fatal("English.Toggle() did not return SimplifiedChinese")
	}
	if SimplifiedChinese.Toggle() != English {
		t.Fatal("SimplifiedChinese.Toggle() did not return English")
	}
	if SimplifiedChinese.NativeName() != "简体中文" {
		t.Fatalf("SimplifiedChinese.NativeName() = %q", SimplifiedChinese.NativeName())
	}
	if English.NativeName() != "English" {
		t.Fatalf("English.NativeName() = %q", English.NativeName())
	}
}

// TestChineseDiffers 确认中文目录确实提供了独立文案，而不是全部回退英语。
func TestChineseDiffers(t *testing.T) {
	chinese := For(SimplifiedChinese)
	for _, key := range []Key{KeyBrandTagline, KeyScreenStart, KeySettingsTitle, KeyReportHeadingAssessment} {
		if chinese.Text(key) == englishMessages[key] {
			t.Errorf("Chinese catalog still uses English text for key %d", key)
		}
	}
}

// formatVerbs 提取格式串中的转换动词，忽略 %%。
func formatVerbs(value string) []string {
	verbs := []string{}
	for index := 0; index < len(value); index++ {
		if value[index] != '%' {
			continue
		}
		index++
		if index >= len(value) {
			break
		}
		if value[index] == '%' {
			continue
		}
		for index < len(value) && strings.ContainsRune("#0+- .123456789", rune(value[index])) {
			index++
		}
		if index < len(value) {
			verbs = append(verbs, string(value[index]))
		}
	}
	return verbs
}
