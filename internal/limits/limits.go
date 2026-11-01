// limits 包集中保存 AsciiArt 使用的所有硬性资源上限。
// 一个 200 字节的 PNG 可以展开成数 GB 的像素数据，因此所有处理不可信输入的路径都只在这里设置上限。
// 把这些数值集中在一个文件里方便审计。
package limits

import "fmt"

const (
	// MaxFileBytes 是程序读取源文件的最大字节数。
	MaxFileBytes int64 = 64 << 20 // 64 MiB

	// MaxPixels 限制解码后的像素总数。
	// 4000 万高于 2400 万相机照片的像素数，同时把一次解码的 RGBA 数据量控制在约 160 MiB。
	MaxPixels int64 = 40_000_000

	// MaxSide 限制解码后单条边（宽或高）的长度。
	// 例如一张 1 x 40,000,000 的图像能通过像素总数检查，但明显是非正常文件，因此宽和高还要各自单独限制。
	MaxSide = 20_000

	// MaxOutputWidth 是程序生成的字符网格的最大宽度。
	MaxOutputWidth = 400

	// MinOutputWidth 是字符网格有意义的最小宽度。
	MinOutputWidth = 1

	// MaxCells 限制字符网格的单元格总数（列数乘行数）。
	// 400 x 600 远超任何终端或预览窗格能够显示的规模。
	MaxCells = 400 * 600
)

// CheckOutputWidth 校验请求的字符宽度。
func CheckOutputWidth(w int) error {
	if w < MinOutputWidth || w > MaxOutputWidth {
		return fmt.Errorf("输出宽度必须在 %d..%d 之间，目前值为 %d", MinOutputWidth, MaxOutputWidth, w)
	}
	return nil
}

// CheckGrid 在生成字符网格之前校验它的最终尺寸。
func CheckGrid(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return fmt.Errorf("字符网格尺寸无效：%dx%d", cols, rows)
	}
	if int64(cols)*int64(rows) > MaxCells {
		return fmt.Errorf("字符网格过大：%dx%d 超过 %d 个单元格上限，请减小宽度", cols, rows, MaxCells)
	}
	return nil
}

// CheckPixelCount 校验从图像头部读到的尺寸，这一步发生在分配像素缓冲区之前。
func CheckPixelCount(w, h int) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("图像尺寸无效：%dx%d", w, h)
	}
	if w > MaxSide || h > MaxSide {
		return fmt.Errorf("图像边长 %dx%d 超过 %d 像素上限", w, h, MaxSide)
	}
	if int64(w)*int64(h) > MaxPixels {
		return fmt.Errorf("图像共 %d 像素，超过 %d 像素上限（可能是声明超大尺寸的小文件）",
			int64(w)*int64(h), MaxPixels)
	}
	return nil
}
