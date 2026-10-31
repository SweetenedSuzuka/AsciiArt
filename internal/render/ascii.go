package render

import (
	"asciiart/internal/limits"
	"asciiart/internal/safedecode"
)

// renderASCII 把亮度映射到字符集上。
// 每个字符位对应一个 1x2 的像素块，所以垂直方向的分辨率减半。
// 终端字符位的高度大约是宽度的两倍，高度减半以后就还原了源图像的宽高比。
func renderASCII(img *safedecode.Image, o Options) (*Art, error) {
	cols := o.Width
	rows := int(float64(cols) * float64(img.Height) / float64(img.Width) * 0.5)
	if rows < 1 {
		rows = 1
	}
	if err := limits.CheckGrid(cols, rows); err != nil {
		return nil, err
	}

	rgb, alpha := compositeOver(img, o.Background)
	rgb = resizeLanczos(rgb, img.Width, img.Height, 3, cols, rows)
	mask := resizeNearest(alpha, img.Width, img.Height, cols, rows)

	charset := []rune(o.Charset)
	last := len(charset) - 1

	cells := make([]Cell, cols*rows)
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			i := y*cols + x
			// 透明区域留空不绘制，这样抠图 PNG 能保住轮廓。
			if mask[i] <= o.AlphaThreshold {
				continue
			}
			c := rgbAt(rgb, cols, x, y)
			lum := 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
			if o.Invert {
				lum = 255 - lum
			}
			idx := int(lum / 255 * float64(last))
			if idx < 0 {
				idx = 0
			} else if idx > last {
				idx = last
			}
			cells[i] = Cell{Ch: charset[idx], FG: c, HasFG: true}
		}
	}
	return &Art{Cols: cols, Rows: rows, Cells: cells}, nil
}
