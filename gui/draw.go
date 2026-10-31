package gui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unsafe"

	"asciiart/internal/render"
)

// 宽度滑块的范围。渲染器接受 1 到 400，但小于 20 时预览无法阅读，大于 240 时在多数窗口中单元格小于一个像素。
const (
	sliderMinWidth = 20
	sliderMaxWidth = 240
)

// 默认设计常量，以 96 DPI 下的像素表示，由主题负责缩放。
const (
	sidebarW    = 280
	marginX     = 20
	headerH     = 74
	btnH        = 34
	btnHPrimary = 38
	radius      = 4
	radiusCard  = 6
)

// 缩放与字号范围，以 baseFontSize 的百分比表示。
const (
	baseFontSize = 13.0 // 100% 缩放下的设计像素
	minArtFont   = 4.0
	maxArtFont   = 240.0
	minZoom      = 0.10
	maxZoom      = 8.00
)

type layout struct {
	client          Rect
	sidebar         Rect
	previewBox      Rect
	base            int32 // 设置区顶部的纵坐标，位于预览框下方
	modeSeg         Rect
	widthSlider     Rect
	thresholdSlider Rect
	colorCheck      Rect
	invertCheck     Rect
	openBtn         Rect
	exportBtn       Rect
	copyBtn         Rect
	card            Rect
	canvas          Rect
	zoomGroup       Rect
	zoomOut         Rect
	zoomValue       Rect
	zoomIn          Rect
	zoomFit         Rect
	zoom100         Rect
}

func inset(r Rect, d int32) Rect {
	return Rect{r.Left + d, r.Top + d, r.Right - d, r.Bottom - d}
}

// 布局始终从当前的客户区矩形计算。窗口较矮时侧栏也必须保持可用，因此预览框使用剩余的纵向空间。
func (a *App) layout() layout {
	c := a.clientRect()
	s := a.theme.scale
	px := func(v float32) int32 { return int32(v*s + 0.5) }

	sideW := px(sidebarW)
	l := layout{client: c}
	l.sidebar = Rect{0, 0, sideW, c.Height()}

	x := px(18)
	w := sideW - 2*px(18)

	boxH := c.Height() - px(372)
	if boxH < px(36) {
		boxH = px(36)
	}
	if boxH > px(200) {
		boxH = px(200)
	}
	l.previewBox = Rect{x, px(92), x + w, px(92) + boxH}
	l.base = px(92) + boxH + px(18)

	l.modeSeg = Rect{x, l.base + px(20), x + w, l.base + px(20) + px(32)}
	l.widthSlider = Rect{x, l.base + px(80), x + w, l.base + px(80) + px(22)}
	l.thresholdSlider = Rect{x, l.base + px(130), x + w, l.base + px(130) + px(22)}
	l.colorCheck = Rect{x, l.base + px(162), x + w/2, l.base + px(162) + px(20)}
	l.invertCheck = Rect{x + w/2, l.base + px(162), x + w, l.base + px(162) + px(20)}
	l.openBtn = Rect{x, c.Height() - px(18) - px(btnHPrimary), x + w, c.Height() - px(18)}

	// 主栏目。
	mx := sideW + px(24)
	l.card = Rect{mx, px(headerH), c.Right - px(marginX), c.Bottom - px(marginX)}
	l.canvas = inset(l.card, px(1)+px(10))

	exportW := px(104)
	l.exportBtn = Rect{c.Right - px(marginX) - exportW, px(18), c.Right - px(marginX), px(18 + btnH)}
	copyW := px(72)
	l.copyBtn = Rect{l.exportBtn.Left - px(10) - copyW, px(18), l.exportBtn.Left - px(10), px(18 + btnH)}

	// 缩放控件位于标题栏中、复制按钮与导出按钮的左侧。
	zoomW := px(210)
	zRight := l.copyBtn.Left - px(10)
	l.zoomGroup = Rect{zRight - zoomW, px(18), zRight, px(18 + btnH)}

	zoomInner := Rect{l.zoomGroup.Left + 1, l.zoomGroup.Top + 1, l.zoomGroup.Right - 1, l.zoomGroup.Bottom - 1}
	cx := zoomInner.Left
	l.zoomOut = Rect{cx, zoomInner.Top, cx + px(26), zoomInner.Bottom}
	cx += px(26)
	l.zoomValue = Rect{cx, zoomInner.Top, cx + px(54), zoomInner.Bottom}
	cx += px(54)
	l.zoomIn = Rect{cx, zoomInner.Top, cx + px(26), zoomInner.Bottom}
	cx += px(26)
	l.zoomFit = Rect{cx, zoomInner.Top, cx + px(52), zoomInner.Bottom}
	cx += px(52)
	l.zoom100 = Rect{cx, zoomInner.Top, zoomInner.Right, zoomInner.Bottom}

	return l
}

// ------------------------------------------------------------------ 背景缓冲区

func (a *App) ensureBackBuffer(w, h int32) bool {
	if w <= 0 || h <= 0 {
		return false
	}
	if a.memDC != 0 && a.bufW == w && a.bufH == h {
		return true
	}
	a.freeBackBuffer()

	bi := BitmapInfo{}
	bi.Header.BiSize = DWORD(unsafe.Sizeof(bi.Header))
	bi.Header.BiWidth = w
	bi.Header.BiHeight = -h
	bi.Header.BiPlanes = 1
	bi.Header.BiBitCount = 32
	bi.Header.BiCompression = biRGB

	var bits unsafe.Pointer
	hbm, _, _ := procCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&bi)), dibRGBColors,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbm == 0 {
		return false
	}
	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		procDeleteObject.Call(hbm)
		return false
	}
	procSelectObject.Call(dc, hbm)

	a.memDC = HDC(dc)
	a.memBMP = HBITMAP(hbm)
	a.memBits = bits
	a.bufW, a.bufH = w, h
	return true
}

func (a *App) freeBackBuffer() {
	if a.memDC != 0 {
		procDeleteDC.Call(uintptr(a.memDC))
		a.memDC = 0
	}
	if a.memBMP != 0 {
		procDeleteObject.Call(uintptr(a.memBMP))
		a.memBMP = 0
	}
	a.memBits = nil
	a.bufW, a.bufH = 0, 0
}

// ---------------------------------------------------------------------- 绘制

func (a *App) paint(hwnd HWND) {
	var ps PaintStruct
	hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))

	c := a.clientRect()
	if !a.ensureBackBuffer(c.Width(), c.Height()) {
		return
	}

	a.ensureArtFont()

	g := newGraphics(a.memDC)
	if g == nil {
		return
	}
	g.setSmoothing(smoothingAntiAlias)
	g.setTextHint(textHintAntiAliasGridFit)

	a.paintWidgets(g)
	g.free()

	// 字符画网格使用 GDI 绘制。
	// ExtTextOut 处理大量单个字符的速度远快于 GDI+，ETO_OPAQUE 使每个半块单元格在同一次调用中得到各自的背景色。
	a.drawArtText(a.memDC)

	// 滚动条在网格之后绘制，使字符画占满整个画布时滚动条仍然可见。
	if over := newGraphics(a.memDC); over != nil {
		over.setSmoothing(smoothingAntiAlias)
		over.setTextHint(textHintAntiAliasGridFit)
		a.drawScrollbars(over, a.layout())
		over.free()
	}

	procBitBlt.Call(hdc, 0, 0, uintptr(c.Width()), uintptr(c.Height()),
		uintptr(a.memDC), 0, 0, srcCopy)
}

func (a *App) paintWidgets(g *gfx) {
	pal := a.theme.pal
	l := a.layout()

	g.clear(pal.window)

	// 先绘制各个表面，使控件的绘制始终位于表层。
	g.fillRect(l.sidebar, pal.view)
	g.fillRectF(float32(l.sidebar.Right-1), 0, 1, float32(l.sidebar.Height()), pal.divider)
	g.fillRoundRect(l.card, a.theme.fpx(radiusCard), pal.card)
	g.strokeRoundRect(l.card, a.theme.fpx(radiusCard), pal.border, 1)
	g.fillRoundRect(l.canvas, a.theme.fpx(radius), pal.canvas)

	a.drawSidebar(g, l)
	a.drawHeader(g, l)
	a.drawZoomBar(g, l)
}

func (a *App) drawSidebar(g *gfx, l layout) {
	pal := a.theme.pal
	x := l.previewBox.Left
	w := l.previewBox.Width()
	b := l.base

	g.drawText(a.theme.uiTitle, "AsciiArt", Rect{x, a.theme.px(12), x + w, a.theme.px(42)}, pal.text, alignNear, false)
	g.drawText(a.theme.uiSmall, "图片转字符画", Rect{x, a.theme.px(44), x + w, a.theme.px(60)}, pal.textDim, alignNear, false)

	g.hline(x, x+w, a.theme.px(68), pal.divider, 1)
	g.drawText(a.theme.uiSmall, "原图", Rect{x, a.theme.px(74), x + w, a.theme.px(88)}, pal.textDim, alignNear, false)

	g.fillRoundRect(l.previewBox, a.theme.fpx(radius), pal.cardInset)
	g.strokeRoundRect(l.previewBox, a.theme.fpx(radius), pal.border, 1)

	if a.imgBmp != 0 && a.img != nil {
		box := inset(l.previewBox, a.theme.px(6))
		iw := float32(a.img.Width)
		ih := float32(a.img.Height)
		if iw > 0 && ih > 0 {
			s := math.Min(float64(box.Width())/float64(iw), float64(box.Height())/float64(ih))
			dw := float32(float64(iw) * s)
			dh := float32(float64(ih) * s)
			dx := float32(box.Left) + (float32(box.Width())-dw)/2
			dy := float32(box.Top) + (float32(box.Height())-dh)/2
			g.setInterp(interpHighQualityBicubic)
			g.drawImage(a.imgBmp, Rect{
				int32(dx), int32(dy), int32(dx + dw), int32(dy + dh),
			})
		}
	} else {
		g.drawText(a.theme.uiSmall, "尚未打开图片", l.previewBox, pal.textDim, alignCenter, true)
	}

	g.drawText(a.theme.uiSmall, "模式", Rect{x, b, x + w, b + a.theme.px(16)}, pal.textDim, alignNear, false)
	a.drawSegmented(g, l.modeSeg)

	g.drawText(a.theme.uiSmall, "宽度", Rect{x, b + a.theme.px(62), x + w/2, b + a.theme.px(78)}, pal.textDim, alignNear, false)
	g.drawText(a.theme.uiBold, fmt.Sprintf("%d", a.opts.Width),
		Rect{x + w/2, b + a.theme.px(62), x + w, b + a.theme.px(78)}, pal.text, alignFar, false)
	a.drawSlider(g, l.widthSlider, a.opts.Width, sliderMinWidth, sliderMaxWidth, ctrlWidth)

	g.drawText(a.theme.uiSmall, "透明度阈值", Rect{x, b + a.theme.px(112), x + w/2, b + a.theme.px(128)}, pal.textDim, alignNear, false)
	g.drawText(a.theme.uiBold, fmt.Sprintf("%d", a.opts.AlphaThreshold),
		Rect{x + w/2, b + a.theme.px(112), x + w, b + a.theme.px(128)}, pal.text, alignFar, false)
	a.drawSlider(g, l.thresholdSlider, int(a.opts.AlphaThreshold), 0, 255, ctrlThreshold)

	a.drawCheck(g, l.colorCheck, a.colorOn, "彩色", ctrlColor)
	a.drawCheck(g, l.invertCheck, a.opts.Invert, "反色", ctrlInvert)

	g.hline(x, x+w, b+a.theme.px(196), pal.divider, 1)
	a.drawButton(g, l.openBtn, "打开图片", btnPrimary, ctrlOpen, false)
}

func (a *App) drawHeader(g *gfx, l layout) {
	pal := a.theme.pal
	x := l.card.Left
	right := l.zoomGroup.Left - a.theme.px(16)

	title := "预览"
	if a.mode != "" {
		title = "预览 · " + a.mode.Label()
	}
	g.drawText(a.theme.uiBold, title, Rect{x, a.theme.px(16), right, a.theme.px(38)}, pal.text, alignNear, false)

	meta := a.status
	if a.img == nil {
		meta = "拖入图片，或点击左侧「打开图片」"
	} else if meta == "" {
		meta = "正在渲染…"
	}
	if a.errMsg != "" {
		g.drawText(a.theme.uiSmall, a.errMsg, Rect{x, a.theme.px(38), right, a.theme.px(58)}, pal.warn, alignNear, false)
	} else {
		g.drawText(a.theme.uiSmall, meta, Rect{x, a.theme.px(38), right, a.theme.px(58)}, pal.textDim, alignNear, false)
	}

	a.drawButton(g, l.copyBtn, "复制", btnSecondary, ctrlCopy, a.art == nil)
	a.drawButton(g, l.exportBtn, "导出 TXT", btnSecondary, ctrlExport, a.art == nil)

	if a.img == nil {
		a.drawEmptyState(g, l)
	} else if a.art == nil && a.errMsg != "" {
		g.drawText(a.theme.uiBase, a.errMsg, inset(l.canvas, a.theme.px(24)), argb(0xE8, 0x9C, 0xA4, 0xFF), alignCenter, true)
	}
}

func (a *App) drawEmptyState(g *gfx, l layout) {
	c := l.canvas
	cx := float32(c.Left+c.Right) / 2
	cy := float32(c.Top+c.Bottom) / 2

	// 用基本图形画出一个终端窗口的图标，使程序除了自带的应用图标外不需要其他图标资源。
	w := a.theme.fpx(76)
	h := a.theme.fpx(58)
	r := Rect{
		int32(cx - w/2), int32(cy - h/2 - a.theme.fpx(22)),
		int32(cx + w/2), int32(cy + h/2 - a.theme.fpx(22)),
	}
	line := argb(0x4A, 0x50, 0x55, 0xFF)
	g.strokeRoundRect(r, a.theme.fpx(6), line, a.theme.fpx(1.5))
	g.hline(r.Left+a.theme.px(2), r.Right-a.theme.px(2), r.Top+a.theme.px(15), line, 1)
	for i := 0; i < 3; i++ {
		g.fillRectF(float32(r.Left+a.theme.px(float32(11+i*9))), float32(r.Top+a.theme.px(6)), a.theme.fpx(4), a.theme.fpx(4), line)
	}
	g.fillRectF(float32(r.Left+a.theme.px(16)), float32(r.Top+a.theme.px(28)), a.theme.fpx(11), a.theme.fpx(2), argb(0x3D, 0xAE, 0xE9, 0xFF))
	g.fillRectF(float32(r.Left+a.theme.px(16)), float32(r.Top+a.theme.px(35)), a.theme.fpx(26), a.theme.fpx(2), argb(0x6B, 0x72, 0x77, 0xFF))

	g.drawText(a.theme.uiBase, "打开一张图片开始", Rect{c.Left, int32(cy) + a.theme.px(14), c.Right, int32(cy) + a.theme.px(38)}, argb(0xA8, 0xAF, 0xB4, 0xFF), alignCenter, true)
	g.drawText(a.theme.uiSmall, "支持 PNG / JPEG / GIF，可直接拖放", Rect{c.Left, int32(cy) + a.theme.px(38), c.Right, int32(cy) + a.theme.px(58)}, argb(0x6B, 0x72, 0x77, 0xFF), alignCenter, true)
}

// --------------------------------------------------------------- 字符画绘制

type gdiFont struct {
	h     HFONT
	cellW float32
	cellH float32
	size  float32
	name  string
	key   string
}

func (a *App) freeArtFont() {
	if a.artFont != nil {
		procDeleteObject.Call(uintptr(a.artFont.h))
		a.artFont = nil
	}
}

func (a *App) monoFontName() string {
	if a.monoName != "" {
		return a.monoName
	}
	a.monoName = pickFont(a.measureDC, []string{"Cascadia Mono", "Consolas", "DejaVu Sans Mono", "Courier New"}, 'M')
	return a.monoName
}

// pickFont 返回第一个包含示例字符的候选字体，避免盲文字形缺失时显示出一排空白方框。
func pickFont(dc HDC, candidates []string, sample rune) string {
	for _, name := range candidates {
		if fontHasGlyph(dc, name, 16, sample) {
			return name
		}
	}
	return candidates[len(candidates)-1]
}

func fontHasGlyph(dc HDC, name string, size int32, r rune) bool {
	hf, _, _ := procCreateFontW.Call(
		iptr(-size), 0, 0, 0, fwNormal, 0, 0, 0,
		defaultCharset, outTTPrecis, clipDefaultPrecis, antialiased,
		fixedPitch|ffModern, uintptr(unsafe.Pointer(u16(name))))
	if hf == 0 {
		return false
	}
	defer procDeleteObject.Call(hf)
	old, _, _ := procSelectObject.Call(uintptr(dc), hf)
	defer procSelectObject.Call(uintptr(dc), old)

	ch := []uint16{uint16(r)}
	var idx uint16
	ok, _, _ := procGetGlyphIndicesW.Call(uintptr(dc), uintptr(unsafe.Pointer(&ch[0])), 1,
		uintptr(unsafe.Pointer(&idx)), ggiMarkNonExisting)
	return ok != 0 && idx != 0xFFFF
}

// artSample 是用于度量单元格的字符。
// 选取当前模式下最宽的字符可以使各列保持对齐。
func (a *App) artSample() rune {
	switch a.mode {
	case render.ModeBraille:
		return 0x28FF
	case render.ModeBlocks:
		return 0x2580
	}
	return 'M'
}

func (a *App) ensureArtFont() {
	if a.art == nil || a.art.Cols == 0 || a.theme == nil {
		return
	}
	inner := a.viewport()
	availW := float32(inner.Width())
	availH := float32(inner.Height())
	if availW < 40 || availH < 30 {
		return
	}
	name := a.monoFontName()
	if a.mode == render.ModeBraille {
		name = pickFont(a.measureDC, []string{"Cascadia Mono", "Segoe UI Symbol", "Noto Sans Mono", "DejaVu Sans Mono", "Consolas"}, 0x28FF)
	}

	zoomPart := "auto"
	if !a.zoomAuto {
		zoomPart = fmt.Sprintf("%.4f", a.zoom)
	}
	key := fmt.Sprintf("%s|%d|%d|%d|%d|%s", name, a.art.Cols, a.art.Rows, int(availW), int(availH), zoomPart)
	if a.artFont != nil && a.artFont.key == key {
		return
	}
	a.freeArtFont()

	base := a.theme.fpx(baseFontSize)
	size := base
	if !a.zoomAuto {
		size = base * float32(a.zoom)
	}
	size = clampF32(size, a.theme.fpx(minArtFont), a.theme.fpx(maxArtFont))

	f := a.newArtFont(name, size)
	if f == nil {
		return
	}

	if a.zoomAuto {
		// 逐步逼近刚好填满可视区的字号。
		// 对任何字体两次遍历都够用：第一次修正估计值，第二次修正整数单元格度量取整带来的偏差。
		for i := 0; i < 2; i++ {
			scale := fitScale(f, availW, availH, a.art.Cols, a.art.Rows)
			if math.Abs(float64(scale-1)) < 0.02 {
				break
			}
			next := clampF32(f.size*scale, a.theme.fpx(minArtFont), a.theme.fpx(maxArtFont))
			c2 := a.newArtFont(name, next)
			if c2 == nil {
				break
			}
			procDeleteObject.Call(uintptr(f.h))
			f = c2
		}
		a.zoom = float64(f.size / base)
	}

	f.key = key
	a.artFont = f
}

// fitScale 返回使网格刚好填满可视区的缩放比例。
func fitScale(f *gdiFont, availW, availH float32, cols, rows int) float32 {
	w := f.cellW * float32(cols)
	h := f.cellH * float32(rows)
	if w <= 0 || h <= 0 {
		return 1
	}
	s := availW / w
	if v := availH / h; v < s {
		s = v
	}
	return s
}

func clampF32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// newArtFont 创建用于绘制网格的 GDI 字体，并度量生成的单元格矩形。
func (a *App) newArtFont(name string, size float32) *gdiFont {
	hf, _, _ := procCreateFontW.Call(
		iptr(-int32(size+0.5)), 0, 0, 0, fwNormal, 0, 0, 0,
		defaultCharset, outTTPrecis, clipDefaultPrecis, antialiased,
		fixedPitch|ffModern, uintptr(unsafe.Pointer(u16(name))))
	if hf == 0 {
		return nil
	}

	old, _, _ := procSelectObject.Call(uintptr(a.measureDC), hf)
	var tm TextMetric
	procGetTextMetricsW.Call(uintptr(a.measureDC), uintptr(unsafe.Pointer(&tm)))

	sample := []uint16{uint16(a.artSample())}
	var sz Size
	procGetTextExtentPoint32W.Call(uintptr(a.measureDC), uintptr(unsafe.Pointer(&sample[0])), 1, uintptr(unsafe.Pointer(&sz)))
	procSelectObject.Call(uintptr(a.measureDC), old)

	cellH := float32(tm.TmHeight)
	if cellH < 4 {
		cellH = size
	}
	if sz.CX < 1 {
		sz.CX = int32(size * 0.6)
	}
	return &gdiFont{
		h:     HFONT(hf),
		cellW: float32(sz.CX),
		cellH: cellH,
		size:  size,
		name:  name,
	}
}

func (a *App) drawArtText(dc HDC) {
	if a.art == nil || a.artFont == nil {
		return
	}
	inner := a.viewport()
	cellW, cellH := a.artFont.cellW, a.artFont.cellH
	if cellW <= 0 || cellH <= 0 {
		return
	}

	contentW := cellW * float32(a.art.Cols)
	contentH := cellH * float32(a.art.Rows)

	originX := float32(inner.Left) - a.scrollX
	if contentW <= float32(inner.Width()) {
		originX = float32(inner.Left) + (float32(inner.Width())-contentW)/2
	}
	originY := float32(inner.Top) - a.scrollY
	if contentH <= float32(inner.Height()) {
		originY = float32(inner.Top) + (float32(inner.Height())-contentH)/2
	}

	procIntersectClipRect.Call(uintptr(dc), uintptr(inner.Left), uintptr(inner.Top),
		uintptr(inner.Right), uintptr(inner.Bottom))
	defer procSelectClipRgn.Call(uintptr(dc), 0)

	oldFont, _, _ := procSelectObject.Call(uintptr(dc), uintptr(a.artFont.h))
	defer procSelectObject.Call(uintptr(dc), oldFont)
	procSetBkMode.Call(uintptr(dc), transparent)

	r0 := int(math.Max(0, math.Floor(float64(a.scrollY/cellH))))
	r1 := int(math.Ceil(float64((a.scrollY+float32(inner.Height()))/cellH))) + 1
	c0 := int(math.Max(0, math.Floor(float64(a.scrollX/cellW))))
	c1 := int(math.Ceil(float64((a.scrollX+float32(inner.Width()))/cellW))) + 1
	if r1 > a.art.Rows {
		r1 = a.art.Rows
	}
	if c1 > a.art.Cols {
		c1 = a.art.Cols
	}

	blocks := a.art.Mode == render.ModeBlocks
	mono := rgb(0xC9, 0xCD, 0xD1)
	monoBG := rgb(0x24, 0x28, 0x2B)

	cellRect := func(x, y int) Rect {
		px := int32(math.Round(float64(originX + float32(x)*cellW)))
		py := int32(math.Round(float64(originY + float32(y)*cellH)))
		return Rect{px, py, px + int32(cellW) + 1, py + int32(cellH) + 1}
	}

	// 半块单元格的字符下方需要背景色。
	// 先填充所有单元格、再绘制字符，可以避免相邻的填充把前一个字符的右缘裁剪成可见的接缝。
	if blocks {
		var brush HBRUSH
		var lastBG COLORREF
		bgValid := false
		for y := r0; y < r1; y++ {
			for x := c0; x < c1; x++ {
				cell := a.art.At(x, y)
				if cell.Blank() {
					continue
				}
				bg := rgb(cell.BG.R, cell.BG.G, cell.BG.B)
				if !a.colorOn {
					bg = monoBG
				}
				if !bgValid || bg != lastBG {
					if brush != 0 {
						procDeleteObject.Call(uintptr(brush))
					}
					brush = HBRUSH(callOptional(procCreateSolidBrush, uintptr(bg)))
					lastBG, bgValid = bg, true
				}
				r := cellRect(x, y)
				procFillRect.Call(uintptr(dc), uintptr(unsafe.Pointer(&r)), uintptr(brush))
			}
		}
		if brush != 0 {
			procDeleteObject.Call(uintptr(brush))
		}
	}

	var lastFG COLORREF
	fgValid := false

	for y := r0; y < r1; y++ {
		for x := c0; x < c1; x++ {
			cell := a.art.At(x, y)
			if cell.Blank() {
				continue
			}
			r := cellRect(x, y)

			fg := rgb(cell.FG.R, cell.FG.G, cell.FG.B)
			if !a.colorOn {
				fg = mono
			}
			if !fgValid || fg != lastFG {
				procSetTextColor.Call(uintptr(dc), uintptr(fg))
				lastFG, fgValid = fg, true
			}

			ch := []uint16{uint16(cell.Ch)}
			procExtTextOutW.Call(uintptr(dc), uintptr(r.Left), uintptr(r.Top), 0,
				0, uintptr(unsafe.Pointer(&ch[0])), 1, 0)
		}
	}
}

// ----------------------------------------------------------------- 滚动条

// viewport 是绘制字符画网格的矩形区域。
func (a *App) viewport() Rect {
	return inset(a.layout().canvas, a.theme.px(10))
}

// scrollGeom 描述当前帧滚动条的几何参数。
// 绘制与命中测试使用同一套几何参数，因此用户看到的滑块就是用户可以拖动的滑块。
type scrollGeom struct {
	hasV, hasH bool
	vHit       Rect // 较宽的抓取区域，比可见的轨道更宽
	vTrack     Rect
	vThumb     Rect
	hHit       Rect
	hTrack     Rect
	hThumb     Rect
}

func (a *App) scrollGeometry() scrollGeom {
	var g scrollGeom
	if a.art == nil || a.artFont == nil {
		return g
	}
	inner := a.viewport()
	viewW := float32(inner.Width())
	viewH := float32(inner.Height())
	contentW := a.artFont.cellW * float32(a.art.Cols)
	contentH := a.artFont.cellH * float32(a.art.Rows)

	trackW := a.theme.fpx(6)
	if contentH > viewH+0.5 {
		g.hasV = true
		g.vTrack = Rect{inner.Right - int32(trackW), inner.Top, inner.Right, inner.Bottom}
		thumbH := viewH * viewH / contentH
		if thumbH < a.theme.fpx(30) {
			thumbH = a.theme.fpx(30)
		}
		span := viewH - thumbH
		pos := float32(0)
		if contentH-viewH > 0 {
			pos = a.scrollY / (contentH - viewH) * span
		}
		top := float32(inner.Top) + pos
		g.vThumb = Rect{inner.Right - int32(trackW), int32(top), inner.Right, int32(top + thumbH)}
		g.vHit = Rect{inner.Right - a.theme.px(16), inner.Top, inner.Right, inner.Bottom}
	}
	if contentW > viewW+0.5 {
		g.hasH = true
		g.hTrack = Rect{inner.Left, inner.Bottom - int32(trackW), inner.Right, inner.Bottom}
		thumbW := viewW * viewW / contentW
		if thumbW < a.theme.fpx(30) {
			thumbW = a.theme.fpx(30)
		}
		span := viewW - thumbW
		pos := float32(0)
		if contentW-viewW > 0 {
			pos = a.scrollX / (contentW - viewW) * span
		}
		left := float32(inner.Left) + pos
		g.hThumb = Rect{int32(left), inner.Bottom - int32(trackW), int32(left + thumbW), inner.Bottom}
		g.hHit = Rect{inner.Left, inner.Bottom - a.theme.px(16), inner.Right, inner.Bottom}
	}
	return g
}

func (a *App) clampScroll() {
	if a.art == nil || a.artFont == nil {
		a.scrollX, a.scrollY = 0, 0
		return
	}
	inner := a.viewport()
	maxX := a.artFont.cellW*float32(a.art.Cols) - float32(inner.Width())
	maxY := a.artFont.cellH*float32(a.art.Rows) - float32(inner.Height())
	if maxX < 0 {
		maxX = 0
	}
	if maxY < 0 {
		maxY = 0
	}
	a.scrollX = clampF32(a.scrollX, 0, maxX)
	a.scrollY = clampF32(a.scrollY, 0, maxY)
}

func (a *App) drawScrollbars(g *gfx, l layout) {
	geom := a.scrollGeometry()
	thumb := argb(0x6A, 0x72, 0x78, 0xFF)
	if a.hover == ctrlScrollV || a.dragScroll == scrollVertical ||
		a.hover == ctrlScrollH || a.dragScroll == scrollHorizontal {
		thumb = argb(0x8C, 0x95, 0x9B, 0xFF)
	}

	if geom.hasV {
		g.fillRectF(float32(geom.vTrack.Left), float32(geom.vTrack.Top),
			float32(geom.vTrack.Width()), float32(geom.vTrack.Height()), argb(0x2A, 0x2F, 0x33, 0xFF))
		g.fillRoundRect(geom.vThumb, a.theme.fpx(3), thumb)
	}
	if geom.hasH {
		g.fillRectF(float32(geom.hTrack.Left), float32(geom.hTrack.Top),
			float32(geom.hTrack.Width()), float32(geom.hTrack.Height()), argb(0x2A, 0x2F, 0x33, 0xFF))
		g.fillRoundRect(geom.hThumb, a.theme.fpx(3), thumb)
	}
}

// ---------------------------------------------------------------- 缩放控件

func (a *App) drawZoomBar(g *gfx, l layout) {
	if a.art == nil {
		return
	}
	pal := a.theme.pal

	g.fillRoundRect(l.zoomGroup, a.theme.fpx(radius), pal.card)
	g.strokeRoundRect(l.zoomGroup, a.theme.fpx(radius), pal.border, 1)

	// 单元格之间用细线分隔，这是 Breeze 主题对动作分组的做法。
	for _, x := range []int32{l.zoomValue.Left, l.zoomIn.Left, l.zoomFit.Left, l.zoom100.Left} {
		g.fillRectF(float32(x), float32(l.zoomGroup.Top+7), 1, float32(l.zoomGroup.Height()-14), pal.divider)
	}

	a.drawZoomCell(g, l.zoomOut, "−", ctrlZoomOut, false, pal)
	a.drawZoomCell(g, l.zoomIn, "+", ctrlZoomIn, false, pal)
	a.drawZoomCell(g, l.zoomFit, "适应", ctrlZoomFit, a.zoomAuto, pal)
	a.drawZoomCell(g, l.zoom100, "100%", ctrlZoom100, !a.zoomAuto && math.Abs(a.zoom-1) < 0.005, pal)

	// 百分比值同时用作文本输入框。
	switch {
	case a.zoomEdit:
		g.fillRoundRect(inset(l.zoomValue, 1), a.theme.fpx(radius-1), pal.accentTint)
	case a.hover == ctrlZoomValue:
		g.fillRoundRect(inset(l.zoomValue, 1), a.theme.fpx(radius-1), pal.hover)
	}
	label := fmt.Sprintf("%d%%", int(math.Round(a.zoom*100)))
	if a.zoomEdit {
		label = a.zoomBuf
	}
	if a.zoomEdit {
		g.drawText(a.theme.uiBase, label, l.zoomValue, pal.text, alignCenter, true)
		if a.caretOn {
			w, _ := g.measure(a.theme.uiBase, label)
			cx := float32(l.zoomValue.Left+l.zoomValue.Right)/2 + w/2 + a.theme.fpx(2)
			cy := float32(l.zoomValue.Top+l.zoomValue.Bottom) / 2
			g.fillRectF(cx, cy-a.theme.fpx(7), a.theme.fpx(1.5), a.theme.fpx(14), pal.accent)
		}
	} else {
		g.drawText(a.theme.uiBase, label, l.zoomValue, pal.text, alignCenter, true)
	}
}

func (a *App) drawZoomCell(g *gfx, r Rect, label string, id int, selected bool, pal palette) {
	switch {
	case selected:
		g.fillRoundRect(inset(r, 1), a.theme.fpx(radius-1), pal.accentTint)
	case a.hover == id:
		g.fillRoundRect(inset(r, 1), a.theme.fpx(radius-1), pal.hover)
	}
	text := pal.text
	if selected {
		text = pal.accentLo
	}
	g.drawText(a.theme.uiBase, label, r, text, alignCenter, true)
}

// --------------------------------------------------------------------- 缩放

const zoomStep = 0.10

// setZoom 设置指定的缩放比例，并退出自动适应状态。
func (a *App) setZoom(z float64) {
	z = math.Round(z*100) / 100
	if z < minZoom {
		z = minZoom
	}
	if z > maxZoom {
		z = maxZoom
	}
	a.zoom = z
	a.zoomAuto = false
	a.afterZoomChange()
}

// stepZoom 按一个 10% 的档位调整缩放，起点是当前显示的数值。
// 在自动适应状态下按加号会从适应后的数值继续调整。
func (a *App) stepZoom(delta float64) {
	a.setZoom(math.Round(a.zoom*10)/10 + delta)
}

// fitZoom 进入自动适应状态，按横向与纵向填满可视区。
func (a *App) fitZoom() {
	a.zoomAuto = true
	a.afterZoomChange()
}

func (a *App) afterZoomChange() {
	a.ensureArtFont()
	a.scrollX, a.scrollY = 0, 0
	a.clampScroll()
	a.invalidate()
}

func (a *App) startZoomEdit() {
	if a.art == nil {
		return
	}
	a.zoomEdit = true
	a.zoomBuf = strconv.Itoa(int(math.Round(a.zoom * 100)))
	a.zoomSelectAll = true
	a.caretOn = true
	procSetTimer.Call(uintptr(a.hwnd), timerCaret, 530, 0)
	a.invalidate()
}

func (a *App) commitZoomEdit() {
	if !a.zoomEdit {
		return
	}
	a.zoomEdit = false
	procKillTimer.Call(uintptr(a.hwnd), timerCaret)
	if v, err := strconv.Atoi(strings.TrimSpace(a.zoomBuf)); err == nil && v > 0 {
		a.setZoom(float64(v) / 100)
	}
	a.invalidate()
}

func (a *App) cancelZoomEdit() {
	a.zoomEdit = false
	procKillTimer.Call(uintptr(a.hwnd), timerCaret)
	a.invalidate()
}

// zoomChar 处理输入到缩放输入框的字符。
func (a *App) zoomChar(r rune) {
	switch {
	case r >= '0' && r <= '9':
		if a.zoomSelectAll {
			a.zoomBuf = ""
			a.zoomSelectAll = false
		}
		if len(a.zoomBuf) < 4 {
			a.zoomBuf += string(r)
		}
		a.caretOn = true
		a.invalidate()
	case r == 8: // 退格键
		if a.zoomSelectAll {
			a.zoomBuf = ""
			a.zoomSelectAll = false
		} else if len(a.zoomBuf) > 0 {
			a.zoomBuf = a.zoomBuf[:len(a.zoomBuf)-1]
		}
		a.invalidate()
	}
}

// -------------------------------------------------------------------- 控件

type btnKind int

const (
	btnSecondary btnKind = iota
	btnPrimary
)

func (a *App) drawButton(g *gfx, r Rect, label string, kind btnKind, id int, disabled bool) {
	pal := a.theme.pal
	hovered := a.hover == id && !disabled
	pressed := a.pressed == id && !disabled

	var fill, border, text uint32
	switch kind {
	case btnPrimary:
		fill, text = pal.accent, pal.textInv
		if hovered {
			fill = pal.accentHi
		}
		if pressed {
			fill = pal.accentLo
		}
		border = fill
	default:
		fill, border, text = pal.card, pal.border, pal.text
		if hovered {
			fill, border = argb(0xF0, 0xF6, 0xFB, 0xFF), pal.accent
		}
		if pressed {
			fill = argb(0xE5, 0xEF, 0xF7, 0xFF)
		}
		if disabled {
			fill, border, text = pal.cardInset, pal.divider, argb(0xB0, 0xB6, 0xBA, 0xFF)
		}
	}
	if disabled && kind == btnPrimary {
		fill, border, text = argb(0xBF, 0xD4, 0xE0, 0xFF), argb(0xBF, 0xD4, 0xE0, 0xFF), argb(0xF2, 0xF6, 0xF8, 0xFF)
	}

	g.fillRoundRect(r, a.theme.fpx(radius), fill)
	g.strokeRoundRect(r, a.theme.fpx(radius), border, 1)
	g.drawText(a.theme.uiBase, label, r, text, alignCenter, true)
}

func (a *App) drawSegmented(g *gfx, r Rect) {
	pal := a.theme.pal
	g.fillRoundRect(r, a.theme.fpx(radius), pal.card)
	g.strokeRoundRect(r, a.theme.fpx(radius), pal.border, 1)

	items := []struct {
		m  render.Mode
		id int
	}{
		{render.ModeASCII, ctrlModeASCII},
		{render.ModeBraille, ctrlModeBraille},
		{render.ModeBlocks, ctrlModeBlocks},
	}
	w := r.Width() / 3
	for i, item := range items {
		seg := Rect{r.Left + int32(i)*w, r.Top, r.Left + int32(i+1)*w, r.Bottom}
		if i == 2 {
			seg.Right = r.Right
		}
		selected := a.mode == item.m
		switch {
		case selected:
			g.fillRoundRect(inset(seg, 1), a.theme.fpx(radius-1), pal.accent)
			g.drawText(a.theme.uiBase, item.m.Label(), seg, pal.textInv, alignCenter, true)
		case a.hover == item.id:
			g.fillRoundRect(inset(seg, 1), a.theme.fpx(radius-1), pal.hover)
			g.drawText(a.theme.uiBase, item.m.Label(), seg, pal.text, alignCenter, true)
		default:
			g.drawText(a.theme.uiBase, item.m.Label(), seg, pal.text, alignCenter, true)
		}
		if i > 0 && !selected && a.mode != items[i-1].m {
			g.fillRectF(float32(seg.Left), float32(seg.Top+7), 1, float32(seg.Height()-14), pal.divider)
		}
	}
}

func (a *App) drawSlider(g *gfx, r Rect, value, min, max int, id int) {
	pal := a.theme.pal
	trackH := a.theme.fpx(4)
	cy := float32(r.Top) + float32(r.Height())/2
	x0 := float32(r.Left) + a.theme.fpx(8)
	x1 := float32(r.Right) - a.theme.fpx(8)

	frac := float64(value-min) / float64(max-min)
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	hx := x0 + float32(frac)*(x1-x0)

	g.fillRoundRect(Rect{
		int32(x0), int32(cy - trackH/2), int32(x1), int32(cy+trackH/2) + 1,
	}, trackH/2, pal.track)
	if hx > x0 {
		g.fillRoundRect(Rect{
			int32(x0), int32(cy - trackH/2), int32(hx), int32(cy+trackH/2) + 1,
		}, trackH/2, pal.accent)
	}

	rad := a.theme.fpx(7)
	handle := Rect{
		int32(hx - rad), int32(cy - rad), int32(hx + rad), int32(cy + rad),
	}
	switch {
	case a.dragSlider == id:
		g.fillRoundRect(handle, rad, pal.accent)
	case a.hover == id:
		g.fillRoundRect(handle, rad, pal.card)
		g.strokeRoundRect(handle, rad, pal.accent, a.theme.fpx(2.5))
	default:
		g.fillRoundRect(handle, rad, pal.card)
		g.strokeRoundRect(handle, rad, pal.border, a.theme.fpx(2))
	}
}

func (a *App) drawCheck(g *gfx, r Rect, checked bool, label string, id int) {
	pal := a.theme.pal
	box := a.theme.px(16)
	br := Rect{r.Left, r.Top + (r.Height()-box)/2, r.Left + box, r.Top + (r.Height()-box)/2 + box}

	if checked {
		g.fillRoundRect(br, a.theme.fpx(3), pal.accent)
		pen := g.pen(pal.textInv, a.theme.fpx(2))
		if pen != 0 {
			x0 := float32(br.Left) + float32(box)*0.24
			y0 := float32(br.Top) + float32(box)*0.52
			x1 := float32(br.Left) + float32(box)*0.43
			y1 := float32(br.Top) + float32(box)*0.72
			x2 := float32(br.Left) + float32(box)*0.76
			y2 := float32(br.Top) + float32(box)*0.30
			procGdipDrawLine.Call(uintptr(g.h), uintptr(pen), f32(x0), f32(y0), f32(x1), f32(y1))
			procGdipDrawLine.Call(uintptr(g.h), uintptr(pen), f32(x1), f32(y1), f32(x2), f32(y2))
			freePen(pen)
		}
	} else {
		g.fillRoundRect(br, a.theme.fpx(3), pal.card)
		border := pal.border
		if a.hover == id {
			border = pal.accent
		}
		g.strokeRoundRect(br, a.theme.fpx(3), border, 1)
	}

	g.drawText(a.theme.uiBase, label, Rect{br.Right + a.theme.px(8), r.Top, r.Right, r.Bottom}, pal.text, alignNear, true)
}

// -------------------------------------------------------------------- 输入

func (a *App) hitTest(x, y int32) int {
	l := a.layout()

	// 缩放控件位于标题栏中，因此命中测试在画布之前进行。
	if hit(l.zoomGroup, x, y) {
		switch {
		case hit(l.zoomOut, x, y):
			return ctrlZoomOut
		case hit(l.zoomIn, x, y):
			return ctrlZoomIn
		case hit(l.zoomValue, x, y):
			return ctrlZoomValue
		case hit(l.zoomFit, x, y):
			return ctrlZoomFit
		case hit(l.zoom100, x, y):
			return ctrlZoom100
		}
		return ctrlNone
	}

	// 滚动条的抓取区域比可见的轨道更宽。
	geom := a.scrollGeometry()
	if geom.hasV && hit(geom.vHit, x, y) {
		return ctrlScrollV
	}
	if geom.hasH && hit(geom.hHit, x, y) {
		return ctrlScrollH
	}

	if hit(l.modeSeg, x, y) {
		w := l.modeSeg.Width()
		switch {
		case x < l.modeSeg.Left+w/3:
			return ctrlModeASCII
		case x < l.modeSeg.Left+2*w/3:
			return ctrlModeBraille
		default:
			return ctrlModeBlocks
		}
	}
	for _, c := range []struct {
		r  Rect
		id int
	}{
		{l.widthSlider, ctrlWidth},
		{l.thresholdSlider, ctrlThreshold},
		{l.colorCheck, ctrlColor},
		{l.invertCheck, ctrlInvert},
		{l.openBtn, ctrlOpen},
		{l.exportBtn, ctrlExport},
		{l.copyBtn, ctrlCopy},
	} {
		if hit(c.r, x, y) {
			return c.id
		}
	}
	return ctrlNone
}

func hit(r Rect, x, y int32) bool {
	return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}

func (a *App) onMouseMove(x, y int32) {
	a.mouseX, a.mouseY = x, y
	if a.dragSlider != ctrlNone {
		a.updateSlider(a.dragSlider, x)
		return
	}
	if a.dragScroll != scrollNone {
		a.dragScrollTo(x, y)
		return
	}
	if h := a.hitTest(x, y); h != a.hover {
		a.hover = h
		a.invalidate()
	}

	tme := TrackMouseEvent{
		CbSize:    DWORD(unsafe.Sizeof(TrackMouseEvent{})),
		DwFlags:   tmeLeave,
		HwndTrack: a.hwnd,
	}
	callOptional(procTrackMouseEvent, uintptr(unsafe.Pointer(&tme)))
}

func (a *App) onMouseDown(x, y int32) {
	id := a.hitTest(x, y)
	if a.zoomEdit && id != ctrlZoomValue {
		a.commitZoomEdit()
	}
	a.pressed = id

	switch id {
	case ctrlWidth, ctrlThreshold:
		a.dragSlider = id
		procSetCapture.Call(uintptr(a.hwnd))
		a.updateSlider(id, x)
		return
	case ctrlScrollV:
		geom := a.scrollGeometry()
		a.dragScroll = scrollVertical
		a.scrollDragOffset = grabOffset(geom.vThumb, y, true)
		procSetCapture.Call(uintptr(a.hwnd))
		a.dragScrollTo(x, y)
		return
	case ctrlScrollH:
		geom := a.scrollGeometry()
		a.dragScroll = scrollHorizontal
		a.scrollDragOffset = grabOffset(geom.hThumb, x, false)
		procSetCapture.Call(uintptr(a.hwnd))
		a.dragScrollTo(x, y)
		return
	}
	a.invalidate()
}

// grabOffset 记录用户在滑块内部按下的位置，使滑块不会跳到鼠标指针下方。
// 按下轨道时滑块以该位置为中心。
func grabOffset(thumb Rect, pos int32, vertical bool) float32 {
	if vertical {
		if pos >= thumb.Top && pos < thumb.Bottom {
			return float32(pos - thumb.Top)
		}
		return float32(thumb.Height()) / 2
	}
	if pos >= thumb.Left && pos < thumb.Right {
		return float32(pos - thumb.Left)
	}
	return float32(thumb.Width()) / 2
}

func (a *App) dragScrollTo(x, y int32) {
	if a.art == nil || a.artFont == nil {
		return
	}
	geom := a.scrollGeometry()
	inner := a.viewport()

	switch a.dragScroll {
	case scrollVertical:
		thumbH := float32(geom.vThumb.Height())
		span := float32(inner.Height()) - thumbH
		if span <= 0 {
			return
		}
		contentH := a.artFont.cellH * float32(a.art.Rows)
		pos := float32(y) - float32(inner.Top) - a.scrollDragOffset
		a.scrollY = pos / span * (contentH - float32(inner.Height()))
	case scrollHorizontal:
		thumbW := float32(geom.hThumb.Width())
		span := float32(inner.Width()) - thumbW
		if span <= 0 {
			return
		}
		contentW := a.artFont.cellW * float32(a.art.Cols)
		pos := float32(x) - float32(inner.Left) - a.scrollDragOffset
		a.scrollX = pos / span * (contentW - float32(inner.Width()))
	}
	a.clampScroll()
	a.invalidate()
}

func (a *App) onMouseUp(x, y int32) {
	id := a.pressed
	a.pressed = ctrlNone
	if a.dragSlider != ctrlNone || a.dragScroll != scrollNone {
		a.dragSlider = ctrlNone
		a.dragScroll = scrollNone
		procReleaseCapture.Call()
		a.invalidate()
		return
	}
	a.invalidate()
	if id == ctrlNone || a.hitTest(x, y) != id {
		return
	}
	switch id {
	case ctrlOpen:
		a.openDialog()
	case ctrlExport:
		a.exportDialog()
	case ctrlCopy:
		a.copyToClipboard()
	case ctrlModeASCII:
		a.setMode(render.ModeASCII)
	case ctrlModeBraille:
		a.setMode(render.ModeBraille)
	case ctrlModeBlocks:
		a.setMode(render.ModeBlocks)
	case ctrlColor:
		a.colorOn = !a.colorOn
		a.invalidate()
	case ctrlInvert:
		a.opts.Invert = !a.opts.Invert
		a.requestRender()
	case ctrlZoomOut:
		a.stepZoom(-zoomStep)
	case ctrlZoomIn:
		a.stepZoom(zoomStep)
	case ctrlZoomFit:
		a.fitZoom()
	case ctrlZoom100:
		a.setZoom(1.0)
	case ctrlZoomValue:
		a.startZoomEdit()
	}
}

func (a *App) updateSlider(id int, x int32) {
	l := a.layout()
	var r Rect
	var min, max int
	if id == ctrlWidth {
		r, min, max = l.widthSlider, sliderMinWidth, sliderMaxWidth
	} else {
		r, min, max = l.thresholdSlider, 0, 255
	}
	x0 := float32(r.Left) + a.theme.fpx(8)
	x1 := float32(r.Right) - a.theme.fpx(8)
	frac := (float64(x) - float64(x0)) / float64(x1-x0)
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	v := min + int(frac*float64(max-min)+0.5)
	if id == ctrlWidth {
		if v != a.opts.Width {
			a.opts.Width = v
			a.scheduleRender()
			a.invalidate()
		}
		return
	}
	if uint8(v) != a.opts.AlphaThreshold {
		a.opts.AlphaThreshold = uint8(v)
		a.scheduleRender()
		a.invalidate()
	}
}
