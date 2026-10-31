// cli 包实现命令行接口。
// 所有用户输入都经过同一个解析、校验和渲染步骤。
// 错误以值的形式返回，并在统一的位置转换成退出码，因此任何代码路径都不会因为输入异常而崩溃。
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"asciiart/internal/ansi"
	"asciiart/internal/artio"
	"asciiart/internal/limits"
	"asciiart/internal/render"
	"asciiart/internal/safedecode"
	"asciiart/internal/term"
)

// Version 由构建脚本在链接时写入。
var Version = "dev"

const usageText = `AsciiArt — 把图片转成字符画

用法：
  asciiart [选项] <图片文件>

选项：
  -m, --mode NAME       输出模式：ascii | braille | blocks | all（默认 all）
  -w, --width N         输出字符宽度，%d..%d（默认 80）
  -a, --threshold N     透明度阈值，低于该值视为透明，0..255（默认 128）
  -c, --colored         强制输出 24 位 ANSI 颜色
      --no-color        强制关闭颜色
  -i, --invert          反转亮度（ASCII 模式）
  -o, --output FILE     写入文件；all 模式会自动加 _ascii/_braille/_blocks 后缀
      --charset STR     ASCII 亮度梯度（默认 %q）
      --bg #RRGGBB      透明区域的背景色（默认 #000000）
      --sigma F         盲文自适应阈值模糊半径（默认 3.0）
      --sharpen F       盲文边缘增强强度，1.0 关闭（默认 1.2）
      --contrast F      盲文对比度，1.0 关闭（默认 1.5）
      --k F             盲文阈值偏移系数（默认 0.4）
      --scale F         半色块纵向缩放（默认 1.0）
  -h, --help            显示帮助
  -v, --version         显示版本

示例：
  asciiart logo.png -w 100 -c
  asciiart logo.png -m braille -o braille.txt
  asciiart photo.jpg -m blocks -w 120 --no-color

不指定 -o 时结果输出到标准输出，不会在当前目录留下任何文件。
`

// Run 执行命令行接口并返回进程退出码。
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("asciiart", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // 用法文本由本包自己打印，不交给 flag 包输出。

	var (
		modeShort       = fs.String("m", "all", "")
		modeLong        = fs.String("mode", "all", "")
		widthShort      = fs.Int("w", 80, "")
		widthLong       = fs.Int("width", 80, "")
		alphaShort      = fs.Int("a", 128, "")
		alphaLong       = fs.Int("threshold", 128, "")
		alphaDash       = fs.Int("alpha-threshold", 128, "")
		coloredShort    = fs.Bool("c", false, "")
		coloredLong     = fs.Bool("colored", false, "")
		noColor         = fs.Bool("no-color", false, "")
		invertShort     = fs.Bool("i", false, "")
		invertLong      = fs.Bool("invert", false, "")
		outputShort     = fs.String("o", "", "")
		outputLong      = fs.String("output", "", "")
		charset         = fs.String("charset", render.DefaultCharset, "")
		bgHex           = fs.String("bg", "#000000", "")
		sigma           = fs.Float64("sigma", 3.0, "")
		sharpen         = fs.Float64("sharpen", 1.2, "")
		contrast        = fs.Float64("contrast", 1.5, "")
		thresholdOffset = fs.Float64("k", 0.4, "")
		scale           = fs.Float64("scale", 1.0, "")
		versionShort    = fs.Bool("v", false, "")
		versionLong     = fs.Bool("version", false, "")
		helpShort       = fs.Bool("h", false, "")
		helpLong        = fs.Bool("help", false, "")
	)

	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(stdout)
			return 0
		}
		fmt.Fprintf(stderr, "参数错误：%v\n\n", err)
		printUsage(stderr)
		return 2
	}
	if *helpShort || *helpLong {
		printUsage(stdout)
		return 0
	}
	if *versionShort || *versionLong {
		fmt.Fprintf(stdout, "asciiart %s\n", Version)
		return 0
	}

	// 后出现的标志优先生效，即仅当短选项保持默认值时，长选项的显式指定才覆盖它。
	// 两种别名因此不会互相覆盖。
	mode := *modeShort
	if *modeLong != "all" {
		mode = *modeLong
	}
	width := *widthShort
	if *widthLong != 80 {
		width = *widthLong
	}
	threshold := *alphaShort
	if *alphaLong != 128 {
		threshold = *alphaLong
	}
	if *alphaDash != 128 {
		threshold = *alphaDash
	}
	invert := *invertShort || *invertLong
	outPath := *outputShort
	if outPath == "" {
		outPath = *outputLong
	}
	useColor := *coloredShort || *coloredLong

	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "缺少图片路径")
		fmt.Fprintln(stderr)
		printUsage(stderr)
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintf(stderr, "只能指定一个图片文件，收到 %d 个\n", fs.NArg())
		return 2
	}

	modes, err := parseModes(mode)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}

	bg, err := parseHexColor(*bgHex)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}

	if threshold < 0 || threshold > 255 {
		fmt.Fprintf(stderr, "透明度阈值必须在 0..255 之间，收到 %d\n", threshold)
		return 2
	}

	opts := render.DefaultOptions()
	opts.Width = width
	opts.AlphaThreshold = uint8(threshold)
	opts.Invert = invert
	opts.Charset = *charset
	opts.Background = bg
	opts.Sigma = *sigma
	opts.Sharpen = *sharpen
	opts.Contrast = *contrast
	opts.ThresholdOffset = *thresholdOffset
	opts.Scale = *scale
	if err := opts.Validate(); err != nil {
		fmt.Fprintf(stderr, "参数无效：%v\n", err)
		return 2
	}

	img, err := safedecode.Decode(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "无法读取图片：%v\n", err)
		return 1
	}

	arts := make([]*render.Art, 0, len(modes))
	for _, m := range modes {
		art, err := render.Render(img, m, opts)
		if err != nil {
			fmt.Fprintf(stderr, "%s 模式渲染失败：%v\n", m.Label(), err)
			return 1
		}
		arts = append(arts, art)
	}

	// 颜色在终端上需要显式开启，在其它场合默认关闭，因此被重定向的标准输出不会收到它无法解释的转义序列。
	useANSI := false
	switch {
	case *noColor:
		useANSI = false
	case useColor:
		useANSI = true
	default:
		useANSI = term.IsTerminal(os.Stdout) && os.Getenv("NO_COLOR") == "" &&
			os.Getenv("TERM") != "dumb"
	}
	if useANSI && !useColor {
		if !term.EnableANSI(os.Stdout) {
			useANSI = false
		}
	}

	if outPath == "" {
		if err := printTo(stdout, arts, useANSI); err != nil {
			fmt.Fprintf(stderr, "输出失败：%v\n", err)
			return 1
		}
		return 0
	}

	// 输出到文件。当指定了多个模式时，基础文件名会加上后缀，避免三个结果互相覆盖。
	// 基础路径会事先校验：目录或设备名必须在 Suffixed 把“dist”变成“dist_ascii” 之前被拒绝，否则会在用户意图所在的目录旁边静默写入成功。
	if err := artio.CheckTarget(outPath); err != nil {
		fmt.Fprintf(stderr, "输出路径无效：%v\n", err)
		return 2
	}
	for _, art := range arts {
		target := outPath
		if len(arts) > 1 {
			target = artio.Suffixed(outPath, string(art.Mode))
		}
		content := ansi.String(art, useANSI)
		abs, err := artio.WriteFile(target, []byte(content))
		if err != nil {
			fmt.Fprintf(stderr, "写入 %s 失败：%v\n", target, err)
			return 1
		}
		fmt.Fprintf(stderr, "已写入 %s (%s, %dx%d)\n", abs, art.Mode.Label(), art.Cols, art.Rows)
	}
	return 0
}

// reorderArgs 把位置参数移到末尾，使标志可以出现在文件名之后。
// Go 的 flag 包遇到第一个非标志参数就会停止解析，否则“asciiart logo.png -w 100”会被静默地当成五个多余的位置参数。
// 这里通过查询 FlagSet 来判断哪些选项需要附带值，因此传给 --k 的负数仍然会被当作值而不是标志。
func reorderArgs(fs *flag.FlagSet, args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) < 2 || a[0] != '-' {
			positional = append(positional, a)
			continue
		}

		flags = append(flags, a)
		name := strings.TrimLeft(a, "-")
		if strings.ContainsRune(name, '=') {
			continue // --name=value 自带值。
		}
		f := fs.Lookup(name)
		if f == nil {
			continue // 未知标志，交给 Parse 报错。
		}
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

func parseModes(s string) ([]render.Mode, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "all" || s == "" {
		return render.AllModes, nil
	}
	// 接受以逗号分隔的组合，例如“ascii,blocks”。
	var out []render.Mode
	seen := map[render.Mode]bool{}
	for _, part := range strings.Split(s, ",") {
		m, err := render.ParseMode(part)
		if err != nil {
			return nil, err
		}
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("没有可用的输出模式")
	}
	return out, nil
}

func parseHexColor(s string) (render.RGB, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return render.RGB{}, fmt.Errorf("背景色必须是 #RRGGBB 形式，收到 %q", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return render.RGB{}, fmt.Errorf("背景色解析失败：%v", err)
	}
	return render.RGB{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v)}, nil
}

// printTo 把结果写入 w。当在终端上请求所有模式时，各面板会并排排列，便于一眼比较三个结果。
// ANSI 颜色会使列宽无法确定，因此这种布局只用于纯文本。
func printTo(w io.Writer, arts []*render.Art, colored bool) error {
	if len(arts) > 1 && !colored && term.IsTerminal(os.Stdout) {
		return writeSideBySide(w, arts)
	}
	for i, art := range arts {
		if len(arts) > 1 {
			if i > 0 {
				if _, err := io.WriteString(w, "\n"); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(w, "── %s ──\n", art.Mode.Label()); err != nil {
				return err
			}
		}
		if err := ansi.Write(w, art, colored); err != nil {
			return err
		}
	}
	return nil
}

func writeSideBySide(w io.Writer, arts []*render.Art) error {
	maxRows := 0
	widths := make([]int, len(arts))
	for i, a := range arts {
		if a.Rows > maxRows {
			maxRows = a.Rows
		}
		if a.Cols > widths[i] {
			widths[i] = a.Cols
		}
	}
	var b strings.Builder
	for y := 0; y < maxRows; y++ {
		for i, a := range arts {
			if i > 0 {
				b.WriteString("   ")
			}
			if y < a.Rows {
				line := lineOf(a, y)
				b.WriteString(line)
				if pad := widths[i] - len([]rune(line)); pad > 0 {
					b.WriteString(strings.Repeat(" ", pad))
				}
			} else {
				b.WriteString(strings.Repeat(" ", widths[i]))
			}
		}
		b.WriteByte('\n')
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func lineOf(a *render.Art, y int) string {
	rs := make([]rune, 0, a.Cols)
	last := -1
	for x := 0; x < a.Cols; x++ {
		c := a.At(x, y)
		if !c.Blank() {
			last = x
		}
	}
	for x := 0; x <= last; x++ {
		c := a.At(x, y)
		if c.Ch == 0 {
			rs = append(rs, ' ')
		} else {
			rs = append(rs, c.Ch)
		}
	}
	return string(rs)
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, usageText, limits.MinOutputWidth, limits.MaxOutputWidth, render.DefaultCharset)
}
