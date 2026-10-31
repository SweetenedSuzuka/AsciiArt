// artio 包把生成的文本写入磁盘，并处理写入过程中可能出现的各种出错情况。
// 每次写入都先写到目标目录下的临时文件，再改名替换目标文件，这样运行失败时不会留下写了一半的文件。
// 不是普通文件的目标（符号链接、目录、Windows 保留设备名（如 NUL、COM1））在打开任何句柄之前就会被拒绝。
package artio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// reservedNames 是 Win32 无论在哪个目录下都会解析的 DOS 设备名。
// 向这些名字写入数据会导致进程卡住或者控制台损坏，因此在打开任何句柄之前就按名字拒绝它们。
var reservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// CheckTarget 校验目标路径，除查询属性外不接触文件系统。
// 它被导出，供图形界面在显示对话框之前做校验。
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
	stem := base
	if i := strings.IndexByte(stem, '.'); i >= 0 {
		stem = stem[:i]
	}
	if reservedNames[strings.ToUpper(strings.TrimSpace(stem))] {
		return fmt.Errorf("%q 是 Windows 保留设备名，不能作为文件名", base)
	}

	dir := filepath.Dir(clean)
	st, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("目录不可用：%w", err)
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
			return errors.New("目标是一个目录")
		}
		if !linkInfo.Mode().IsRegular() {
			return errors.New("目标不是普通文件")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("无法检查目标：%w", err)
	}
	return nil
}

// WriteFile 以独立的方式（即通常所说的原子操作，我不喜欢这种命名）把 data 写入 path 并替换原文件。
// 内容先写入目标目录下的临时文件，再改名到目标位置。
// 在 NTFS 和 POSIX 文件系统上改名都是独立的不可分割操作，读取时要么是旧文件，要么是完整的新文件，不会出现生成一半的字符画。
// 失败时临时文件会被删除。
func WriteFile(path string, data []byte) (string, error) {
	if err := CheckTarget(path); err != nil {
		return "", err
	}
	clean := filepath.Clean(path)
	dir := filepath.Dir(clean)

	tmp, err := os.CreateTemp(dir, ".asciiart-*.tmp")
	if err != nil {
		return "", fmt.Errorf("无法创建临时文件：%w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		// 改名成功时无操作，失败时删除临时文件。
		os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("写入失败：%w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", fmt.Errorf("刷新失败：%w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("关闭失败：%w", err)
	}
	_ = os.Chmod(tmpName, 0o644)

	if err := os.Rename(tmpName, clean); err != nil {
		return "", fmt.Errorf("替换目标文件失败：%w", err)
	}

	abs, err := filepath.Abs(clean)
	if err != nil {
		return clean, nil
	}
	return abs, nil
}

// Suffixed 根据用户给定的基础文件名和模式名生成带后缀的文件名，例如“out.txt”加上 ascii 得到“out_ascii.txt”。
// 一次运行产出多个文件时使用这个函数，使各文件互不冲突。
func Suffixed(path, suffix string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	return base + "_" + suffix + ext
}
