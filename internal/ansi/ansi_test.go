package ansi

import (
	"bytes"
	"strings"
	"testing"

	"asciiart/internal/render"
)

func art(cells []render.Cell, cols, rows int) *render.Art {
	return &render.Art{Cols: cols, Rows: rows, Cells: cells}
}

func TestPlainOutputHasNoEscapes(t *testing.T) {
	a := art([]render.Cell{
		{Ch: 'A', FG: rgb(255, 0, 0), HasFG: true},
		{Ch: 'B', FG: rgb(0, 255, 0), HasFG: true},
	}, 2, 1)

	out := String(a, false)
	if strings.ContainsRune(out, 0x1b) {
		t.Fatalf("plain output contains an escape sequence: %q", out)
	}
	if out != "AB\n" {
		t.Fatalf("plain output = %q, want %q", out, "AB\n")
	}
}

func TestColoredOutput(t *testing.T) {
	a := art([]render.Cell{
		{Ch: 'A', FG: rgb(255, 0, 0), HasFG: true},
		{Ch: 'B', FG: rgb(0, 255, 0), HasFG: true},
	}, 2, 1)

	out := String(a, true)
	if !strings.Contains(out, "\x1b[38;2;255;0;0m") {
		t.Fatalf("missing first colour: %q", out)
	}
	if !strings.Contains(out, "\x1b[38;2;0;255;0m") {
		t.Fatalf("missing second colour: %q", out)
	}
	if !strings.HasSuffix(out, "\x1b[0m\n") {
		t.Fatalf("line should end with a reset: %q", out)
	}
}

func TestColorRunCompression(t *testing.T) {
	a := art([]render.Cell{
		{Ch: 'A', FG: rgb(1, 2, 3), HasFG: true},
		{Ch: 'B', FG: rgb(1, 2, 3), HasFG: true},
		{Ch: 'C', FG: rgb(1, 2, 3), HasFG: true},
	}, 3, 1)

	out := String(a, true)
	if n := strings.Count(out, "\x1b[38;2;1;2;3m"); n != 1 {
		t.Fatalf("colour emitted %d times, want 1: %q", n, out)
	}
}

func TestBlankCellResetsColor(t *testing.T) {
	a := art([]render.Cell{
		{Ch: 'A', FG: rgb(255, 0, 0), HasFG: true},
		{}, // 透明空隙。
		{Ch: 'B', FG: rgb(255, 0, 0), HasFG: true},
	}, 3, 1)

	out := String(a, true)
	// 由于空隙必须保持透明，因此必须先输出重置序列，再在空格之后重新输出颜色。
	if !strings.Contains(out, "A\x1b[0m \x1b[38;2;255;0;0mB") {
		t.Fatalf("blank cell did not reset: %q", out)
	}
}

func TestTrailingBlanksTrimmed(t *testing.T) {
	a := art([]render.Cell{
		{Ch: 'A', FG: rgb(9, 9, 9), HasFG: true},
		{}, {}, {},
	}, 4, 1)

	out := String(a, false)
	if out != "A\n" {
		t.Fatalf("output = %q, want %q", out, "A\n")
	}
}

func TestBlocksEmitForegroundAndBackground(t *testing.T) {
	a := art([]render.Cell{
		{Ch: '▀', FG: rgb(10, 20, 30), BG: rgb(40, 50, 60), HasFG: true, HasBG: true},
	}, 1, 1)

	out := String(a, true)
	if !strings.Contains(out, "\x1b[38;2;10;20;30m") {
		t.Fatalf("missing foreground: %q", out)
	}
	if !strings.Contains(out, "\x1b[48;2;40;50;60m") {
		t.Fatalf("missing background: %q", out)
	}
}

func TestWriteMatchesString(t *testing.T) {
	a := art([]render.Cell{
		{Ch: 'X', FG: rgb(1, 2, 3), HasFG: true},
	}, 1, 1)

	var buf bytes.Buffer
	if err := Write(&buf, a, true); err != nil {
		t.Fatal(err)
	}
	if buf.String() != String(a, true) {
		t.Fatalf("Write and String disagree")
	}
}

func rgb(r, g, b uint8) render.RGB { return render.RGB{R: r, G: g, B: b} }
