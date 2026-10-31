//go:build windows

package term

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
)

const enableVirtualTerminalProcessing = 0x0004

// IsTerminal 返回 f 是否连接到终端。
func IsTerminal(f *os.File) bool {
	var mode uint32
	r, _, _ := procGetConsoleMode.Call(f.Fd(), uintptr(unsafe.Pointer(&mode)))
	return r != 0
}

// EnableANSI 为 f 连接的终端开启虚拟终端转义序列处理。
// Windows 10 1511 起支持 24 位颜色转义序列，但需要先开启对应的控制台模式。
// 如果不开启，颜色输出会按字面字符打印成“[38;2;...”这样的内容。
// 当 f 是管道或文件时该调用不生效，这正是输出被重定向时需要的行为。
func EnableANSI(f *os.File) bool {
	var mode uint32
	r, _, _ := procGetConsoleMode.Call(f.Fd(), uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		return false
	}
	if mode&enableVirtualTerminalProcessing != 0 {
		return true
	}
	mode |= enableVirtualTerminalProcessing
	r, _, _ = kernel32.NewProc("SetConsoleMode").Call(f.Fd(), uintptr(mode))
	return r != 0
}
