package termtext

import (
	"strings"
	"testing"
)

// TestWidthCountsDisplayCells 验证中文与 ANSI 序列的宽度计算。
func TestWidthCountsDisplayCells(t *testing.T) {
	if got := Width("简体中文"); got != 8 {
		t.Fatalf("Width(简体中文) = %d, want 8", got)
	}
	if got := Width("\x1b[31m简体中文\x1b[0m"); got != 8 {
		t.Fatalf("Width with ANSI = %d, want 8", got)
	}
	if got := Width("abc"); got != 3 {
		t.Fatalf("Width(abc) = %d, want 3", got)
	}
}

// TestTruncateKeepsWidthAndANSI 验证截断按显示宽度进行并保留样式序列。
func TestTruncateKeepsWidthAndANSI(t *testing.T) {
	cases := []struct {
		value string
		width int
		want  string
	}{
		{"简体中文测试", 5, "简体…"},
		{"简体中文测试", 1, "…"},
		{"abc", 5, "abc"},
		{"\x1b[31m简体中文测试\x1b[0m", 5, "\x1b[31m简体…\x1b[0m"},
		{"abc", 0, ""},
	}
	for _, tc := range cases {
		got := Truncate(tc.value, tc.width, "…")
		if got != tc.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tc.value, tc.width, got, tc.want)
		}
		if tc.width > 0 && Width(got) > tc.width {
			t.Errorf("Truncate(%q, %d) produced width %d", tc.value, tc.width, Width(got))
		}
	}
}

// TestPadRightUsesDisplayWidth 验证补齐按单元格而不是 rune 数量。
func TestPadRightUsesDisplayWidth(t *testing.T) {
	got := PadRight("中文", 6)
	if got != "中文  " {
		t.Fatalf("PadRight = %q, want %q", got, "中文  ")
	}
	if Width(got) != 6 {
		t.Fatalf("padded width = %d, want 6", Width(got))
	}
	if got := PadRight("abcdef", 3); got != "abcdef" {
		t.Fatalf("PadRight should not truncate, got %q", got)
	}
}

// TestWrapEnglishAndChinese 验证两种语言都不会产生超出宽度的行。
func TestWrapEnglishAndChinese(t *testing.T) {
	cases := []string{
		"Reads registry metadata and advisories. Never installs or executes packages.",
		"依赖风险，清晰解释。这是一个较长的中文句子，用来测试换行。",
		"混合 mixed 文本 with English words 以及中文。",
		"supercalifragilisticexpialidocious",
	}
	for _, value := range cases {
		for _, width := range []int{1, 2, 5, 10, 20, 42} {
			lines := Wrap(value, width)
			for _, line := range lines {
				if Width(line) > width && Width(line) > 2 {
					t.Fatalf("Wrap(%q, %d) produced line %q with width %d", value, width, line, Width(line))
				}
				if line == "" && strings.TrimSpace(value) != "" {
					t.Fatalf("Wrap(%q, %d) produced an empty line", value, width)
				}
			}
		}
	}
}

// TestWrapPreservesParagraphs 验证显式换行与空行得到保留。
func TestWrapPreservesParagraphs(t *testing.T) {
	lines := Wrap("first line\n\nsecond paragraph", 20)
	want := []string{"first line", "", "second paragraph"}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Fatalf("Wrap = %v, want %v", lines, want)
	}
}

// TestCellWindowKeepsCursorVisible 验证输入窗口在中文与长文本下保持光标可见。
func TestCellWindowKeepsCursorVisible(t *testing.T) {
	value := []rune("简体中文测试")
	visible, cursorCell, start := CellWindow(value, 6, 5)
	if start != 4 {
		t.Fatalf("start = %d, want 4", start)
	}
	if visible != "测试" {
		t.Fatalf("visible = %q, want 测试", visible)
	}
	if cursorCell != 4 || cursorCell >= 5 {
		t.Fatalf("cursorCell = %d, want 4", cursorCell)
	}

	long := []rune("abcdefghij")
	visible, cursorCell, start = CellWindow(long, 10, 4)
	// 窗口需要给光标本身留一列，因此显示最后三个字符。
	if visible != "hij" || cursorCell != 3 || start != 7 {
		t.Fatalf("window = %q, cursor %d, start %d", visible, cursorCell, start)
	}
}
