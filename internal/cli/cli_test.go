package cli

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestImage(t *testing.T, dir string) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 6), uint8(y * 12), 90, 255})
		}
	}
	path := filepath.Join(dir, "in.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = Run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestNoArgumentsIsUsageError(t *testing.T) {
	code, _, stderr := run(t)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "缺少图片路径") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestUnknownFlag(t *testing.T) {
	code, _, _ := run(t, "--nope")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestWidthOutOfRange(t *testing.T) {
	dir := t.TempDir()
	img := writeTestImage(t, dir)
	code, _, stderr := run(t, img, "-w", "100000")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (%s)", code, stderr)
	}
}

func TestMissingFileIsRuntimeError(t *testing.T) {
	code, _, stderr := run(t, filepath.Join(t.TempDir(), "nope.png"))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (%s)", code, stderr)
	}
}

func TestRenderToStdout(t *testing.T) {
	dir := t.TempDir()
	img := writeTestImage(t, dir)
	code, stdout, stderr := run(t, img, "-m", "ascii", "-w", "20", "--no-color")
	if code != 0 {
		t.Fatalf("exit code = %d (%s)", code, stderr)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Fatal("no output produced")
	}
	if strings.ContainsRune(stdout, 0x1b) {
		t.Fatal("--no-color still emitted escape sequences")
	}
}

// TestFlagsAfterPositional 用于验证参数重排是否正常工作。
// 如果不做重排，Go 的 flag 包会把“-w 20”当作位置参数。
func TestFlagsAfterPositional(t *testing.T) {
	dir := t.TempDir()
	img := writeTestImage(t, dir)
	code, stdout, stderr := run(t, img, "-m", "ascii", "-w", "20", "--no-color")
	if code != 0 {
		t.Fatalf("exit code = %d (%s)", code, stderr)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5 (40x20 at width 20 -> 20x5)", len(lines))
	}
}

func TestOutputFileSingleMode(t *testing.T) {
	dir := t.TempDir()
	img := writeTestImage(t, dir)
	out := filepath.Join(dir, "art.txt")

	code, _, stderr := run(t, img, "-m", "blocks", "-w", "10", "-o", out)
	if code != 0 {
		t.Fatalf("exit code = %d (%s)", code, stderr)
	}
	content, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if !strings.Contains(string(content), "▀") {
		t.Fatalf("blocks output has no half-block glyph: %q", content)
	}
}

func TestOutputFileAllModesGetsSuffixes(t *testing.T) {
	dir := t.TempDir()
	img := writeTestImage(t, dir)
	base := filepath.Join(dir, "art.txt")

	code, _, stderr := run(t, img, "-m", "all", "-w", "10", "-o", base)
	if code != 0 {
		t.Fatalf("exit code = %d (%s)", code, stderr)
	}
	for _, suffix := range []string{"ascii", "braille", "blocks"} {
		want := filepath.Join(dir, "art_"+suffix+".txt")
		if _, err := os.Stat(want); err != nil {
			t.Fatalf("missing %s: %v", want, err)
		}
	}
	if _, err := os.Stat(base); err == nil {
		t.Fatal("the bare base name should not have been written")
	}
}

func TestStdoutOnlyWritesNoFiles(t *testing.T) {
	dir := t.TempDir()
	img := writeTestImage(t, dir)
	before, _ := os.ReadDir(dir)

	code, _, stderr := run(t, img, "-m", "ascii", "-w", "10")
	if code != 0 {
		t.Fatalf("exit code = %d (%s)", code, stderr)
	}
	after, _ := os.ReadDir(dir)
	if len(after) != len(before) {
		t.Fatalf("a stdout-only run created files: %d -> %d", len(before), len(after))
	}
}

// TestOutputDirectoryRejected 验证一类容易忽略的情况。
// 指定多个模式时基础文件名会加后缀，“dist”会变成“dist_ascii”，并在用户意图所在的目录旁边静默写入成功。
func TestOutputDirectoryRejected(t *testing.T) {
	dir := t.TempDir()
	img := writeTestImage(t, dir)
	code, _, stderr := run(t, img, "-o", dir)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (%s)", code, stderr)
	}
	parent := filepath.Dir(dir)
	entries, _ := os.ReadDir(parent)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), filepath.Base(dir)+"_") {
			t.Fatalf("wrote %q next to the target directory", e.Name())
		}
	}
}

func TestBadModeIsUsageError(t *testing.T) {
	dir := t.TempDir()
	img := writeTestImage(t, dir)
	code, _, _ := run(t, img, "-m", "klingon")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestThresholdOutOfRange(t *testing.T) {
	dir := t.TempDir()
	img := writeTestImage(t, dir)
	code, _, _ := run(t, img, "-a", "300")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestVersionAndHelp(t *testing.T) {
	if code, out, _ := run(t, "--version"); code != 0 || !strings.Contains(out, "asciiart") {
		t.Fatalf("version failed: %d %q", code, out)
	}
	if code, out, _ := run(t, "-h"); code != 0 || !strings.Contains(out, "用法") {
		t.Fatalf("help failed: %d %q", code, out)
	}
}
