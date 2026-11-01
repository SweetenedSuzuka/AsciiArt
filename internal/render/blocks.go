package render

import (
	"asciiart/internal/limits"
	"asciiart/internal/safedecode"
)

// renderBlocks 生成半色块字符画。
// 每个单元格用“▀”绘制，上半部分和下半部分各用一种颜色，垂直方向的分辨率因此提高一倍。
func renderBlocks(img *safedecode.Image, o Options) (*Art, error) {
	cols := o.Width
	ph := int(float64(cols) * float64(img.Height) / float64(img.Width) * o.Scale)
	if ph%2 != 0 {
		ph++
	}
	if ph < 2 {
		ph = 2
	}
	rows := ph / 2
	if err := limits.CheckGrid(cols, rows); err != nil {
		return nil, err
	}

	rgb, alpha := compositeOver(img, o.Background)
	rgb = resizeLanczos(rgb, img.Width, img.Height, 3, cols, ph)
	mask := resizeNearest(alpha, img.Width, img.Height, cols, ph)

	cells := make([]Cell, cols*rows)
	for y := 0; y < rows; y++ {
		top := (y * 2) * cols
		bot := (y*2 + 1) * cols
		for x := 0; x < cols; x++ {
			// 单元格的上半部分和下半部分都必须不透明。
			// 半透明的单元格会把背景色画进图像的轮廓里。
			if mask[top+x] <= o.AlphaThreshold || mask[bot+x] <= o.AlphaThreshold {
				continue
			}
			cells[y*cols+x] = Cell{
				Ch:    '▀',
				FG:    rgbAt(rgb, cols, x, y*2),
				BG:    rgbAt(rgb, cols, x, y*2+1),
				HasFG: true,
				HasBG: true,
			}
		}
	}
	return &Art{Cols: cols, Rows: rows, Cells: cells}, nil
}
