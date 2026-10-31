package safedecode

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"asciiart/internal/limits"
)

func pngBytes(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 7), uint8(y * 11), 200, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func writePNG(t *testing.T, dir string, w, h int) string {
	t.Helper()
	path := filepath.Join(dir, "sample.png")
	if err := os.WriteFile(path, pngBytes(w, h), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDecodePNG(t *testing.T) {
	dir := t.TempDir()
	path := writePNG(t, dir, 32, 16)

	img, err := Decode(path)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if img.Width != 32 || img.Height != 16 {
		t.Fatalf("size = %dx%d, want 32x16", img.Width, img.Height)
	}
	if img.Format != FormatPNG {
		t.Fatalf("format = %s, want PNG", img.Format)
	}
	if len(img.Pix) != 32*16*4 {
		t.Fatalf("pixel buffer = %d bytes, want %d", len(img.Pix), 32*16*4)
	}
	if img.Pix[3] != 255 {
		t.Fatalf("alpha = %d, want 255", img.Pix[3])
	}
}

func TestDecodeRejectsNonImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("this is definitely not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Decode(path)
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("err = %v, want ErrUnsupportedFormat", err)
	}
}

func TestDecodeRejectsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.png")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(path); err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestDecodeRejectsDirectory(t *testing.T) {
	if _, err := Decode(t.TempDir()); err == nil {
		t.Fatal("expected error for directory")
	}
}

func TestDecodeRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := writePNG(t, dir, 8, 8)
	link := filepath.Join(dir, "link.png")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Decode(link); err == nil {
		t.Fatal("expected error for symlink")
	}
}

// pngHeader 按给定的宽高构造 PNG 签名和 IHDR 数据块。
func pngHeader(w, h uint32) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})

	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], w)
	binary.BigEndian.PutUint32(ihdr[4:8], h)
	ihdr[8] = 8 // 位深度
	ihdr[9] = 6 // 颜色类型 RGBA
	// 压缩方式、滤镜方式、隔行扫描均为零。

	chunk := append([]byte("IHDR"), ihdr...)
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(ihdr)))
	buf.Write(length[:])
	buf.Write(chunk)
	var crc [4]byte
	binary.BigEndian.PutUint32(crc[:], crc32.ChecksumIEEE(chunk))
	buf.Write(crc[:])
	return buf.Bytes()
}

// TestDecodeRejectsOversizedHeader 验证头部声明超大尺寸的文件会被拒绝。
func TestDecodeRejectsOversizedHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oversized.png")
	if err := os.WriteFile(path, pngHeader(50000, 50000), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Decode(path)
	if err == nil {
		t.Fatal("expected the header dimensions to be rejected")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("像素")) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckPixelCount(t *testing.T) {
	if err := limits.CheckPixelCount(1920, 1080); err != nil {
		t.Fatalf("normal image rejected: %v", err)
	}
	if err := limits.CheckPixelCount(0, 10); err == nil {
		t.Fatal("zero width accepted")
	}
	if err := limits.CheckPixelCount(100000, 10); err == nil {
		t.Fatal("oversized side accepted")
	}
	if err := limits.CheckPixelCount(8000, 8000); err == nil {
		t.Fatal("64 MP accepted")
	}
}

// FuzzDecode 断言任意字节序列都不会让解码器崩溃、卡住，也不会返回超过配置上限的图像。
func FuzzDecode(f *testing.F) {
	f.Add(pngBytes(8, 8))
	f.Add(pngHeader(4, 4))
	f.Add([]byte("GIF89a"))
	f.Add([]byte{0xff, 0xd8, 0xff, 0xe0})

	f.Fuzz(func(t *testing.T, raw []byte) {
		path := filepath.Join(t.TempDir(), "fuzz.bin")
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Skip()
		}
		img, err := Decode(path)
		if err != nil {
			return
		}
		if int64(img.Width)*int64(img.Height) > limits.MaxPixels {
			t.Fatalf("decoder exceeded the pixel budget: %dx%d", img.Width, img.Height)
		}
		if len(img.Pix) != img.Width*img.Height*4 {
			t.Fatalf("buffer size mismatch: %d vs %dx%d", len(img.Pix), img.Width, img.Height)
		}
	})
}
