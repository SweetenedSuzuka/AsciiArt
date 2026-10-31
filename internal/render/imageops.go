package render

import (
	"math"

	"asciiart/internal/safedecode"
)

// compositeOver 把预乘 RGBA 图像压平到不透明的背景色上。
// 函数返回颜色平面（每个像素 3 个字节）和原始透明度平面（每个像素 1 个字节）。
// 因为源图像是预乘的，压平每个通道只需要一次相加：结果 = 预乘值 + 背景色 × (1 - 透明度)。
// 整个过程没有除法，全透明的像素不会产生 NaN 或者错误的颜色。
func compositeOver(img *safedecode.Image, bg RGB) (rgb []byte, alpha []byte) {
	n := img.Width * img.Height
	rgb = make([]byte, n*3)
	alpha = make([]byte, n)
	bgR := int(bg.R)
	bgG := int(bg.G)
	bgB := int(bg.B)

	for i := 0; i < n; i++ {
		p := i * 4
		a := int(img.Pix[p+3])
		alpha[i] = uint8(a)

		var r, g, b int
		if a == 255 {
			r, g, b = int(img.Pix[p]), int(img.Pix[p+1]), int(img.Pix[p+2])
		} else if a == 0 {
			r, g, b = bgR, bgG, bgB
		} else {
			inv := 255 - a
			r = int(img.Pix[p]) + (bgR*inv+127)/255
			g = int(img.Pix[p+1]) + (bgG*inv+127)/255
			b = int(img.Pix[p+2]) + (bgB*inv+127)/255
		}
		rgb[i*3] = uint8(min(r, 255))
		rgb[i*3+1] = uint8(min(g, 255))
		rgb[i*3+2] = uint8(min(b, 255))
	}
	return rgb, alpha
}

// toGray 把 RGB 平面转换成单通道的灰度平面。
// 加权系数使用 ITU-R BT.601 标准，这组系数对应人眼对红、绿、蓝三种颜色的敏感程度。
func toGray(rgb []byte, w, h int) []float32 {
	out := make([]float32, w*h)
	for i := 0; i < w*h; i++ {
		r := float32(rgb[i*3])
		g := float32(rgb[i*3+1])
		b := float32(rgb[i*3+2])
		out[i] = 0.299*r + 0.587*g + 0.114*b
	}
	return out
}

// rgbAt 返回 RGB 平面里像素 (x, y) 的颜色。
func rgbAt(rgb []byte, w, x, y int) RGB {
	i := (y*w + x) * 3
	return RGB{rgb[i], rgb[i+1], rgb[i+2]}
}

// ------------------------------------------------------------------- 重采样

// lanczos3 是用于缩小图像的加窗 sinc 核函数。
func lanczos3(x float64) float64 {
	if x < 0 {
		x = -x
	}
	if x < 1e-9 {
		return 1
	}
	if x >= 3 {
		return 0
	}
	px := math.Pi * x
	return 3 * math.Sin(px) * math.Sin(px/3) / (px * px)
}

type taps struct {
	start int
	w     []float64
}

// buildTaps 预先计算某一个方向的滤波权重。
// 采样窗口随缩小倍数一起放大。
// 把 4000 像素宽的源图像缩小到 80 像素时，仍然是对整个覆盖范围取平均，而不是只采样零星几个像素。
func buildTaps(src, dst int) []taps {
	scale := float64(src) / float64(dst)
	filterScale := scale
	if filterScale < 1 {
		filterScale = 1
	}
	support := 3 * filterScale

	out := make([]taps, dst)
	for d := 0; d < dst; d++ {
		center := (float64(d)+0.5)*scale - 0.5
		lo := int(math.Ceil(center - support))
		hi := int(math.Floor(center + support))
		if lo < 0 {
			lo = 0
		}
		if hi > src-1 {
			hi = src - 1
		}
		if hi < lo {
			hi = lo
		}
		weights := make([]float64, hi-lo+1)
		sum := 0.0
		for i := lo; i <= hi; i++ {
			w := lanczos3((float64(i) - center) / filterScale)
			weights[i-lo] = w
			sum += w
		}
		if sum != 0 {
			for i := range weights {
				weights[i] /= sum
			}
		} else {
			weights[0] = 1
		}
		out[d] = taps{start: lo, w: weights}
	}
	return out
}

// boxReduce 用面积平均的方式把平面按整数倍缩小。
// 它在 Lanczos 重采样之前运行，滤波器因此不必直接处理四千万像素的数据。
// 内存和时间的开销与输出大小成正比，与用户打开的源图像大小无关。
// 先取平均也能避免大幅度缩小时出现混叠。
func boxReduce(src []byte, sw, sh, ch, fx, fy int) ([]byte, int, int) {
	if fx <= 1 && fy <= 1 {
		return src, sw, sh
	}
	if fx < 1 {
		fx = 1
	}
	if fy < 1 {
		fy = 1
	}
	dw := sw / fx
	dh := sh / fy
	if dw < 1 {
		dw, fx = 1, 1
	}
	if dh < 1 {
		dh, fy = 1, 1
	}
	if fx == 1 && fy == 1 {
		return src, sw, sh
	}

	dst := make([]byte, dw*dh*ch)
	inv := 1.0 / float64(fx*fy)
	for dy := 0; dy < dh; dy++ {
		for dx := 0; dx < dw; dx++ {
			for c := 0; c < ch; c++ {
				sum := 0
				for yy := 0; yy < fy; yy++ {
					row := ((dy*fy+yy)*sw + dx*fx) * ch
					for xx := 0; xx < fx; xx++ {
						sum += int(src[row+xx*ch+c])
					}
				}
				v := float64(sum)*inv + 0.5
				if v > 255 {
					v = 255
				}
				dst[(dy*dw+dx)*ch+c] = byte(v)
			}
		}
	}
	return dst, dw, dh
}

// resizeLanczos 缩放包含 ch 个通道的交错字节平面。
func resizeLanczos(src []byte, sw, sh, ch, dw, dh int) []byte {
	if sw == dw && sh == dh {
		return src
	}
	// 限制处理的数据量：每个方向上最多对目标大小的约 2 倍做滤波。
	// 超过这个范围的部分对结果没有可见影响。
	fx, fy := 1, 1
	if dw > 0 && sw > dw*2 {
		fx = sw / (dw * 2)
	}
	if dh > 0 && sh > dh*2 {
		fy = sh / (dh * 2)
	}
	src, sw, sh = boxReduce(src, sw, sh, ch, fx, fy)
	if sw == dw && sh == dh {
		return src
	}

	xw := buildTaps(sw, dw)
	yw := buildTaps(sh, dh)

	// 水平方向的一遍处理，结果写入 float32 中间缓存。
	horiz := make([]float32, dw*sh*ch)
	for y := 0; y < sh; y++ {
		srow := y * sw * ch
		trow := y * dw * ch
		for d := 0; d < dw; d++ {
			t := xw[d]
			base := srow + t.start*ch
			for c := 0; c < ch; c++ {
				acc := 0.0
				for k, w := range t.w {
					acc += float64(src[base+k*ch+c]) * w
				}
				horiz[trow+d*ch+c] = float32(acc)
			}
		}
	}

	// 垂直方向的一遍处理，结果直接写入目标平面。
	dst := make([]byte, dw*dh*ch)
	for d := 0; d < dh; d++ {
		t := yw[d]
		for x := 0; x < dw; x++ {
			for c := 0; c < ch; c++ {
				acc := 0.0
				for k, w := range t.w {
					acc += float64(horiz[((t.start+k)*dw+x)*ch+c]) * w
				}
				if acc < 0 {
					acc = 0
				} else if acc > 255 {
					acc = 255
				}
				dst[(d*dw+x)*ch+c] = byte(acc + 0.5)
			}
		}
	}
	return dst
}

// resizeNearest 用点采样的方式缩小单通道平面。
// 透明度掩码必须保持二值，所以有意不做滤波。
// 滤波会让掩码出现半透明的取值，图像的轮廓也会变模糊。
func resizeNearest(src []byte, sw, sh, dw, dh int) []byte {
	if sw == dw && sh == dh {
		return src
	}
	dst := make([]byte, dw*dh)
	xs := make([]int, dw)
	for x := 0; x < dw; x++ {
		v := int((float64(x) + 0.5) * float64(sw) / float64(dw))
		if v > sw-1 {
			v = sw - 1
		}
		xs[x] = v
	}
	for y := 0; y < dh; y++ {
		sy := int((float64(y) + 0.5) * float64(sh) / float64(dh))
		if sy > sh-1 {
			sy = sh - 1
		}
		row := sy * sw
		drow := y * dw
		for x := 0; x < dw; x++ {
			dst[drow+x] = src[row+xs[x]]
		}
	}
	return dst
}

// --------------------------------------------------------------- 逐像素效果

// gaussianBlur 对 float32 平面做可分离的高斯模糊。
// 这里使用高斯核本身，不使用近似的盒式模糊，因为后面的自适应阈值依赖准确的局部均值和标准差。
func gaussianBlur(src []float32, w, h int, sigma float64) []float32 {
	if sigma <= 0 {
		return src
	}
	radius := int(4*sigma + 0.5)
	if radius < 1 {
		radius = 1
	}
	if radius > 64 {
		radius = 64
	}
	kernel := make([]float64, radius*2+1)
	sum := 0.0
	for i := -radius; i <= radius; i++ {
		v := math.Exp(-float64(i*i) / (2 * sigma * sigma))
		kernel[i+radius] = v
		sum += v
	}
	for i := range kernel {
		kernel[i] /= sum
	}

	horiz := make([]float32, w*h)
	for y := 0; y < h; y++ {
		row := y * w
		for x := 0; x < w; x++ {
			acc := 0.0
			for k := -radius; k <= radius; k++ {
				xx := x + k
				if xx < 0 {
					xx = 0
				} else if xx >= w {
					xx = w - 1
				}
				acc += float64(src[row+xx]) * kernel[k+radius]
			}
			horiz[row+x] = float32(acc)
		}
	}

	dst := make([]float32, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			acc := 0.0
			for k := -radius; k <= radius; k++ {
				yy := y + k
				if yy < 0 {
					yy = 0
				} else if yy >= h {
					yy = h - 1
				}
				acc += float64(horiz[yy*w+x]) * kernel[k+radius]
			}
			dst[y*w+x] = float32(acc)
		}
	}
	return dst
}

// unsharpMask 对平面做锐化：结果 = 源 + (源 - 模糊结果) × (强度 - 1)。
// blurSigma 以平面自身的像素为单位。
// 调用方如果事先缩小过图像，要按相同的比例传入更大的 sigma，让锐化半径折算到输出像素以后保持设定的大小。
func unsharpMask(src []float32, w, h int, strength, blurSigma float64) []float32 {
	if strength <= 1.0 {
		return src
	}
	amount := strength - 1.0
	blur := gaussianBlur(src, w, h, blurSigma)
	out := make([]float32, len(src))
	for i := range src {
		v := float64(src[i]) + (float64(src[i])-float64(blur[i]))*amount
		out[i] = float32(clampF(v, 0, 255))
	}
	return out
}

// adjustContrast 以平面自身的均值为中心拉伸对比度。
// 每个数值都离均值更远，暗部更暗、亮部更亮，整体亮度不发生变化。
func adjustContrast(src []float32, factor float64) []float32 {
	if factor == 1.0 || len(src) == 0 {
		return src
	}
	mean := 0.0
	for _, v := range src {
		mean += float64(v)
	}
	mean /= float64(len(src))
	out := make([]float32, len(src))
	for i, v := range src {
		out[i] = float32(clampF(mean+(float64(v)-mean)*factor, 0, 255))
	}
	return out
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// resizePlaneF 用与颜色平面相同的 Lanczos 滤波器缩放单通道 float32 平面，灰度和颜色因此保持对齐。
func resizePlaneF(src []float32, sw, sh, dw, dh int) []float32 {
	if sw == dw && sh == dh {
		return src
	}
	xw := buildTaps(sw, dw)
	yw := buildTaps(sh, dh)

	horiz := make([]float32, dw*sh)
	for y := 0; y < sh; y++ {
		row := y * sw
		trow := y * dw
		for d := 0; d < dw; d++ {
			t := xw[d]
			acc := 0.0
			for k, w := range t.w {
				acc += float64(src[row+t.start+k]) * w
			}
			horiz[trow+d] = float32(acc)
		}
	}

	dst := make([]float32, dw*dh)
	for d := 0; d < dh; d++ {
		t := yw[d]
		for x := 0; x < dw; x++ {
			acc := 0.0
			for k, w := range t.w {
				acc += float64(horiz[(t.start+k)*dw+x]) * w
			}
			dst[d*dw+x] = float32(clampF(acc, 0, 255))
		}
	}
	return dst
}
