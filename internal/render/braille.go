package render

import (
	"math"

	"asciiart/internal/limits"
	"asciiart/internal/safedecode"
)

// brailleDots 把 Unicode 盲文的位编号映射到它在 2x4 字符位内的位置，格式为（行, 列）。
// 这份映射由 Unicode 盲文模式区块固定：第 1 到 4 点是左列从上到下，第 5 到 8 点是右列。
// 顺序写错以后，生成的字符看起来像合法盲文，编码的内容却是错的，所以这里逐项写出完整的对应关系。
var brailleDots = [8][2]int{
	{0, 0}, {1, 0}, {2, 0}, {3, 0}, // 第 0-3 位对应第 1-4 点，位于左列。
	{0, 1}, {1, 1}, {2, 1}, {3, 1}, // 第 4-7 位对应第 5-8 点，位于右列。
}

// renderBraille 生成盲文点阵字符画。
// 每个盲文字符覆盖 2x4 个像素。
// 是否绘制圆点由局部自适应阈值决定，阈值为局部均值减去 k 倍标准差。
// 这样线条内容在浅色和深色背景上都清晰可读，不会糊成一整块。
func renderBraille(img *safedecode.Image, o Options) (*Art, error) {
	pw := o.Width * 2
	ph := int(float64(pw) * float64(img.Height) / float64(img.Width))
	ph = ((ph + 3) / 4) * 4
	if ph < 4 {
		ph = 4
	}
	cols, rows := pw/2, ph/4
	if err := limits.CheckGrid(cols, rows); err != nil {
		return nil, err
	}

	rgb, alpha := compositeOver(img, o.Background)

	// 边缘增强只有在源图像分辨率远高于输出分辨率时才有意义，而在四千万像素的数据上直接处理会超出时间和内存的上限。
	// 这里最多以目标分辨率的 2 倍进行处理，并按同样的比例放大卷积核。
	// 低于 2 倍时，差别看不出来。
	fx, fy := 1, 1
	if img.Width > pw*2 {
		fx = img.Width / (pw * 2)
	}
	if img.Height > ph*2 {
		fy = img.Height / (ph * 2)
	}
	reduced, rw, rh := boxReduce(rgb, img.Width, img.Height, 3, fx, fy)
	effectScale := float64(img.Width) / float64(rw)

	gray := toGray(reduced, rw, rh)
	gray = unsharpMask(gray, rw, rh, o.Sharpen, effectScale)
	gray = adjustContrast(gray, o.Contrast)
	gray = resizePlaneF(gray, rw, rh, pw, ph)

	// 自适应阈值：像素比局部均值减去 k 倍标准差的结果更暗时，判定为需要绘制的像素。
	mean := gaussianBlur(gray, pw, ph, o.Sigma)
	dev := make([]float32, len(gray))
	for i := range gray {
		d := float64(gray[i]) - float64(mean[i])
		dev[i] = float32(d * d)
	}
	variance := gaussianBlur(dev, pw, ph, o.Sigma)

	rgb = resizeLanczos(rgb, img.Width, img.Height, 3, pw, ph)
	mask := resizeNearest(alpha, img.Width, img.Height, pw, ph)

	cells := make([]Cell, cols*rows)
	for cy := 0; cy < rows; cy++ {
		for cx := 0; cx < cols; cx++ {
			y0, x0 := cy*4, cx*2

			// 一个字符位至少要有一半的像素不透明才绘制，否则抠图的边缘会变成噪点。
			opaque := 0
			var sumR, sumG, sumB, n int
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 2; dx++ {
					i := (y0+dy)*pw + x0 + dx
					if mask[i] > o.AlphaThreshold {
						opaque++
						c := rgbAt(rgb, pw, x0+dx, y0+dy)
						sumR += int(c.R)
						sumG += int(c.G)
						sumB += int(c.B)
						n++
					}
				}
			}
			if opaque < 4 || n == 0 {
				continue
			}

			bitmask := 0
			for bit, d := range brailleDots {
				i := (y0+d[0])*pw + x0 + d[1]
				if mask[i] <= o.AlphaThreshold {
					continue
				}
				thr := float64(mean[i]) - o.ThresholdOffset*math.Sqrt(float64(variance[i]))
				if float64(gray[i]) < thr {
					bitmask |= 1 << uint(bit)
				}
			}
			if bitmask == 0 {
				continue
			}

			cells[cy*cols+cx] = Cell{
				Ch:    rune(0x2800 + bitmask),
				FG:    RGB{uint8(sumR / n), uint8(sumG / n), uint8(sumB / n)},
				HasFG: true,
			}
		}
	}
	return &Art{Cols: cols, Rows: rows, Cells: cells}, nil
}
