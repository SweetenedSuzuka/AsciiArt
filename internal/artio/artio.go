// artio 包把生成的文本写入磁盘，并处理写入过程中可能出现的各种错误。
// 每次写入都先把内容输出到目标目录下的临时文件，再把这个临时文件重命名成目标文件，这样写入失败时不会出现输出失败的错误文件。
// 符号链接、目录和 Windows 保留设备名（如 NUL、COM1）一律禁止执行，其中保留设备名在打开任何句柄之前就按关键词阻止。
package artio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// reservedNames 是 Win32 无论在哪个目录下都会解析的 DOS 设备名。
// 这些名字会被 Win32 解析成设备，其中控制台设备和串口还会改变屏幕内容或者让写入阻塞，因此在打开任何句柄之前就按名字拒绝它们。
var reservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"CONIN$": true, "CONOUT$": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"COM¹": true, "COM²": true, "COM³": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	"LPT¹": true, "LPT²": true, "LPT³": true,
}

// CheckTarget 校验目标路径，除查询属性外不接触文件系统。
// WriteFile 在创建临时文件之前调用它，命令行与图形界面因此共用同一套目标检查。
func CheckTarget(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("输出路径为空")
	}
	clean := filepath.Clean(path)
	if strings.HasSuffix(path, string(os.PathSeparator)) || strings.HasSuffix(path, "/") {
		return errors.New("输出路径是目录")
	}

	base := filepath.Base(clean)
	if base == "." || base == string(os.PathSeparator) {
		return errors.New("输出路径是目录")
	}
	if strings.ContainsRune(base, ':') {
		return errors.New("文件名不能包含冒号（NTFS 备用数据流）")
	}
	stem := base
	if i := strings.IndexByte(stem, '.'); i >= 0 {
		stem = stem[:i]
	}
	if reservedNames[strings.ToUpper(strings.TrimSpace(stem))] {
		return fmt.Errorf("「%s」是 Windows 保留设备名，不能作为文件名", base)
	}

	dir := filepath.Dir(clean)
	st, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("无法访问目录：%w", err)
	}
	if !st.IsDir() {
		return fmt.Errorf("%s 不是目录", dir)
	}

	linkInfo, err := os.Lstat(clean)
	if err == nil {
		if linkInfo.Mode()&os.ModeSymlink != 0 {
			return errors.New("拒绝覆盖符号链接（可能指向目录外）")
		}
		if linkInfo.IsDir() {
			return errors.New("输出路径是目录")
		}
		if !linkInfo.Mode().IsRegular() {
			return errors.New("输出路径不是普通文件")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("无法检查输出路径：%w", err)
	}
	return nil
}

// WriteFile 把 data 写入 path，并替换已有的同名文件。
// 内容先写入目标目录下的临时文件，再把临时文件重命名成目标文件。
// 在 NTFS 和 POSIX 文件系统上，重命名在一次操作中完成，因此读取时要么是旧文件，要么是完整的新文件。
// 失败时临时文件会被删除。
func WriteFile(path string, data []byte) (string, error) {
	if err := CheckTarget(path); err != nil {
		return "", err
	}
	clean := filepath.Clean(path)
	dir := filepath.Dir(clean)

	// 目标已经存在时沿用它的权限，避免覆盖文件时导致权限改变。
	oldMode, existed := os.FileMode(0), false
	if info, statErr := os.Lstat(clean); statErr == nil {
		oldMode, existed = info.Mode().Perm(), true
	}

	tmp, err := os.CreateTemp(dir, ".asciiart-*.tmp")
	if err != nil {
		return "", fmt.Errorf("无法创建临时文件：%w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		// 重命名成功时无操作，失败时删除临时文件。
		os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("写入临时文件失败：%w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", fmt.Errorf("同步到磁盘失败：%w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("关闭临时文件失败：%w", err)
	}
	if existed {
		_ = os.Chmod(tmpName, oldMode)
	} else {
		_ = os.Chmod(tmpName, 0o644)
	}

	if err := os.Rename(tmpName, clean); err != nil {
		return "", fmt.Errorf("替换目标文件失败：%w", err)
	}

	abs, err := filepath.Abs(clean)
	if err != nil {
		return clean, nil
	}
	return abs, nil
}

// Suffixed 在基础文件名的扩展名之前插入后缀，例如“out.txt”加上 ascii 得到“out_ascii.txt”。
// 一次运行产出多个文件时使用这个函数，使各文件互不冲突。
func Suffixed(path, suffix string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	return base + "_" + suffix + ext
}

// SameTarget 判断两个路径是否指向同一个文件。
// 目标尚不存在时按路径比较，Windows 上不区分大小写。
func SameTarget(pathA, pathB string) bool {
	infoA, errA := os.Stat(pathA)
	infoB, errB := os.Stat(pathB)
	if errA == nil && errB == nil {
		return os.SameFile(infoA, infoB)
	}
	absA, errA := filepath.Abs(pathA)
	absB, errB := filepath.Abs(pathB)
	if errA != nil || errB != nil {
		return false
	}
	absA, absB = filepath.Clean(absA), filepath.Clean(absB)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(absA, absB)
	}
	return absA == absB
}
