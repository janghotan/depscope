// Package i18n 提供 DepScope 交互界面的离线双语文案目录。
// 所有消息以类型化键集中管理；未知语言与非空回退始终使用英语，
// 界面不得以英文句子作为键，也不得在运行时访问网络翻译服务。
package i18n

import (
	"fmt"
	"strings"
)

// Language 是界面显示语言的稳定标识。
type Language string

const (
	// English 是默认语言，也是所有回退路径的目标语言。
	English Language = "en"
	// SimplifiedChinese 是简体中文界面语言。
	SimplifiedChinese Language = "zh-Hans"
	// Default 是新会话使用的语言。
	Default = English
)

// Parse 解析用户或配置提供的语言标识；无法识别时返回默认语言与 false。
func Parse(value string) (Language, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "en", "en-us", "en-gb", "english":
		return English, true
	case "zh", "zh-cn", "zh-hans", "zh-hans-cn", "chinese", "simplified chinese", "简体中文":
		return SimplifiedChinese, true
	default:
		return Default, false
	}
}

// Normalize 把零值或未知语言归一为英语，保证渲染路径总有可用目录。
func Normalize(language Language) Language {
	if language == SimplifiedChinese {
		return SimplifiedChinese
	}
	return English
}

// NativeName 返回语言自身的名称，供设置界面直接显示。
func (l Language) NativeName() string {
	if Normalize(l) == SimplifiedChinese {
		return For(SimplifiedChinese).Text(KeyLanguageChinese)
	}
	return For(English).Text(KeyLanguageEnglish)
}

// Toggle 返回另一种受支持的语言；用于设置界面的单键切换。
func (l Language) Toggle() Language {
	if Normalize(l) == SimplifiedChinese {
		return English
	}
	return SimplifiedChinese
}

// Catalog 是某个语言的消息集合；缺失条目回退到英语目录。
type Catalog struct {
	language Language
	messages map[Key]string
	fallback map[Key]string
}

var (
	englishMessages   = englishCatalog()
	chineseMessages   = chineseCatalog()
	englishCatalogVal = Catalog{language: English, messages: englishMessages, fallback: englishMessages}
)

// For 返回指定语言的消息目录；未知语言使用英语。
func For(language Language) Catalog {
	language = Normalize(language)
	if language == SimplifiedChinese {
		return Catalog{language: language, messages: chineseMessages, fallback: englishMessages}
	}
	return englishCatalogVal
}

// Language 返回目录对应的语言。
func (c Catalog) Language() Language {
	return Normalize(c.language)
}

// Text 返回消息文本；当前语言缺失或为空时回退到英语。
func (c Catalog) Text(key Key) string {
	if value := c.messages[key]; value != "" {
		return value
	}
	if value := c.fallback[key]; value != "" {
		return value
	}
	return ""
}

// Format 先取本地化格式串，再使用 fmt 填充参数。
// 调用方必须传入与格式串匹配的参数，测试会覆盖所有参数化消息。
func (c Catalog) Format(key Key, args ...any) string {
	return fmt.Sprintf(c.Text(key), args...)
}

// Has 报告当前目录与英语回退是否都包含该键。
func (c Catalog) Has(key Key) bool {
	return c.messages[key] != "" || c.fallback[key] != ""
}

// AllKeys 按声明顺序返回全部消息键，供完整性测试遍历。
func AllKeys() []Key {
	keys := make([]Key, 0, int(KeyCount))
	for key := Key(0); key < KeyCount; key++ {
		keys = append(keys, key)
	}
	return keys
}
