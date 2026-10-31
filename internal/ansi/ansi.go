// ansi 包把字符画序列化为文本，可以选择附加 24 位颜色转义序列。
// 转义序列只在本包中生成。
// 图像里的任何内容（颜色值、字符集、文件名）都不会被复制进转义序列，因此输出流无法被用来注入终端控制码。
package ansi

import (
	"bufio"
	"io"
	"strconv"

	"asciiart/internal/render"
)

const (
	esc   = "\x1b["
	reset = "\x1b[0m"
	fgOff = "\x1b[39m"
	bgOff = "\x1b[49m"
)

// String 把字符画渲染为一个完整的字符串。
func String(a *render.Art, colored bool) string {
	var out []byte
	buf := newSliceWriter(&out)
	_ = Write(buf, a, colored)
	return string(out)
}

// Write 把字符画逐行写入 w。
// 颜色状态在单元格之间保持记录，连续多个相同颜色的单元格只需要输出一次转义序列，而不是每个单元格都输出一次。
// 这样一张 400 列的字符画就不会产生数兆字节的重复控制码。
func Write(w io.Writer, a *render.Art, colored bool) error {
	bw := bufio.NewWriterSize(w, 64<<10)
	var fg, bg string // 当前颜色状态，空字符串表示使用终端默认色。

	for y := 0; y < a.Rows; y++ {
		// 行尾的空白格不携带信息，只会让文件多出一串空格，因此先找出最后一个有内容的列。
		last := -1
		for x := a.Cols - 1; x >= 0; x-- {
			if !a.At(x, y).Blank() {
				last = x
				break
			}
		}
		if last < 0 {
			if err := bw.WriteByte('\n'); err != nil {
				return err
			}
			continue
		}

		for x := 0; x <= last; x++ {
			c := a.At(x, y)

			if !colored {
				if _, err := bw.WriteRune(cellRune(c)); err != nil {
					return err
				}
				continue
			}

			if c.Blank() {
				// 透明位置不能沿用前一个单元格的颜色，否则背景色会涂到这个位置上。
				if fg != "" || bg != "" {
					if _, err := bw.WriteString(reset); err != nil {
						return err
					}
					fg, bg = "", ""
				}
				if err := bw.WriteByte(' '); err != nil {
					return err
				}
				continue
			}

			if c.HasFG {
				if code := rgbCode("38", c.FG); code != fg {
					if _, err := bw.WriteString(code); err != nil {
						return err
					}
					fg = code
				}
			} else if fg != "" {
				if _, err := bw.WriteString(fgOff); err != nil {
					return err
				}
				fg = ""
			}

			if c.HasBG {
				if code := rgbCode("48", c.BG); code != bg {
					if _, err := bw.WriteString(code); err != nil {
						return err
					}
					bg = code
				}
			} else if bg != "" {
				if _, err := bw.WriteString(bgOff); err != nil {
					return err
				}
				bg = ""
			}

			if _, err := bw.WriteRune(cellRune(c)); err != nil {
				return err
			}
		}

		if fg != "" || bg != "" {
			if _, err := bw.WriteString(reset); err != nil {
				return err
			}
			fg, bg = "", ""
		}
		if err := bw.WriteByte('\n'); err != nil {
			return err
		}
	}
	return bw.Flush()
}

func cellRune(c render.Cell) rune {
	if c.Ch == 0 {
		return ' '
	}
	return c.Ch
}

func rgbCode(prefix string, c render.RGB) string {
	b := make([]byte, 0, 20)
	b = append(b, esc...)
	b = append(b, prefix...)
	b = append(b, ";2;"...)
	b = strconv.AppendUint(b, uint64(c.R), 10)
	b = append(b, ';')
	b = strconv.AppendUint(b, uint64(c.G), 10)
	b = append(b, ';')
	b = strconv.AppendUint(b, uint64(c.B), 10)
	b = append(b, 'm')
	return string(b)
}

// sliceWriter 把 *[]byte 适配为 io.Writer，避免仅为一个调用处引入 bytes 包。
type sliceWriter struct{ p *[]byte }

func newSliceWriter(p *[]byte) io.Writer { return sliceWriter{p} }

func (s sliceWriter) Write(b []byte) (int, error) {
	*s.p = append(*s.p, b...)
	return len(b), nil
}
