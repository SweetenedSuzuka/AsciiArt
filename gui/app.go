package gui

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"asciiart/internal/ansi"
	"asciiart/internal/artio"
	"asciiart/internal/limits"
	"asciiart/internal/render"
	"asciiart/internal/safedecode"
)

// renderResult 是工作协程执行渲染后传回的结果。
type renderResult struct {
	seq int
	art *render.Art
	err error
}

// 控件编号。鼠标移动和点击时据此判断指针位于哪个控件上，并记录悬停与按下的控件。
const (
	ctrlNone = iota
	ctrlOpen
	ctrlExport
	ctrlCopy
	ctrlModeASCII
	ctrlModeBraille
	ctrlModeBlocks
	ctrlWidth
	ctrlThreshold
	ctrlColor
	ctrlInvert
	ctrlZoomOut
	ctrlZoomIn
	ctrlZoomValue
	ctrlZoomFit
	ctrlZoom100
	ctrlScrollV
	ctrlScrollH
)

// 滚动条拖动的状态编号。
const (
	scrollNone = iota
	scrollVertical
	scrollHorizontal
)

const (
	timerDebounce = 1
	timerCaret    = 2
	windowTitle   = "AsciiArt"
)

// App 保存整个应用程序的状态。它只存在于界面线程中。
// 工作协程只通过 mu 保护下的 result 把渲染结果交回界面线程。
type App struct {
	hwnd  HWND
	hInst HINSTANCE

	theme *theme
	dpi   int32

	// 背景缓冲区。所有界面内容先在这里合成，再每帧一次性复制到窗口，缩放和滚动时不会闪烁。
	memDC   HDC
	memBMP  HBITMAP
	memBits unsafe.Pointer
	bufW    int32
	bufH    int32

	// 当前图片与渲染结果。
	img     *safedecode.Image
	imgPath string
	imgBmp  GpBitmap
	imgHBM  HBITMAP

	mode   render.Mode
	opts   render.Options
	art    *render.Art
	status string
	errMsg string

	// 交互状态。
	hover            int
	pressed          int
	dragSlider       int
	dragScroll       int
	scrollDragOffset float32
	scrollX          float32
	scrollY          float32

	// 缩放状态。zoom 为 1.0 表示按 100% 显示。
	// zoomAuto 为真时，缩放比例跟随窗口尺寸自动调整。
	zoom          float64
	zoomAuto      bool
	zoomEdit      bool
	zoomBuf       string
	zoomSelectAll bool
	caretOn       bool

	artFont *gdiFont

	// GDI 辅助资源。
	measureDC HDC
	monoName  string

	colorOn bool

	// 渲染流程的并发状态。
	mu            sync.Mutex
	seq           int
	result        *renderResult
	busyMu        sync.Mutex
	rendering     bool
	renderPending bool

	mouseX, mouseY int32
}

var (
	app         *App
	wndProcPtr  uintptr
	handCursor  HCURSOR
	arrowCursor HCURSOR
)

// Options 配置启动时打开的图片、渲染模式和输出宽度。
type Options struct {
	Image string
	Mode  render.Mode
	Width int
}

// Run 创建窗口并运行消息循环，直到窗口关闭。
func Run(opt Options) int {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	callOptional(procSetProcessDPIAware)

	if !gpStartup() {
		return 1
	}
	defer gpShutdown()

	hInst, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)

	base := render.DefaultOptions()
	if opt.Width >= limits.MinOutputWidth && opt.Width <= limits.MaxOutputWidth {
		// 滑块只覆盖 20 到 240 的范围。
		// 命令行传入更大的值会使滑块手柄超出滑槽。
		w := opt.Width
		if w < sliderMinWidth {
			w = sliderMinWidth
		}
		if w > sliderMaxWidth {
			w = sliderMaxWidth
		}
		base.Width = w
	}
	mode := opt.Mode
	if mode == "" {
		mode = render.ModeASCII
	}

	a := &App{
		hInst:      HINSTANCE(hInst),
		mode:       mode,
		opts:       base,
		hover:      ctrlNone,
		pressed:    ctrlNone,
		colorOn:    true,
		dragSlider: ctrlNone,
		zoom:       1.0,
		zoomAuto:   true,
	}
	// 主题对象必须在窗口之前创建。
	// WM_GETMINMAXINFO 和 WM_PAINT 可能在 CreateWindowEx 尚未返回时就到达。
	a.dpi = int32(callOptional(procGetDpiForWindow, 0))
	if a.dpi == 0 {
		a.dpi = 96
	}
	a.theme = newTheme(a.dpi)
	defer func() {
		if a.theme != nil {
			a.theme.free()
		}
	}()

	dc, _, _ := procCreateCompatibleDC.Call(0)
	a.measureDC = HDC(dc)
	defer procDeleteDC.Call(uintptr(a.measureDC))

	app = a

	arrowCursor = HCURSOR(callOptional(procLoadCursorW, 0, idcArrow))
	handCursor = HCURSOR(callOptional(procLoadCursorW, 0, idcHand))

	icon, _, _ := procLoadIconW.Call(0, idiApplication)

	className := utf16Ptr("AsciiArtWindowClass")
	wc := WndClassEx{
		CbSize:        UINT(unsafe.Sizeof(WndClassEx{})),
		Style:         csHRedraw | csVRedraw,
		HInstance:     a.hInst,
		HIcon:         HICON(icon),
		HCursor:       HCURSOR(arrowCursor),
		HbrBackground: 0,
		LpszClassName: className,
		HIconSm:       HICON(icon),
	}
	wndProcPtr = syscall.NewCallback(wndProc)
	wc.LpfnWndProc = wndProcPtr
	if atom, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return 1
	}

	style := uintptr(wsOverlappedWindow | wsVisible)
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16Ptr(windowTitle))),
		style,
		cwUseDefault, cwUseDefault, 1200, 800,
		0, 0, uintptr(a.hInst), 0,
	)
	if hwnd == 0 {
		return 1
	}
	a.hwnd = HWND(hwnd)

	// 使用浅色标题栏，与浅色的客户区保持一致。
	useDark := uint32(0)
	callOptional(procDwmSetWindowAttribute, uintptr(a.hwnd), dwmwaUseImmersiveDarkMode,
		uintptr(unsafe.Pointer(&useDark)), unsafe.Sizeof(useDark))

	a.dpi = int32(callOptional(procGetDpiForWindow, uintptr(a.hwnd)))
	if a.dpi == 0 {
		a.dpi = 96
	}
	if old := a.theme; old != nil {
		old.free()
	}
	a.theme = newTheme(a.dpi)

	// 显式地设置窗口尺寸并居中。
	// CreateWindowExW 的参数使用设备的物理像素，布局使用设计像素，只有在得知显示器 DPI 之后两者才能换算一致。
	a.centerWindow(1180, 780)

	procDragAcceptFiles.Call(uintptr(a.hwnd), 1)

	procShowWindow.Call(uintptr(a.hwnd), swShow)
	procUpdateWindow.Call(uintptr(a.hwnd))

	if opt.Image != "" {
		a.loadImage(opt.Image)
	}

	var msg Msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	a.releaseResources()
	return int(msg.WParam)
}

func (a *App) releaseResources() {
	a.freeBackBuffer()
	a.freeArtFont()
	if a.imgBmp != 0 {
		freeImage(a.imgBmp)
		a.imgBmp = 0
	}
	if a.imgHBM != 0 {
		procDeleteObject.Call(uintptr(a.imgHBM))
		a.imgHBM = 0
	}
}

// centerWindow 把窗口调整为给定的设计尺寸，并使窗口在当前所在显示器的工作区内居中。
// 设计尺寸以设计像素表示，只在最后一步换算为设备像素。
// 窗口尺寸同时限制在工作区范围内，因为在 200% 缩放的 1080p 显示器上，工作区只有 960x540 的设计像素可用，窗口过大会使一半的界面显示在屏幕之外。
func (a *App) centerWindow(designW, designH float32) {
	s := float32(a.dpi) / 96
	if s < 1 {
		s = 1
	}

	work := Rect{0, 0, int32(callOptional(procGetSystemMetrics, 0)), int32(callOptional(procGetSystemMetrics, 1))}
	var mi MonitorInfo
	mi.CbSize = DWORD(unsafe.Sizeof(MonitorInfo{}))
	mon := callOptional(procMonitorFromWindow, uintptr(a.hwnd), 2 /* 取最近的显示器 */)
	if mon != 0 && callOptional(procGetMonitorInfoW, mon, uintptr(unsafe.Pointer(&mi))) != 0 {
		work = mi.RcWork
	}

	// 把工作区换算为设计像素，再让窗口尺寸收进工作区以内。
	availW := float32(work.Width()) / s
	availH := float32(work.Height()) / s
	wl := designW
	hl := designH
	if wl > availW-24 {
		wl = availW - 24
	}
	if hl > availH-24 {
		hl = availH - 24
	}
	if wl < 640 {
		wl = 640
	}
	if hl < 440 {
		hl = 440
	}

	w := int32(wl*s + 0.5)
	h := int32(hl*s + 0.5)
	x := work.Left + (work.Width()-w)/2
	y := work.Top + (work.Height()-h)/2
	procSetWindowPos.Call(uintptr(a.hwnd), 0,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h), swpNoZOrder)
}

// ------------------------------------------------------------------ 窗口过程

func wndProc(hwnd HWND, msg uint32, wParam, lParam uintptr) uintptr {
	a := app
	if a == nil {
		r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
		return r
	}
	// 原生回调内部发生 panic 会让进程直接退出，界面上不会显示任何信息。
	// 这里用 recover 捕获异常，把错误显示在界面上，让窗口过程返回而不退出进程。
	defer func() {
		if rec := recover(); rec != nil {
			a.errMsg = fmt.Sprintf("内部错误：%v", rec)
			a.invalidate()
		}
	}()

	switch msg {
	case wmEraseBkgnd:
		return 1 // 背景已在 wmPaint 中完整绘制

	case wmPaint:
		a.paint(hwnd)
		return 0

	case wmSize:
		a.invalidate()
		return 0

	case wmGetMinMaxInfo:
		mmi := (*MinMaxInfo)(unsafe.Pointer(lParam))
		mmi.PtMinTrackSize = Point{X: a.theme.px(860), Y: a.theme.px(500)}
		return 0

	case wmDpiChanged:
		a.dpi = int32(hiword(wParam))
		if old := a.theme; old != nil {
			old.free()
		}
		a.theme = newTheme(a.dpi)
		// 操作系统给出了窗口应当使用的矩形。
		r := (*Rect)(unsafe.Pointer(lParam))
		procSetWindowPos.Call(uintptr(hwnd), 0,
			uintptr(r.Left), uintptr(r.Top),
			uintptr(r.Width()), uintptr(r.Height()),
			swpNoZOrder|swpNoActivate)
		a.freeArtFont()
		a.monoName = ""
		a.invalidate()
		return 0

	case wmMouseMove:
		x, y := getX(lParam), getY(lParam)
		a.onMouseMove(x, y)
		return 0

	case wmMouseLeave:
		a.hover = ctrlNone
		a.invalidate()
		return 0

	case wmLButtonDown:
		a.onMouseDown(getX(lParam), getY(lParam))
		return 0

	case wmLButtonUp:
		a.onMouseUp(getX(lParam), getY(lParam))
		return 0

	case wmMouseWheel:
		if a.pointInCanvas(a.mouseX, a.mouseY) {
			delta := float32(int16((wParam >> 16) & 0xffff))
			step := float32(a.theme.px(48))
			shift, _, _ := procGetKeyState.Call(0x10) // VK_SHIFT
			if int16(shift) < 0 {
				// 多数鼠标没有横向滚轮，因此按住 Shift 滚动时执行左右平移。
				if delta > 0 {
					a.scrollX -= step
				} else {
					a.scrollX += step
				}
			} else if delta > 0 {
				a.scrollY -= step
			} else {
				a.scrollY += step
			}
			a.clampScroll()
			a.invalidate()
		}
		return 0

	case wmKeyDown:
		if a.onKey(wParam) {
			return 0
		}

	case wmChar:
		if a.zoomEdit {
			a.zoomChar(rune(wParam))
			return 0
		}

	case wmTimer:
		switch wParam {
		case timerDebounce:
			procKillTimer.Call(uintptr(hwnd), timerDebounce)
			a.requestRender()
		case timerCaret:
			a.caretOn = !a.caretOn
			a.invalidate()
		}
		return 0

	case wmSetCursor:
		// 在可交互的控件上显示手形光标，用来提示该控件可以点击。
		// 其余区域由 DefWindowProc 恢复为箭头光标。
		if int32(lParam&0xffff) == htClient && a.hover != ctrlNone {
			procSetCursor.Call(uintptr(handCursor))
			return 1
		}
		return 0

	case wmCaptureChanged:
		// 失去鼠标捕获时（切换窗口、其他窗口接管鼠标），滑块和滚动条都不应继续跟随指针移动。
		a.dragSlider = ctrlNone
		a.dragScroll = scrollNone
		a.pressed = ctrlNone
		a.invalidate()
		return 0

	case wmDropFiles:
		a.onDrop(HDROP(wParam))
		return 0

	case wmApp + 1:
		a.consumeRender()
		return 0

	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

// ------------------------------------------------------------------ 几何计算

func (a *App) clientRect() Rect {
	var r Rect
	procGetClientRect.Call(uintptr(a.hwnd), uintptr(unsafe.Pointer(&r)))
	return r
}

func (a *App) invalidate() {
	procInvalidateRect.Call(uintptr(a.hwnd), 0, 0)
}

func (a *App) pointInCanvas(x, y int32) bool {
	c := a.layout().canvas
	return x >= c.Left && x < c.Right && y >= c.Top && y < c.Bottom
}

// ------------------------------------------------------------------ 操作处理

func (a *App) loadImage(path string) {
	img, err := safedecode.Decode(path)
	if err != nil {
		if a.imgBmp != 0 {
			freeImage(a.imgBmp)
			a.imgBmp = 0
		}
		if a.imgHBM != 0 {
			procDeleteObject.Call(uintptr(a.imgHBM))
			a.imgHBM = 0
		}
		a.img = nil
		a.imgPath = ""
		a.art = nil
		a.status = ""
		a.errMsg = err.Error()
		a.setTitle("")
		a.invalidate()
		return
	}

	if a.imgBmp != 0 {
		freeImage(a.imgBmp)
		a.imgBmp = 0
	}
	if a.imgHBM != 0 {
		procDeleteObject.Call(uintptr(a.imgHBM))
		a.imgHBM = 0
	}
	a.imgBmp, a.imgHBM = bitmapFromRGBA(img.Pix, img.Width, img.Height)

	a.img = img
	a.imgPath = path
	a.errMsg = ""
	a.scrollX, a.scrollY = 0, 0
	a.setTitle(filepath.Base(path))
	a.requestRender()
}

func (a *App) setTitle(name string) {
	title := windowTitle
	if name != "" {
		title = name + " — " + windowTitle
	}
	procSetWindowTextW.Call(uintptr(a.hwnd), uintptr(unsafe.Pointer(utf16Ptr(title))))
}

func (a *App) openDialog() {
	path, ok := a.showOpenDialog()
	if !ok {
		return
	}
	a.loadImage(path)
}

func (a *App) exportDialog() {
	if a.art == nil {
		return
	}
	path, ok := a.showSaveDialog()
	if !ok {
		return
	}
	if artio.SameTarget(path, a.imgPath) {
		a.errMsg = "导出路径不能与输入图片相同"
		a.invalidate()
		return
	}
	content := a.artText()
	if _, err := artio.WriteFile(path, []byte(content)); err != nil {
		a.errMsg = err.Error()
		a.invalidate()
		return
	}
	a.errMsg = ""
	a.status = a.colorStateLabel() + " · 已导出到 " + path
	a.invalidate()
}

// artText 生成当前字符画的文本，供导出文件与复制到剪贴板使用。
// 「彩色」开关同时决定预览、导出和复制是否带颜色，开启时文本带有 24 位 ANSI 颜色转义序列，关闭时是纯文本。
func (a *App) artText() string {
	if a.art == nil {
		return ""
	}
	return ansi.String(a.art, a.colorOn)
}

// colorStateLabel 返回颜色状态标签，说明导出或复制的内容是否带有颜色转义序列。
// 标签放在状态栏最前面。
func (a *App) colorStateLabel() string {
	if a.colorOn {
		return "含 ANSI 颜色"
	}
	return "纯文本"
}

// copyToClipboard 把当前字符画复制到剪贴板。
func (a *App) copyToClipboard() {
	if a.art == nil {
		return
	}
	text := a.artText()
	utf, err := syscall.UTF16FromString(text)
	if err != nil {
		a.errMsg = "文本包含空字符（U+0000），无法复制到剪贴板"
		a.invalidate()
		return
	}
	size := uintptr(len(utf) * 2)

	h, _, _ := procGlobalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		a.errMsg = "无法分配剪贴板内存"
		a.invalidate()
		return
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		a.errMsg = "无法锁定剪贴板内存"
		a.invalidate()
		return
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(p)), len(utf)), utf)
	procGlobalUnlock.Call(h)

	if r, _, _ := procOpenClipboard.Call(uintptr(a.hwnd)); r == 0 {
		procGlobalFree.Call(h)
		a.errMsg = "无法打开剪贴板"
		a.invalidate()
		return
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()
	if r, _, _ := procSetClipboardData.Call(cfUnicodeText, h); r == 0 {
		procGlobalFree.Call(h)
		a.errMsg = "写入剪贴板失败"
		a.invalidate()
		return
	}

	a.errMsg = ""
	a.status = a.colorStateLabel() + " · 已复制到剪贴板"
	a.invalidate()
}

func (a *App) onDrop(h HDROP) {
	count, _, _ := procDragQueryFileW.Call(uintptr(h), 0xFFFFFFFF, 0, 0)
	defer procDragFinish.Call(uintptr(h))
	if count == 0 {
		return
	}
	buf := make([]uint16, 1024)
	n, _, _ := procDragQueryFileW.Call(uintptr(h), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return
	}
	if n > uintptr(len(buf)) {
		n = uintptr(len(buf))
	}
	a.loadImage(syscall.UTF16ToString(buf[:n]))
}

func (a *App) onKey(vk uintptr) bool {
	// 缩放输入框处于编辑状态时，键盘按键交给它处理。
	if a.zoomEdit {
		switch vk {
		case 0x0D: // 回车键
			a.commitZoomEdit()
			return true
		case 0x1B: // Esc 键
			a.cancelZoomEdit()
			return true
		}
		return false
	}

	ctrl := false
	if s, _, _ := procGetKeyState.Call(0x11); int16(s) < 0 { // VK_CONTROL
		ctrl = true
	}
	switch vk {
	case 0x4F: // O 键
		if ctrl {
			a.openDialog()
			return true
		}
	case 0x53: // S 键
		if ctrl {
			a.exportDialog()
			return true
		}
	case 0x43: // C 键
		if ctrl {
			a.copyToClipboard()
			return true
		}
	case 0x25: // 左方向键
		a.setWidth(a.opts.Width - 2)
		return true
	case 0x27: // 右方向键
		a.setWidth(a.opts.Width + 2)
		return true
	}
	return false
}

func (a *App) setWidth(w int) {
	// 滑块的范围是 20 到 240，方向键也保持在同一范围内，使滑块手柄不会超出滑槽。
	if w < sliderMinWidth {
		w = sliderMinWidth
	}
	if w > sliderMaxWidth {
		w = sliderMaxWidth
	}
	if w == a.opts.Width {
		return
	}
	a.opts.Width = w
	a.scheduleRender()
}

func (a *App) setMode(m render.Mode) {
	if a.mode == m {
		return
	}
	a.mode = m
	a.scrollX, a.scrollY = 0, 0
	a.requestRender()
}

// -------------------------------------------------------------------- 渲染

// requestRender 启动一次渲染。已有渲染在进行时，只记下还有一次待渲染，等这次结束后再启动。
// 通过请求序号丢弃过期的结果，而不中断进行中的渲染。
func (a *App) requestRender() {
	if a.img == nil {
		a.art = nil
		a.status = ""
		a.invalidate()
		return
	}
	a.seq++
	seq := a.seq
	img := a.img
	mode := a.mode
	opts := a.opts

	a.busyMu.Lock()
	if a.rendering {
		a.renderPending = true
		a.busyMu.Unlock()
		return
	}
	a.rendering = true
	a.busyMu.Unlock()

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				a.mu.Lock()
				a.result = &renderResult{seq: seq, err: fmt.Errorf("渲染失败：%v", rec)}
				a.mu.Unlock()
				procPostMessageW.Call(uintptr(a.hwnd), wmApp+1, 0, 0)
			}
		}()
		art, err := render.Render(img, mode, opts)
		a.mu.Lock()
		a.result = &renderResult{seq: seq, art: art, err: err}
		a.mu.Unlock()
		procPostMessageW.Call(uintptr(a.hwnd), wmApp+1, 0, 0)
	}()
}

// scheduleRender 延迟一小段时间再启动渲染。
// 拖动滑块时即使频繁触发，也只在停下后启动一次渲染，防止连续进行大量的渲染。
func (a *App) scheduleRender() {
	procSetTimer.Call(uintptr(a.hwnd), timerDebounce, 110, 0)
}

func (a *App) consumeRender() {
	a.mu.Lock()
	res := a.result
	a.result = nil
	a.mu.Unlock()

	a.busyMu.Lock()
	a.rendering = false
	renderPending := a.renderPending
	a.renderPending = false
	a.busyMu.Unlock()

	if res != nil && res.seq == a.seq {
		if res.err != nil {
			a.errMsg = res.err.Error()
			a.art = nil
		} else {
			a.art = res.art
			a.errMsg = ""
			a.status = fmt.Sprintf("%d×%d %s · %d×%d 单元格 · %s",
				res.art.SourceWidth, res.art.SourceHeight, res.art.Format,
				res.art.Cols, res.art.Rows, formatDuration(res.art.Elapsed))
		}
		a.scrollX, a.scrollY = 0, 0
		a.invalidate()
	}
	if renderPending {
		a.requestRender()
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return "<1 ms"
	}
	return fmt.Sprintf("%d ms", d.Milliseconds())
}

// ------------------------------------------------------------------- 对话框

func utf16List(parts ...string) []uint16 {
	var out []uint16
	for _, p := range parts {
		s, err := syscall.UTF16FromString(p)
		if err != nil {
			continue
		}
		out = append(out, s...)
	}
	return append(out, 0)
}

func (a *App) showOpenDialog() (string, bool) {
	buf := make([]uint16, 1024)
	filter := utf16List(
		"图片文件 (*.png;*.jpg;*.jpeg;*.gif)", "*.png;*.jpg;*.jpeg;*.gif",
		"所有文件 (*.*)", "*.*",
	)
	ofn := OpenFileName{
		LStructSize: DWORD(unsafe.Sizeof(OpenFileName{})),
		HwndOwner:   a.hwnd,
		LpstrFilter: &filter[0],
		LpstrFile:   &buf[0],
		NMaxFile:    DWORD(len(buf)),
		LpstrTitle:  utf16Ptr("打开图片"),
		Flags:       ofnFileMustExist | ofnPathMustExist | ofnExplorer | ofnNoChangeDir,
		LpstrDefExt: utf16Ptr("png"),
	}
	r, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

func (a *App) showSaveDialog() (string, bool) {
	buf := make([]uint16, 1024)
	def := "art.txt"
	if a.imgPath != "" {
		stem := strings.TrimSuffix(filepath.Base(a.imgPath), filepath.Ext(a.imgPath))
		if stem != "" {
			def = stem + ".txt"
		}
	}
	if u, err := syscall.UTF16FromString(def); err == nil && len(u) <= len(buf) {
		copy(buf, u)
	}
	// 对话框标题写明这次保存的内容是否带颜色转义序列，用户在保存前就能看到。
	title := "导出为纯文本"
	if a.colorOn {
		title = "导出为带 ANSI 颜色的文本"
	}
	filter := utf16List(
		"文本文件 (*.txt)", "*.txt",
		"所有文件 (*.*)", "*.*",
	)
	ofn := OpenFileName{
		LStructSize: DWORD(unsafe.Sizeof(OpenFileName{})),
		HwndOwner:   a.hwnd,
		LpstrFilter: &filter[0],
		LpstrFile:   &buf[0],
		NMaxFile:    DWORD(len(buf)),
		LpstrTitle:  utf16Ptr(title),
		Flags:       ofnOverwritePrompt | ofnPathMustExist | ofnExplorer | ofnNoChangeDir,
		LpstrDefExt: utf16Ptr("txt"),
	}
	r, _, _ := procGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

const cwUseDefault = ^uintptr(0x7FFFFFFF)
