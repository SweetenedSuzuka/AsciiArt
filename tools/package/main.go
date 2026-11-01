// 把发布用的文件打包成 zip 压缩包，并写出 SHA256 校验值。
//
// 程序只使用 Go 标准库，编译后不依赖外部程序。
// 由于 Git Bash、PowerShell 与持续集成环境不一定都提供 zip、7z 或 Compress-Archive，因此打包逻辑直接用 Go 实现。
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func main() {
	version := flag.String("version", "dev", "压缩包名称中使用的版本号")
	dist := flag.String("dist", "dist", "存放构建产物的目录")
	outDir := flag.String("out", "dist", "存放压缩包的目录")
	flag.Parse()

	if err := checkVersion(*version); err != nil {
		fail(err)
	}

	archiveName := fmt.Sprintf("asciiart-%s-windows-amd64", *version)
	zipPath := filepath.Join(*outDir, archiveName+".zip")

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fail(err)
	}

	f, err := os.Create(zipPath)
	if err != nil {
		fail(err)
	}
	zw := zip.NewWriter(f)

	entries := []struct {
		src  string
		name string
		mode os.FileMode
	}{
		{filepath.Join(*dist, "asciiart.exe"), "asciiart.exe", 0o755},
		{filepath.Join(*dist, "asciiart-gui.exe"), "asciiart-gui.exe", 0o755},
		{"LICENSE", "LICENSE", 0o644},
	}
	for _, e := range entries {
		if err := addFile(zw, e.src, e.name, e.mode); err != nil {
			zw.Close()
			f.Close()
			fail(err)
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		fail(err)
	}
	if err := f.Close(); err != nil {
		fail(err)
	}

	sum, err := hashFile(zipPath)
	if err != nil {
		fail(err)
	}
	sumPath := filepath.Join(*outDir, "SHA256SUMS.txt")
	line := fmt.Sprintf("%s  %s\n", sum, filepath.Base(zipPath))
	if err := os.WriteFile(sumPath, []byte(line), 0o644); err != nil {
		fail(err)
	}

	fmt.Printf("打包完成：%s\n", zipPath)
	fmt.Printf("校验值：  %s\n", sum)
}

func addFile(zw *zip.Writer, src, name string, mode os.FileMode) error {
	content, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) && mode == 0o644 {
			return nil // 权限为 0o644 的条目按可选文件处理，缺失时跳过。
		}
		return fmt.Errorf("%s: %w", src, err)
	}
	h := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()}
	h.SetMode(mode)
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	_, err = w.Write(content)
	return err
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil))), nil
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "打包失败：%v\n", err)
	os.Exit(1)
}

// versionPattern 限定版本号可以使用的字符，防止版本号里的斜杠等字符把压缩包写到别的目录。
var versionPattern = regexp.MustCompile(`^[0-9A-Za-z._+-]+$`)

func checkVersion(v string) error {
	if !versionPattern.MatchString(v) {
		return fmt.Errorf("版本号只允许字母、数字、点、加号、下划线和连字符，目前值为「%s」", v)
	}
	return nil
}
