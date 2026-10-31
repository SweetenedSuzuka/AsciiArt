package render

import (
	"strings"
	"testing"

	"asciiart/internal/safedecode"
)

// solid 生成一张单一颜色、完全不透明的图像。
func solid(w, h int, c RGB) *safedecode.Image {
	img := &safedecode.Image{Width: w, Height: h, Pix: make([]byte, w*h*4)}
	for i := 0; i < w*h; i++ {
		img.Pix[i*4] = c.R
		img.Pix[i*4+1] = c.G
		img.Pix[i*4+2] = c.B
		img.Pix[i*4+3] = 255
	}
	return img
}

// blank 生成一张完全透明的图像。
func blank(w, h int) *safedecode.Image {
	return &safedecode.Image{Width: w, Height: h, Pix: make([]byte, w*h*4)}
}

func TestASCIIGridShape(t *testing.T) {
	img := solid(20, 10, RGB{255, 255, 255})
	o := DefaultOptions()
	o.Width = 10

	art, err := Render(img, ModeASCII, o)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// 字符位的高度大约是宽度的两倍，因此 20x10 的源图像在宽度 10 下必须输出 10x2 的网格。
	if art.Cols != 10 || art.Rows != 2 {
		t.Fatalf("grid = %dx%d, want 10x2", art.Cols, art.Rows)
	}
	if len(art.Cells) != art.Cols*art.Rows {
		t.Fatalf("cell count = %d, want %d", len(art.Cells), art.Cols*art.Rows)
	}
}

func TestASCIIBrightnessRamp(t *testing.T) {
	o := DefaultOptions()
	o.Width = 2

	white, err := Render(solid(2, 2, RGB{255, 255, 255}), ModeASCII, o)
	if err != nil {
		t.Fatal(err)
	}
	if got := white.At(0, 0).Ch; got != '@' {
		t.Errorf("white pixel -> %q, want '@'", got)
	}

	black, err := Render(solid(2, 2, RGB{0, 0, 0}), ModeASCII, o)
	if err != nil {
		t.Fatal(err)
	}
	if got := black.At(0, 0).Ch; got != ' ' {
		t.Errorf("black pixel -> %q, want ' '", got)
	}
}

func TestASCIITransparency(t *testing.T) {
	o := DefaultOptions()
	o.Width = 4

	art, err := Render(blank(8, 8), ModeASCII, o)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range art.Cells {
		if !c.Blank() {
			t.Fatalf("cell %d is not blank: %+v", i, c)
		}
	}
}

func TestASCIIInvert(t *testing.T) {
	o := DefaultOptions()
	o.Width = 2
	o.Invert = true

	art, err := Render(solid(2, 2, RGB{255, 255, 255}), ModeASCII, o)
	if err != nil {
		t.Fatal(err)
	}
	if got := art.At(0, 0).Ch; got != ' ' {
		t.Errorf("inverted white -> %q, want ' '", got)
	}
}

// TestBrailleDotOrder 固定 Unicode 的点位顺序：
// 完整的左列必须设置第 0-3 位（第 1-4 点），即 U+280F。
// 顺序写错以后，生成的字符看起来像正常的盲文，编码的内容却是错的。
func TestBrailleDotOrder(t *testing.T) {
	const w, h = 8, 16
	img := solid(w, h, RGB{255, 255, 255})
	// 把第一个 2x4 字符位左列的四个像素涂黑。
	for y := 0; y < 4; y++ {
		i := (y*w + 0) * 4
		img.Pix[i], img.Pix[i+1], img.Pix[i+2] = 0, 0, 0
	}

	o := DefaultOptions()
	o.Width = w / 2 // pw 等于 w、ph 等于 h，不做重采样，像素一一对应。
	o.Sigma = 0.5
	o.Sharpen = 1.0
	o.Contrast = 1.0

	art, err := Render(img, ModeBraille, o)
	if err != nil {
		t.Fatal(err)
	}
	if art.Cols != 4 || art.Rows != 4 {
		t.Fatalf("grid = %dx%d, want 4x4", art.Cols, art.Rows)
	}
	if got := art.At(0, 0).Ch; got != 0x280F {
		t.Errorf("left column -> U+%04X, want U+280F (dots 1-4)", got)
	}
}

func TestBrailleSingleDot(t *testing.T) {
	const w, h = 8, 16
	img := solid(w, h, RGB{255, 255, 255})
	i := 0 // 索引 0 对应像素 (0,0)。
	img.Pix[i], img.Pix[i+1], img.Pix[i+2] = 0, 0, 0

	o := DefaultOptions()
	o.Width = w / 2
	o.Sigma = 0.5
	o.Sharpen = 1.0
	o.Contrast = 1.0

	art, err := Render(img, ModeBraille, o)
	if err != nil {
		t.Fatal(err)
	}
	if got := art.At(0, 0).Ch; got != 0x2801 {
		t.Errorf("top-left dot -> U+%04X, want U+2801 (dot 1)", got)
	}
}

func TestBlocksRequireBothHalvesOpaque(t *testing.T) {
	img := solid(4, 4, RGB{10, 20, 30})
	// 在第一个字符位的下半行打一个透明孔。
	for x := 0; x < 4; x++ {
		img.Pix[((1)*4+x)*4+3] = 0
	}
	o := DefaultOptions()
	o.Width = 4

	art, err := Render(img, ModeBlocks, o)
	if err != nil {
		t.Fatal(err)
	}
	if !art.At(0, 0).Blank() {
		t.Errorf("cell with a transparent half should be blank, got %+v", art.At(0, 0))
	}
}

func TestBlocksColours(t *testing.T) {
	img := solid(2, 2, RGB{0, 0, 0})
	// 上一行红色，下一行蓝色。
	for x := 0; x < 2; x++ {
		i := x * 4
		img.Pix[i], img.Pix[i+1], img.Pix[i+2] = 255, 0, 0
		j := (2 + x) * 4
		img.Pix[j], img.Pix[j+1], img.Pix[j+2] = 0, 0, 255
	}
	o := DefaultOptions()
	o.Width = 2

	art, err := Render(img, ModeBlocks, o)
	if err != nil {
		t.Fatal(err)
	}
	c := art.At(0, 0)
	if c.Ch != '▀' || c.FG != (RGB{255, 0, 0}) || c.BG != (RGB{0, 0, 255}) {
		t.Errorf("cell = %+v, want ▀ fg red bg blue", c)
	}
}

func TestOptionsValidate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Options)
	}{
		{"width too small", func(o *Options) { o.Width = 0 }},
		{"width too large", func(o *Options) { o.Width = 100000 }},
		{"charset control char", func(o *Options) { o.Charset = "ab\x1b[31mc" }},
		{"charset too short", func(o *Options) { o.Charset = "x" }},
		{"sigma out of range", func(o *Options) { o.Sigma = 0 }},
		{"sharpen below one", func(o *Options) { o.Sharpen = 0.5 }},
		{"scale out of range", func(o *Options) { o.Scale = 100 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := DefaultOptions()
			tc.mutate(&o)
			if err := o.Validate(); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}

func TestDefaultOptionsAreValid(t *testing.T) {
	o := DefaultOptions()
	if err := o.Validate(); err != nil {
		t.Fatalf("default options must validate: %v", err)
	}
	if !strings.Contains(DefaultCharset, "@") {
		t.Fatalf("default charset lost its brightest glyph")
	}
}

func TestRenderRejectsOversizedGrid(t *testing.T) {
	// 1x20000 的源图像在宽度 400 下会生成远超单元上限的网格。
	img := solid(2, 20000, RGB{128, 128, 128})
	o := DefaultOptions()
	o.Width = 400
	if _, err := Render(img, ModeASCII, o); err == nil {
		t.Fatal("expected grid size error")
	}
}
