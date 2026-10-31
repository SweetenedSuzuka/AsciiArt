//go:build !windows

package term

import "os"

// IsTerminal 返回 f 是否连接到终端。
func IsTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// EnableANSI 在 Windows 之外不执行任何操作。
// POSIX 终端本身就能处理转义序列。
func EnableANSI(f *os.File) bool { return IsTerminal(f) }
