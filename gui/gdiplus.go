package gui

import (
	"syscall"
	"unsafe"
)

// GDI+ 扁平 API。这里只绑定程序界面需要用到的少数入口。
// 这些入口都不接收路径或网址，用户输入因此不会到达图像编解码相关接口。
type (
	GpGraphics     uintptr
	GpBrush        uintptr
	GpPen          uintptr
	GpPath         uintptr
	GpImage        uintptr
	GpBitmap       uintptr
	GpFont         uintptr
	GpFontFamily   uintptr
	GpStringFormat uintptr
)

type GpRectF struct{ X, Y, Width, Height float32 }

type GdiplusStartupInput struct {
	GdiplusVersion           uint32
	DebugEventCallback       uintptr
	SuppressBackgroundThread BOOL
	SuppressExternalCodecs   BOOL
}

var (
	gdiplus = syscall.NewLazyDLL("gdiplus.dll")

	procGdiplusStartup            = gdiplus.NewProc("GdiplusStartup")
	procGdiplusShutdown           = gdiplus.NewProc("GdiplusShutdown")
	procGdipCreateFromHDC         = gdiplus.NewProc("GdipCreateFromHDC")
	procGdipDeleteGraphics        = gdiplus.NewProc("GdipDeleteGraphics")
	procGdipGraphicsClear         = gdiplus.NewProc("GdipGraphicsClear")
	procGdipSetSmoothingMode      = gdiplus.NewProc("GdipSetSmoothingMode")
	procGdipSetTextRenderingHint  = gdiplus.NewProc("GdipSetTextRenderingHint")
	procGdipSetPixelOffsetMode    = gdiplus.NewProc("GdipSetPixelOffsetMode")
	procGdipSetInterpolationMode  = gdiplus.NewProc("GdipSetInterpolationMode")
	procGdipCreateSolidFill       = gdiplus.NewProc("GdipCreateSolidFill")
	procGdipDeleteBrush           = gdiplus.NewProc("GdipDeleteBrush")
	procGdipCreatePen1            = gdiplus.NewProc("GdipCreatePen1")
	procGdipDeletePen             = gdiplus.NewProc("GdipDeletePen")
	procGdipCreatePath            = gdiplus.NewProc("GdipCreatePath")
	procGdipAddPathArc            = gdiplus.NewProc("GdipAddPathArc")
	procGdipAddPathLine           = gdiplus.NewProc("GdipAddPathLine")
	procGdipClosePathFigure       = gdiplus.NewProc("GdipClosePathFigure")
	procGdipDeletePath            = gdiplus.NewProc("GdipDeletePath")
	procGdipFillPath              = gdiplus.NewProc("GdipFillPath")
	procGdipDrawPath              = gdiplus.NewProc("GdipDrawPath")
	procGdipFillRectangle         = gdiplus.NewProc("GdipFillRectangle")
	procGdipDrawLine              = gdiplus.NewProc("GdipDrawLine")
	procGdipDrawRectangle         = gdiplus.NewProc("GdipDrawRectangle")
	procGdipCreateFontFamilyName  = gdiplus.NewProc("GdipCreateFontFamilyFromName")
	procGdipDeleteFontFamily      = gdiplus.NewProc("GdipDeleteFontFamily")
	procGdipCreateFont            = gdiplus.NewProc("GdipCreateFont")
	procGdipDeleteFont            = gdiplus.NewProc("GdipDeleteFont")
	procGdipCreateStringFormat    = gdiplus.NewProc("GdipCreateStringFormat")
	procGdipDeleteStringFormat    = gdiplus.NewProc("GdipDeleteStringFormat")
	procGdipSetStringFormatAlign  = gdiplus.NewProc("GdipSetStringFormatAlign")
	procGdipSetStringFormatLine   = gdiplus.NewProc("GdipSetStringFormatLineAlign")
	procGdipSetStringFormatTrim   = gdiplus.NewProc("GdipSetStringFormatTrimming")
	procGdipSetStringFormatFlags  = gdiplus.NewProc("GdipSetStringFormatFlags")
	procGdipDrawString            = gdiplus.NewProc("GdipDrawString")
	procGdipMeasureString         = gdiplus.NewProc("GdipMeasureString")
	procGdipSetClipRect           = gdiplus.NewProc("GdipSetClipRect")
	procGdipResetClip             = gdiplus.NewProc("GdipResetClip")
	procGdipCreateBitmapFromScan0 = gdiplus.NewProc("GdipCreateBitmapFromScan0")
	procGdipDrawImageRect         = gdiplus.NewProc("GdipDrawImageRect")
	procGdipDisposeImage          = gdiplus.NewProc("GdipDisposeImage")
	procGdipGetImageWidth         = gdiplus.NewProc("GdipGetImageWidth")
	procGdipGetImageHeight        = gdiplus.NewProc("GdipGetImageHeight")
)

const (
	unitPixel       = 2
	fontStyleBold   = 1
	fontStyleItalic = 2

	sfNoWrap = 0x00001000

	alignNear   = 0
	alignCenter = 1
	alignFar    = 2

	trimmingNone = 0

	smoothingNone        = 0
	smoothingAntiAlias   = 4
	smoothingHighQuality = 4

	pixelOffsetHighQuality   = 2
	interpHighQualityBicubic = 7

	textHintAntiAliasGridFit = 3

	pixelFormat32bppPARGB = 0x000E200B

	fillModeWinding = 1
)

var gdiplusToken uintptr

// argb 把颜色打包成 GDI+ 需要的 0xAARRGGBB 格式。
func argb(r, g, b, a byte) uint32 {
	return uint32(a)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b)
}

func gpStartup() bool {
	if gdiplusToken != 0 {
		return true
	}
	in := GdiplusStartupInput{GdiplusVersion: 1}
	var token uintptr
	st, _, _ := procGdiplusStartup.Call(
		uintptr(unsafe.Pointer(&token)),
		uintptr(unsafe.Pointer(&in)),
		0,
	)
	if st != 0 {
		return false
	}
	gdiplusToken = token
	return true
}

func gpShutdown() {
	if gdiplusToken != 0 {
		procGdiplusShutdown.Call(gdiplusToken)
		gdiplusToken = 0
	}
}

type gfx struct {
	h GpGraphics
}

func newGraphics(hdc HDC) *gfx {
	var g GpGraphics
	if st, _, _ := procGdipCreateFromHDC.Call(uintptr(hdc), uintptr(unsafe.Pointer(&g))); st != 0 {
		return nil
	}
	x := &gfx{h: g}
	x.setSmoothing(smoothingAntiAlias)
	x.setTextHint(textHintAntiAliasGridFit)
	callOptional(procGdipSetPixelOffsetMode, uintptr(g), pixelOffsetHighQuality)
	return x
}

func (g *gfx) free() {
	if g != nil && g.h != 0 {
		procGdipDeleteGraphics.Call(uintptr(g.h))
		g.h = 0
	}
}

func (g *gfx) setSmoothing(mode uintptr) {
	callOptional(procGdipSetSmoothingMode, uintptr(g.h), mode)
}

func (g *gfx) setTextHint(hint uintptr) {
	callOptional(procGdipSetTextRenderingHint, uintptr(g.h), hint)
}

func (g *gfx) setInterp(mode uintptr) {
	callOptional(procGdipSetInterpolationMode, uintptr(g.h), mode)
}

func (g *gfx) clear(c uint32) {
	callOptional(procGdipGraphicsClear, uintptr(g.h), uintptr(c))
}

func (g *gfx) solidBrush(c uint32) GpBrush {
	var b GpBrush
	if st, _, _ := procGdipCreateSolidFill.Call(uintptr(c), uintptr(unsafe.Pointer(&b))); st != 0 {
		return 0
	}
	return b
}

func (g *gfx) pen(c uint32, w float32) GpPen {
	var p GpPen
	st, _, _ := procGdipCreatePen1.Call(
		uintptr(c),
		uintptr(float32bits(w)),
		unitPixel,
		uintptr(unsafe.Pointer(&p)),
	)
	if st != 0 {
		return 0
	}
	return p
}

func freeBrush(b GpBrush) {
	if b != 0 {
		procGdipDeleteBrush.Call(uintptr(b))
	}
}

func freePen(p GpPen) {
	if p != 0 {
		procGdipDeletePen.Call(uintptr(p))
	}
}

func float32bits(f float32) uint32 {
	return *(*uint32)(unsafe.Pointer(&f))
}

func f32(v float32) uintptr { return uintptr(float32bits(v)) }

// fillRect 填充一个轴对齐的矩形。
func (g *gfx) fillRect(r Rect, c uint32) {
	b := g.solidBrush(c)
	if b == 0 {
		return
	}
	callOptional(procGdipFillRectangle, uintptr(g.h), uintptr(b),
		f32(float32(r.Left)), f32(float32(r.Top)),
		f32(float32(r.Width())), f32(float32(r.Height())))
	freeBrush(b)
}

func (g *gfx) fillRectF(x, y, w, h float32, c uint32) {
	b := g.solidBrush(c)
	if b == 0 {
		return
	}
	callOptional(procGdipFillRectangle, uintptr(g.h), uintptr(b), f32(x), f32(y), f32(w), f32(h))
	freeBrush(b)
}

func (g *gfx) hline(x1, x2, y int32, c uint32, width float32) {
	p := g.pen(c, width)
	if p == 0 {
		return
	}
	callOptional(procGdipDrawLine, uintptr(g.h), uintptr(p),
		f32(float32(x1)), f32(float32(y)+width/2),
		f32(float32(x2)), f32(float32(y)+width/2))
	freePen(p)
}

// roundRectPath 构造一条闭合的圆角矩形路径。
func roundRectPath(x, y, w, h, r float32) GpPath {
	if r < 0 {
		r = 0
	}
	if r*2 > w {
		r = w / 2
	}
	if r*2 > h {
		r = h / 2
	}
	var p GpPath
	if st, _, _ := procGdipCreatePath.Call(fillModeWinding, uintptr(unsafe.Pointer(&p))); st != 0 {
		return 0
	}
	if r <= 0.01 {
		callOptional(procGdipAddPathLine, uintptr(p), f32(x), f32(y), f32(x+w), f32(y))
		callOptional(procGdipAddPathLine, uintptr(p), f32(x+w), f32(y), f32(x+w), f32(y+h))
		callOptional(procGdipAddPathLine, uintptr(p), f32(x+w), f32(y+h), f32(x), f32(y+h))
		callOptional(procGdipClosePathFigure, uintptr(p))
		return p
	}
	// 四段圆弧，从左上角起顺时针排列。
	callOptional(procGdipAddPathArc, uintptr(p), f32(x), f32(y), f32(2*r), f32(2*r), f32(180), f32(90))
	callOptional(procGdipAddPathArc, uintptr(p), f32(x+w-2*r), f32(y), f32(2*r), f32(2*r), f32(270), f32(90))
	callOptional(procGdipAddPathArc, uintptr(p), f32(x+w-2*r), f32(y+h-2*r), f32(2*r), f32(2*r), f32(0), f32(90))
	callOptional(procGdipAddPathArc, uintptr(p), f32(x), f32(y+h-2*r), f32(2*r), f32(2*r), f32(90), f32(90))
	callOptional(procGdipClosePathFigure, uintptr(p))
	return p
}

func freePath(p GpPath) {
	if p != 0 {
		procGdipDeletePath.Call(uintptr(p))
	}
}

func (g *gfx) fillRoundRect(r Rect, radius float32, c uint32) {
	p := roundRectPath(float32(r.Left), float32(r.Top), float32(r.Width()), float32(r.Height()), radius)
	if p == 0 {
		return
	}
	b := g.solidBrush(c)
	if b != 0 {
		callOptional(procGdipFillPath, uintptr(g.h), uintptr(b), uintptr(p))
		freeBrush(b)
	}
	freePath(p)
}

func (g *gfx) strokeRoundRect(r Rect, radius float32, c uint32, w float32) {
	p := roundRectPath(float32(r.Left), float32(r.Top), float32(r.Width()), float32(r.Height()), radius)
	if p == 0 {
		return
	}
	pen := g.pen(c, w)
	if pen != 0 {
		callOptional(procGdipDrawPath, uintptr(g.h), uintptr(pen), uintptr(p))
		freePen(pen)
	}
	freePath(p)
}

func (g *gfx) clipRect(r Rect) {
	callOptional(procGdipSetClipRect, uintptr(g.h),
		f32(float32(r.Left)), f32(float32(r.Top)),
		f32(float32(r.Width())), f32(float32(r.Height())), 0)
}

func (g *gfx) resetClip() {
	callOptional(procGdipResetClip, uintptr(g.h))
}

// font 封装 GDI+ 的字体族以及创建时的字号。
type font struct {
	h    GpFont
	fam  GpFontFamily
	size float32
	name string
}

func newFont(family string, sizePx float32, bold bool) *font {
	name := utf16Ptr(family)
	var fam GpFontFamily
	st, _, _ := procGdipCreateFontFamilyName.Call(
		uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&fam)))
	if st != 0 || fam == 0 {
		return nil
	}
	style := uintptr(0)
	if bold {
		style = fontStyleBold
	}
	var f GpFont
	st, _, _ = procGdipCreateFont.Call(
		uintptr(fam), f32(sizePx), style, unitPixel, uintptr(unsafe.Pointer(&f)))
	if st != 0 || f == 0 {
		procGdipDeleteFontFamily.Call(uintptr(fam))
		return nil
	}
	return &font{h: f, fam: fam, size: sizePx, name: family}
}

func (f *font) free() {
	if f == nil {
		return
	}
	if f.h != 0 {
		procGdipDeleteFont.Call(uintptr(f.h))
		f.h = 0
	}
	if f.fam != 0 {
		procGdipDeleteFontFamily.Call(uintptr(f.fam))
		f.fam = 0
	}
}

// measure 返回字符串 s 渲染后的尺寸。
func (g *gfx) measure(f *font, s string) (w, h float32) {
	if f == nil || s == "" {
		return 0, 0
	}
	txt, err := syscall.UTF16FromString(s)
	if err != nil {
		return 0, 0
	}
	layout := GpRectF{X: 0, Y: 0, Width: 10000, Height: 10000}
	var box GpRectF
	st, _, _ := procGdipMeasureString.Call(
		uintptr(g.h),
		uintptr(unsafe.Pointer(&txt[0])),
		uintptr(int32(len(txt)-1)),
		uintptr(f.h),
		uintptr(unsafe.Pointer(&layout)),
		0,
		uintptr(unsafe.Pointer(&box)),
		0, 0,
	)
	if st != 0 {
		return 0, 0
	}
	return box.Width, box.Height
}

// drawText 按指定的对齐方式把字符串 s 渲染到矩形 r 内。
func (g *gfx) drawText(f *font, s string, r Rect, c uint32, hAlign int32, vCenter bool) {
	if f == nil || s == "" {
		return
	}
	txt, err := syscall.UTF16FromString(s)
	if err != nil || len(txt) == 0 {
		return
	}
	var format GpStringFormat
	if st, _, _ := procGdipCreateStringFormat.Call(0, 0, uintptr(unsafe.Pointer(&format))); st != 0 {
		return
	}
	defer procGdipDeleteStringFormat.Call(uintptr(format))
	callOptional(procGdipSetStringFormatFlags, uintptr(format), sfNoWrap)
	callOptional(procGdipSetStringFormatTrim, uintptr(format), trimmingNone)
	callOptional(procGdipSetStringFormatAlign, uintptr(format), uintptr(hAlign))
	lineAlign := uintptr(alignNear)
	if vCenter {
		lineAlign = alignCenter
	}
	callOptional(procGdipSetStringFormatLine, uintptr(format), lineAlign)

	layout := GpRectF{
		X:      float32(r.Left),
		Y:      float32(r.Top),
		Width:  float32(r.Width()),
		Height: float32(r.Height()),
	}
	b := g.solidBrush(c)
	if b == 0 {
		return
	}
	defer freeBrush(b)
	procGdipDrawString.Call(
		uintptr(g.h),
		uintptr(unsafe.Pointer(&txt[0])),
		uintptr(int32(len(txt)-1)),
		uintptr(f.h),
		uintptr(unsafe.Pointer(&layout)),
		uintptr(format),
		uintptr(b),
	)
}

// bitmapFromRGBA 把预乘 RGBA 缓冲区封装成 GDI+ 位图。
// 像素会被复制到由 Windows 管理的 DIB 节中，GDI+ 因此不会持有指向 Go 堆的指针。
// 如果 Go 指针被 Windows 原生代码保留会发生生命周期错误，导致释放后使用问题，必须避免。
func bitmapFromRGBA(pix []byte, w, h int) (GpBitmap, HBITMAP) {
	if w <= 0 || h <= 0 || len(pix) < w*h*4 {
		return 0, 0
	}
	bi := BitmapInfo{}
	bi.Header.BiSize = DWORD(unsafe.Sizeof(bi.Header))
	bi.Header.BiWidth = int32(w)
	bi.Header.BiHeight = -int32(h) // 负数高度表示行序自上而下。
	bi.Header.BiPlanes = 1
	bi.Header.BiBitCount = 32
	bi.Header.BiCompression = biRGB

	var bits unsafe.Pointer
	hbm, _, _ := procCreateDIBSection.Call(
		0,
		uintptr(unsafe.Pointer(&bi)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&bits)),
		0, 0,
	)
	if hbm == 0 || bits == nil {
		return 0, 0
	}

	// DIB 的像素格式是 BGRA，传入的缓冲区是预乘 RGBA，因此交换两个颜色字节。
	dst := unsafe.Slice((*byte)(bits), w*h*4)
	for i := 0; i < w*h; i++ {
		dst[i*4+0] = pix[i*4+2]
		dst[i*4+1] = pix[i*4+1]
		dst[i*4+2] = pix[i*4+0]
		dst[i*4+3] = pix[i*4+3]
	}

	var bmp GpBitmap
	st, _, _ := procGdipCreateBitmapFromScan0.Call(
		uintptr(int32(w)), uintptr(int32(h)), uintptr(int32(w*4)),
		pixelFormat32bppPARGB, uintptr(bits), uintptr(unsafe.Pointer(&bmp)))
	if st != 0 || bmp == 0 {
		procDeleteObject.Call(hbm)
		return 0, 0
	}
	return bmp, HBITMAP(hbm)
}

func (g *gfx) drawImage(img GpBitmap, r Rect) {
	if img == 0 {
		return
	}
	callOptional(procGdipDrawImageRect, uintptr(g.h), uintptr(img),
		f32(float32(r.Left)), f32(float32(r.Top)),
		f32(float32(r.Width())), f32(float32(r.Height())))
}

func freeImage(img GpBitmap) {
	if img != 0 {
		procGdipDisposeImage.Call(uintptr(img))
	}
}
