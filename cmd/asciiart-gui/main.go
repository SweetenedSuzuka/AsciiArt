// asciiart-gui 是 AsciiArt 的图形界面程序。
//
// 用法:
//
//	asciiart-gui [选项] [图片]
//
// 选项与命令行工具的选项一致，程序因此可以带着配置好的参数启动，例如通过文件关联或者快捷方式启动。
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"asciiart/gui"
	"asciiart/internal/limits"
	"asciiart/internal/render"
)

// Version 的值由构建脚本在链接阶段写入。
var Version = "dev"

func main() {
	fs := flag.NewFlagSet("asciiart-gui", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	mode := fs.String("mode", "ascii", "初始模式：ascii | braille | blocks")
	width := fs.Int("width", 80, "初始输出宽度")
	showVersion := fs.Bool("version", false, "显示版本")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "AsciiArt GUI %s\n\n用法：asciiart-gui [选项] [图片]\n\n选项：\n", Version)
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if *showVersion {
		fmt.Printf("asciiart-gui %s\n", Version)
		return
	}

	opts := gui.Options{Mode: render.ModeASCII, Width: 80}
	if parsedMode, err := render.ParseMode(*mode); err == nil {
		opts.Mode = parsedMode
	}
	if *width >= limits.MinOutputWidth && *width <= limits.MaxOutputWidth {
		opts.Width = *width
	}
	if fs.NArg() > 0 {
		opts.Image = strings.Trim(fs.Arg(0), "\"")
	}

	os.Exit(gui.Run(opts))
}
