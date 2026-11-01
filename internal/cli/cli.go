// cli 包实现命令行接口。
// 所有用户输入都经过同一套解析、校验与渲染流程。
// 各步骤把错误作为返回值交给 Run，由 Run 打印错误并决定退出码，因此输入异常不会让程序崩溃。
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

const usageText = `AsciiArt — 把图片转换成字符画

用法：
  asciiart [选项] <图片文件>

选项：
  -m, --mode NAME       输出模式：ascii | braille | blocks | all，也可以用逗号组合（默认 all）
  -w, --width N         输出字符宽度，%d..%d（默认 80）
  -a, --threshold N     透明度阈值，低于该值视为透明，0..255（默认 128）
  -c, --colored         强制输出 24 位 ANSI 颜色
      --no-color        强制关闭颜色
  -i, --invert          反转亮度（ASCII 模式）
  -o, --output FILE     写入文件；all 模式会自动加 _ascii/_braille/_blocks 后缀
      --charset STR     ASCII 字符集，按从暗到亮排列（默认 %q）
      --bg #RRGGBB      透明区域的背景色（默认 #000000）
      --sigma F         盲文自适应阈值的模糊半径（默认 3.0）
      --sharpen F       盲文锐化强度，1.0 表示不锐化（默认 1.2）
      --contrast F      盲文对比度，1.0 表示不调整（默认 1.5）
      --k F             盲文阈值偏移量（默认 0.4）
      --scale F         半色块高度相对宽度的缩放比例（默认 1.0）
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
		thresholdShort  = fs.Int("a", 128, "")
		thresholdLong   = fs.Int("threshold", 128, "")
		thresholdDash   = fs.Int("alpha-threshold", 128, "")
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

	reordered := reorderArgs(fs, args)
	if err := fs.Parse(reordered); err != nil {
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

	// 同一组别名以最后出现在命令行里的那个为准。
	mode := *modeShort
	if lastFlagName(reordered, "m", "mode") == "mode" {
		mode = *modeLong
	}
	width := *widthShort
	if lastFlagName(reordered, "w", "width") == "width" {
		width = *widthLong
	}
	threshold := *thresholdShort
	switch lastFlagName(reordered, "a", "threshold", "alpha-threshold") {
	case "threshold":
		threshold = *thresholdLong
	case "alpha-threshold":
		threshold = *thresholdDash
	}
	invert := *invertShort || *invertLong
	outPath := *outputShort
	if lastFlagName(reordered, "o", "output") == "output" {
		outPath = *outputLong
	}
	forceColor := *coloredShort || *coloredLong

	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "缺少图片路径")
		fmt.Fprintln(stderr)
		printUsage(stderr)
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintf(stderr, "只能指定一个图片文件，目前值为 %d 个\n", fs.NArg())
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
		fmt.Fprintf(stderr, "透明度阈值必须在 0..255 之间，目前值为 %d\n", threshold)
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

	// 未指定 -c 时，只有输出目标是终端、没有设置 NO_COLOR 且 TERM 不是 dumb，才自动开启颜色，因此被重定向的标准输出与写入文件的内容不会带上转义序列。
	// 显式指定 -c 时按调用方的要求输出颜色，并在目标是终端时尝试开启虚拟终端处理。
	outputTerminal := terminalOf(stdout)
	toTerminal := outPath == "" && outputTerminal != nil && term.IsTerminal(outputTerminal)
	useANSI := false
	switch {
	case *noColor:
		useANSI = false
	case forceColor:
		useANSI = true
		if toTerminal {
			term.EnableANSI(outputTerminal)
		}
	default:
		useANSI = toTerminal && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
		if useANSI && !term.EnableANSI(outputTerminal) {
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

	// 输出到文件。指定多个模式时，输出路径会加上后缀，避免各结果互相覆盖。
	// 目录要在加后缀之前进行校验，当路径是目录时，Suffixed 会把“dist”变成“dist_ascii”，写入的文件会出现在目录旁边。
	// Windows 保留设备名同样要在加后缀之前校验，否则加后缀之后的名字不属于保留设备名，检查就失效了。
	if err := artio.CheckTarget(outPath); err != nil {
		fmt.Fprintf(stderr, "输出路径无效：%v\n", err)
		return 2
	}
	for _, art := range arts {
		target := outPath
		if len(arts) > 1 {
			target = artio.Suffixed(outPath, string(art.Mode))
		}
		if artio.SameTarget(target, fs.Arg(0)) {
			fmt.Fprintf(stderr, "输出路径 %s 与输入图片相同，拒绝覆盖该文件\n", target)
			return 2
		}
		content := ansi.String(art, useANSI)
		abs, err := artio.WriteFile(target, []byte(content))
		if err != nil {
			fmt.Fprintf(stderr, "写入 %s 失败：%v\n", target, err)
			return 1
		}
		fmt.Fprintf(stderr, "已写入 %s（%s，%d×%d 单元格）\n", abs, art.Mode.Label(), art.Cols, art.Rows)
	}
	return 0
}

// reorderArgs 把位置参数移到末尾，使标志可以出现在文件名之后。
// Go 的 flag 包遇到第一个非标志参数就会停止解析，因此“asciiart logo.png -w 100”里的 -w 和 100 会被当成多余的位置参数。
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

// lastFlagName 返回一组别名中最后出现在命令行里的名称，都没有出现时返回空字符串。
func lastFlagName(args []string, names ...string) string {
	last, index := "", -1
	for i, arg := range args {
		if len(arg) < 2 || arg[0] != '-' {
			continue
		}
		name := strings.TrimLeft(arg, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		for _, candidate := range names {
			if name == candidate && i > index {
				last, index = candidate, i
			}
		}
	}
	return last
}

func parseModes(s string) ([]render.Mode, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return nil, errors.New("输出模式不能为空")
	}
	if s == "all" {
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
	raw := strings.TrimSpace(s)
	hex := strings.TrimPrefix(raw, "#")
	if len(hex) != 6 {
		return render.RGB{}, fmt.Errorf("背景色必须是 #RRGGBB 形式，目前值为「%s」", raw)
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return render.RGB{}, fmt.Errorf("背景色解析失败：%v", err)
	}
	return render.RGB{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v)}, nil
}

// terminalOf 返回 w 对应的终端文件；w 不是 *os.File 时返回 nil。
func terminalOf(w io.Writer) *os.File {
	f, ok := w.(*os.File)
	if !ok {
		return nil
	}
	return f
}

// printTo 把结果写入 w。当 w 是终端、请求了多个模式且没有颜色时，各模式的输出会并排排列，便于比较。
// ANSI 颜色会使列宽无法确定，因此这种布局只用于纯文本。
func printTo(w io.Writer, arts []*render.Art, colored bool) error {
	if len(arts) > 1 && !colored {
		if f := terminalOf(w); f != nil && term.IsTerminal(f) {
			return writeSideBySide(w, arts)
		}
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
