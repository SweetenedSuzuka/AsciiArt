package gui

import (
	"strings"
	"testing"

	"asciiart/internal/render"
)

// sampleArt 返回两个相邻单元格的字符画，用来检查颜色转义序列的输出。
func sampleArt() *render.Art {
	return &render.Art{Cols: 2, Rows: 1, Cells: []render.Cell{
		{Ch: 'A', FG: render.RGB{R: 255, G: 0, B: 0}, HasFG: true},
		{Ch: 'B', FG: render.RGB{R: 0, G: 0, B: 255}, HasFG: true},
	}}
}

// artText 生成导出文件与复制到剪贴板共用的文本。
// 它必须随「彩色」开关变化，否则预览是彩色的、导出的内容却是单色的。
func TestArtTextFollowsColorToggle(t *testing.T) {
	colored := (&App{art: sampleArt(), colorOn: true}).artText()
	if !strings.Contains(colored, "\x1b[38;2;255;0;0m") {
		t.Fatalf("「彩色」开启时文本缺少第一个单元格的颜色：%q", colored)
	}
	if !strings.Contains(colored, "\x1b[38;2;0;0;255m") {
		t.Fatalf("「彩色」开启时文本缺少第二个单元格的颜色：%q", colored)
	}

	plain := (&App{art: sampleArt(), colorOn: false}).artText()
	if strings.ContainsRune(plain, 0x1b) {
		t.Fatalf("「彩色」关闭时文本仍带有转义序列：%q", plain)
	}
	if plain != "AB\n" {
		t.Fatalf("「彩色」关闭时的文本为 %q，应为 %q", plain, "AB\n")
	}
}

func TestArtTextWithoutRender(t *testing.T) {
	if got := (&App{colorOn: true}).artText(); got != "" {
		t.Fatalf("没有渲染结果时应返回空字符串，目前是 %q", got)
	}
}

func TestColorStateLabelFollowsToggle(t *testing.T) {
	if got := (&App{colorOn: true}).colorStateLabel(); !strings.Contains(got, "ANSI") {
		t.Fatalf("「彩色」开启时状态栏应说明带有 ANSI 颜色，目前是 %q", got)
	}
	if got := (&App{colorOn: false}).colorStateLabel(); !strings.Contains(got, "纯文本") {
		t.Fatalf("「彩色」关闭时状态栏应说明是纯文本，目前是 %q", got)
	}
}
