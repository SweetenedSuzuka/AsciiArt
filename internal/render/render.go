// render 包把解码后的像素转换成字符网格。
// 渲染器把经过校验的 Options 配置应用到经过校验的源图像上。
// 每个渲染器都是纯函数，命令行界面和图形界面可以共用同一套渲染器。
package render

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"asciiart/internal/limits"
	"asciiart/internal/safedecode"
)

// Mode 选定一种渲染样式。
type Mode string

const (
	ModeASCII   Mode = "ascii"
	ModeBraille Mode = "braille"
	ModeBlocks  Mode = "blocks"
)

// AllModes 按照界面展示的顺序列出全部模式。
var AllModes = []Mode{ModeASCII, ModeBraille, ModeBlocks}

// ParseMode 接受模式的全名或者单字母别名。
func ParseMode(name string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "a", "ascii":
		return ModeASCII, nil
	case "b", "braille":
		return ModeBraille, nil
	case "c", "blocks", "colorblocks":
		return ModeBlocks, nil
	}
	return "", fmt.Errorf("未知模式 %q（可用：ascii / braille / blocks）", name)
}

// Label 返回模式的中文显示名称，供图形界面和状态输出使用。
func (m Mode) Label() string {
	switch m {
	case ModeASCII:
		return "ASCII"
	case ModeBraille:
		return "盲文"
	case ModeBlocks:
		return "半色块"
	}
	return string(m)
}

// RGB 表示一种不透明颜色。
type RGB struct{ R, G, B uint8 }

// Cell 是输出网格中的一个字符位。
// 一个 Cell 有三种情况：用前景色绘制一个字符、用不同的前景色和背景色绘制“▀”、留空（源像素透明）。零值 Cell 是空白。
type Cell struct {
	Ch    rune
	FG    RGB
	BG    RGB
	HasFG bool
	HasBG bool
}

// Blank 返回该字符位是否为空白，空白对应源图像中的透明像素。
func (c Cell) Blank() bool { return !c.HasFG && !c.HasBG }

// Art 是渲染完成的字符画。
type Art struct {
	Mode    Mode
	Cols    int
	Rows    int
	Cells   []Cell
	Elapsed time.Duration
	// Source 描述输入图像的信息，供状态行使用。
	SourceWidth, SourceHeight int
	Format                    safedecode.Format
}

// At 返回第 x 列、第 y 行的字符位。
func (a *Art) At(x, y int) Cell {
	if x < 0 || y < 0 || x >= a.Cols || y >= a.Rows {
		return Cell{}
	}
	return a.Cells[y*a.Cols+x]
}

// DefaultCharset 是 ASCII 渲染器使用的字符集，按从暗到亮的顺序排列。
const DefaultCharset = " .:-=+*#%@"

// Options 是一份完整并且经过校验的渲染请求描述。
type Options struct {
	Width          int
	AlphaThreshold uint8
	Invert         bool
	Charset        string
	Background     RGB

	// 仅用于盲文模式的调节参数。
	Sigma           float64 // 自适应阈值的模糊半径，以工作图像的像素为单位。
	Sharpen         float64 // 锐化强度，1.0 表示不锐化。
	Contrast        float64 // 对比度系数，1.0 表示不调整对比度。
	ThresholdOffset float64 // 阈值偏移量，以局部标准差为单位。

	// 仅用于半色块模式的调节参数。
	Scale float64 // 高度相对宽度的缩放比例。
}

// DefaultOptions 返回每个字段调整好的默认值。
func DefaultOptions() Options {
	return Options{
		Width:           80,
		AlphaThreshold:  128,
		Charset:         DefaultCharset,
		Background:      RGB{0, 0, 0},
		Sigma:           3.0,
		Sharpen:         1.2,
		Contrast:        1.5,
		ThresholdOffset: 0.4,
		Scale:           1.0,
	}
}

// Validate 规范化并检查每个字段。
// 未受信任的配置必须先经过这道校验，才能进入渲染器内部的内存分配。
func (o *Options) Validate() error {
	if err := limits.CheckOutputWidth(o.Width); err != nil {
		return err
	}
	if o.Charset == "" {
		o.Charset = DefaultCharset
	}
	runes := []rune(o.Charset)
	if len(runes) < 2 {
		return fmt.Errorf("字符集至少需要 2 个字符")
	}
	if len(runes) > 64 {
		return fmt.Errorf("字符集最多 64 个字符")
	}
	for _, r := range runes {
		// 控制字符和格式字符会让刻意构造的字符集把终端转义序列注入输出流。
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) || unicode.Is(unicode.Cf, r) {
			return fmt.Errorf("字符集包含控制字符 U+%04X", r)
		}
	}
	if o.Sigma < 0.1 || o.Sigma > 50 {
		return fmt.Errorf("sigma 必须在 0.1..50 之间")
	}
	if o.Sharpen < 1.0 || o.Sharpen > 5 {
		return fmt.Errorf("锐化强度必须在 1.0..5.0 之间")
	}
	if o.Contrast < 0.1 || o.Contrast > 10 {
		return fmt.Errorf("对比度必须在 0.1..10 之间")
	}
	if o.ThresholdOffset < 0 || o.ThresholdOffset > 3 {
		return fmt.Errorf("阈值系数必须在 0..3 之间")
	}
	if o.Scale < 0.1 || o.Scale > 4 {
		return fmt.Errorf("高度缩放必须在 0.1..4.0 之间")
	}
	return nil
}

// Render 按照请求的模式把渲染分发给对应的渲染器。
func Render(img *safedecode.Image, mode Mode, o Options) (*Art, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	start := time.Now()

	var (
		art *Art
		err error
	)
	switch mode {
	case ModeASCII:
		art, err = renderASCII(img, o)
	case ModeBraille:
		art, err = renderBraille(img, o)
	case ModeBlocks:
		art, err = renderBlocks(img, o)
	default:
		return nil, fmt.Errorf("未知模式 %q", mode)
	}
	if err != nil {
		return nil, err
	}

	if err := limits.CheckGrid(art.Cols, art.Rows); err != nil {
		return nil, err
	}
	art.Mode = mode
	art.Elapsed = time.Since(start)
	art.SourceWidth, art.SourceHeight, art.Format = img.Width, img.Height, img.Format
	return art, nil
}
