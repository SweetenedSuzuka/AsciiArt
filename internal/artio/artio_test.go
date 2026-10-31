package artio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckTargetRejectsReservedNames(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"NUL", "nul.txt", "CON", "aux.log", "COM1.txt", "lpt9"} {
		if err := CheckTarget(filepath.Join(dir, name)); err == nil {
			t.Errorf("%q should be rejected as a reserved device name", name)
		}
	}
}

func TestCheckTargetRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := CheckTarget(dir); err == nil {
		t.Error("directory should be rejected")
	}
	if err := CheckTarget(dir + string(os.PathSeparator)); err == nil {
		t.Error("trailing separator should be rejected")
	}
}

func TestCheckTargetRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := CheckTarget(link); err == nil {
		t.Fatal("symlink should be rejected")
	}
}

func TestWriteFileCreatesAndReplaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	abs, err := WriteFile(path, []byte("first"))
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if !filepath.IsAbs(abs) {
		t.Fatalf("returned path is not absolute: %s", abs)
	}
	if got, _ := os.ReadFile(path); string(got) != "first" {
		t.Fatalf("content = %q, want %q", got, "first")
	}

	if _, err := WriteFile(path, []byte("second")); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "second" {
		t.Fatalf("content = %q, want %q", got, "second")
	}

	// 写入成功后不能残留任何临时文件。
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".asciiart-") {
			t.Fatalf("temporary file left behind: %s", e.Name())
		}
	}
}

func TestWriteFileLeavesNoTempOnFailure(t *testing.T) {
	dir := t.TempDir()
	// 把目录当作目标会在创建临时文件之前就校验失败。
	if _, err := WriteFile(dir, []byte("x")); err == nil {
		t.Fatal("expected error")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("directory is not clean: %v", entries)
	}
}

func TestSuffixed(t *testing.T) {
	cases := map[string]string{
		"out.txt":  "out_ascii.txt",
		"out":      "out_ascii",
		"a.b.txt":  "a.b_ascii.txt",
		"dir/o.md": "dir/o_ascii.md",
	}
	for in, want := range cases {
		if got := Suffixed(in, "ascii"); got != want {
			t.Errorf("Suffixed(%q) = %q, want %q", in, got, want)
		}
	}
}
