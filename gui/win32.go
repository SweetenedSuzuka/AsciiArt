// Package gui 实现 AsciiArt 的桌面应用程序。
// 窗口界面全部用 GDI+ 手工绘制，采用 KDE Breeze 设计语言。程序不依赖第三方库：user32、gdi32、gdiplus、comdlg32 和 shell32 都通过 syscall 调用。
// 编译产物因此是单个独立文件，涉及外部调用的只有本文件中列出的这些函数。
package gui

import (
	"syscall"
)

type (
	HWND      uintptr
	HDC       uintptr
	HMENU     uintptr
	HINSTANCE uintptr
	HBITMAP   uintptr
	HGDIOBJ   uintptr
	HFONT     uintptr
	HPEN      uintptr
	HBRUSH    uintptr
	HICON     uintptr
	HCURSOR   uintptr
	HRGN      uintptr
	HMONITOR  uintptr
	HDROP     uintptr
	HANDLE    uintptr
	HLOCAL    uintptr
	ATOM      uint16
	BOOL      int32
	DWORD     uint32
	UINT      uint32
	WPARAM    uintptr
	LPARAM    uintptr
	LRESULT   uintptr
	COLORREF  uint32
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
)

var (
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procShowWindow          = user32.NewProc("ShowWindow")
	procUpdateWindow        = user32.NewProc("UpdateWindow")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procSendMessageW        = user32.NewProc("SendMessageW")
	procSetWindowTextW      = user32.NewProc("SetWindowTextW")
	procGetClientRect       = user32.NewProc("GetClientRect")
	procInvalidateRect      = user32.NewProc("InvalidateRect")
	procBeginPaint          = user32.NewProc("BeginPaint")
	procEndPaint            = user32.NewProc("EndPaint")
	procSetTimer            = user32.NewProc("SetTimer")
	procKillTimer           = user32.NewProc("KillTimer")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procSetCursor           = user32.NewProc("SetCursor")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procSetCapture          = user32.NewProc("SetCapture")
	procReleaseCapture      = user32.NewProc("ReleaseCapture")
	procTrackMouseEvent     = user32.NewProc("TrackMouseEvent")
	procSetProcessDPIAware  = user32.NewProc("SetProcessDPIAware")
	procGetDpiForWindow     = user32.NewProc("GetDpiForWindow")
	procOpenClipboard       = user32.NewProc("OpenClipboard")
	procEmptyClipboard      = user32.NewProc("EmptyClipboard")
	procSetClipboardData    = user32.NewProc("SetClipboardData")
	procCloseClipboard      = user32.NewProc("CloseClipboard")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procFillRect            = user32.NewProc("FillRect")
	procMessageBoxW         = user32.NewProc("MessageBoxW")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetKeyState         = user32.NewProc("GetKeyState")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procMonitorFromWindow   = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")

	procCreateCompatibleDC    = gdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection      = gdi32.NewProc("CreateDIBSection")
	procSelectObject          = gdi32.NewProc("SelectObject")
	procDeleteObject          = gdi32.NewProc("DeleteObject")
	procDeleteDC              = gdi32.NewProc("DeleteDC")
	procBitBlt                = gdi32.NewProc("BitBlt")
	procSetBkMode             = gdi32.NewProc("SetBkMode")
	procSetTextColor          = gdi32.NewProc("SetTextColor")
	procExtTextOutW           = gdi32.NewProc("ExtTextOutW")
	procGetTextExtentPoint32W = gdi32.NewProc("GetTextExtentPoint32W")
	procGetTextMetricsW       = gdi32.NewProc("GetTextMetricsW")
	procCreateFontW           = gdi32.NewProc("CreateFontW")
	procGetGlyphIndicesW      = gdi32.NewProc("GetGlyphIndicesW")
	procCreateSolidBrush      = gdi32.NewProc("CreateSolidBrush")
	procSetBkColor            = gdi32.NewProc("SetBkColor")
	procIntersectClipRect     = gdi32.NewProc("IntersectClipRect")
	procSelectClipRgn         = gdi32.NewProc("SelectClipRgn")

	procGlobalAlloc  = kernel32.NewProc("GlobalAlloc")
	procGlobalLock   = kernel32.NewProc("GlobalLock")
	procGlobalUnlock = kernel32.NewProc("GlobalUnlock")

	procGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")
	procGetSaveFileNameW = comdlg32.NewProc("GetSaveFileNameW")

	procDragAcceptFiles = shell32.NewProc("DragAcceptFiles")
	procDragQueryFileW  = shell32.NewProc("DragQueryFileW")
	procDragFinish      = shell32.NewProc("DragFinish")
	procShellExecuteW   = shell32.NewProc("ShellExecuteW")

	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")

	procCoInitializeEx = ole32.NewProc("CoInitializeEx")
)

// Point、Rect 及相关结构体与 Win32 的结构体逐字段对应。Go 插入的填充字节与 C 编译器一致，因此这些结构体可以直接按指针传递。
type Point struct{ X, Y int32 }

type Rect struct{ Left, Top, Right, Bottom int32 }

func (r Rect) Width() int32  { return r.Right - r.Left }
func (r Rect) Height() int32 { return r.Bottom - r.Top }

type Size struct{ CX, CY int32 }

type Msg struct {
	Hwnd    HWND
	Message UINT
	WParam  WPARAM
	LParam  LPARAM
	Time    DWORD
	Pt      Point
}

type PaintStruct struct {
	Hdc         HDC
	FErase      BOOL
	RcPaint     Rect
	FRestore    BOOL
	FIncUpdate  BOOL
	RgbReserved [32]byte
}

type WndClassEx struct {
	CbSize        UINT
	Style         UINT
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     HINSTANCE
	HIcon         HICON
	HCursor       HCURSOR
	HbrBackground HBRUSH
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       HICON
}

type BitmapInfoHeader struct {
	BiSize          DWORD
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   DWORD
	BiSizeImage     DWORD
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       DWORD
	BiClrImportant  DWORD
}

type RGBQuad struct{ Blue, Green, Red, Reserved byte }

type BitmapInfo struct {
	Header BitmapInfoHeader
	Colors [1]RGBQuad
}

type TextMetric struct {
	TmHeight           int32
	TmAscent           int32
	TmDescent          int32
	TmInternalLeading  int32
	TmExternalLeading  int32
	TmAveCharWidth     int32
	TmMaxCharWidth     int32
	TmWeight           int32
	TmOverhang         int32
	TmDigitizedAspectX int32
	TmDigitizedAspectY int32
	TmFirstChar        uint16
	TmLastChar         uint16
	TmDefaultChar      uint16
	TmBreakChar        uint16
	TmItalic           byte
	TmUnderlined       byte
	TmStruckOut        byte
	TmPitchAndFamily   byte
	TmCharSet          byte
}

type OpenFileName struct {
	LStructSize       DWORD
	HwndOwner         HWND
	HInstance         HINSTANCE
	LpstrFilter       *uint16
	LpstrCustomFilter *uint16
	NMaxCustFilter    DWORD
	NFilterIndex      DWORD
	LpstrFile         *uint16
	NMaxFile          DWORD
	LpstrFileTitle    *uint16
	NMaxFileTitle     DWORD
	LpstrInitialDir   *uint16
	LpstrTitle        *uint16
	Flags             DWORD
	NFileOffset       uint16
	NFileExtension    uint16
	LpstrDefExt       *uint16
	LCustData         LPARAM
	LpfnHook          uintptr
	LpTemplateName    *uint16
	PvReserved        uintptr
	DwReserved        DWORD
	FlagsEx           DWORD
}

type MonitorInfo struct {
	CbSize    DWORD
	RcMonitor Rect
	RcWork    Rect
	DwFlags   DWORD
}

type MinMaxInfo struct {
	PtReserved     Point
	PtMaxSize      Point
	PtMaxPosition  Point
	PtMinTrackSize Point
	PtMaxTrackSize Point
}

type TrackMouseEvent struct {
	CbSize      DWORD
	DwFlags     DWORD
	HwndTrack   HWND
	DwHoverTime DWORD
}

// 窗口样式与消息常量。
const (
	wsOverlappedWindow = 0x00CF0000
	wsVisible          = 0x10000000
	csHRedraw          = 0x0002
	csVRedraw          = 0x0001
	swShow             = 5

	wmDestroy        = 0x0002
	wmSize           = 0x0005
	wmSetFocus       = 0x0007
	wmKillFocus      = 0x0008
	wmPaint          = 0x000F
	wmClose          = 0x0010
	wmEraseBkgnd     = 0x0014
	wmSetCursor      = 0x0020
	wmGetMinMaxInfo  = 0x0024
	wmKeyDown        = 0x0100
	wmChar           = 0x0102
	wmTimer          = 0x0113
	wmMouseMove      = 0x0200
	wmLButtonDown    = 0x0201
	wmLButtonUp      = 0x0202
	wmLButtonDblClk  = 0x0203
	wmMouseWheel     = 0x020A
	wmCaptureChanged = 0x0215
	wmDropFiles      = 0x0233
	wmMouseLeave     = 0x02A3
	wmDpiChanged     = 0x02E0
	wmApp            = 0x8000

	htClient  = 1
	mkLButton = 0x0001
	mkShift   = 0x0004
	mkControl = 0x0008

	idiApplication = 32512
	idcArrow       = 32512
	idcHand        = 32649
	idcSizeNS      = 32645

	tmeLeave = 0x0002

	cfUnicodeText = 13
	gmemMoveable  = 0x0042

	ofnOverwritePrompt = 0x00000002
	ofnPathMustExist   = 0x00000800
	ofnFileMustExist   = 0x00001000
	ofnNoChangeDir     = 0x00000008
	ofnExplorer        = 0x00080000

	dwmwaUseImmersiveDarkMode = 20

	biRGB        = 0
	dibRGBColors = 0

	srcCopy     = 0x00CC0020
	transparent = 1
	antialiased = 4
	etoOpaque   = 0x0002

	fwNormal   = 400
	fwDemiBold = 600
	fwBold     = 700

	defaultCharset    = 1
	outTTPrecis       = 4
	clipDefaultPrecis = 0
	fixedPitch        = 1
	ffModern          = 48
	ffSwiss           = 32

	ggiMarkNonExisting = 0x0001

	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
	swpNoMove     = 0x0002
	swpNoSize     = 0x0001
)

func u16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}
	return p
}

func loword(v uintptr) int32 { return int32(v & 0xffff) }

func hiword(v uintptr) int32 { return int32((v >> 16) & 0xffff) }

func getX(l uintptr) int32 { return int32(int16(l & 0xffff)) }

func getY(l uintptr) int32 { return int32(int16((l >> 16) & 0xffff)) }

func rgb(r, g, b byte) COLORREF {
	return COLORREF(uint32(r) | uint32(g)<<8 | uint32(b)<<16)
}

// iptr 把有符号整数转换成 Win32 需要的 uintptr 形式，并保留补码的位排列。
// CreateFontW 的高度参数传入负数时表示“字符高度”而不是“单元格高度”，而负数字面量无法在编译期转换成 uintptr。
func iptr(v int32) uintptr { return uintptr(v) }

// callOptional 调用延迟加载的 proc，当导出函数不存在时返回零值而不是触发panic。旧版 Windows 可能缺少这里用到的部分函数（GetDpiForWindow、DwmSetWindowAttribute）。
func callOptional(proc *syscall.LazyProc, args ...uintptr) uintptr {
	if err := proc.Find(); err != nil {
		return 0
	}
	r, _, _ := proc.Call(args...)
	return r
}
