package auth

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRowStringAndBytes(t *testing.T) {
	row := map[string]any{
		"str":   "hello",
		"bytes": []byte("world"),
		"int":   int64(42),
		"nil":   nil,
	}

	if got := rowString(row, "str"); got != "hello" {
		t.Errorf("rowString(str) = %q", got)
	}
	if got := rowString(row, "bytes"); got != "world" {
		t.Errorf("rowString(bytes) = %q", got)
	}
	if got := rowString(row, "int"); got != "" {
		t.Errorf("rowString(int) = %q, want empty", got)
	}
	if got := rowString(row, "nil"); got != "" {
		t.Errorf("rowString(nil) = %q, want empty", got)
	}

	if got := rowBytes(row, "bytes"); !bytes.Equal(got, []byte("world")) {
		t.Errorf("rowBytes(bytes) = %q", got)
	}
	if got := rowBytes(row, "str"); !bytes.Equal(got, []byte("hello")) {
		t.Errorf("rowBytes(str) = %q", got)
	}
	if got := rowBytes(row, "int"); got != nil {
		t.Errorf("rowBytes(int) = %v, want nil", got)
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "payload" {
		t.Errorf("copied = %q, err = %v", got, err)
	}

	if err := copyFile(filepath.Join(dir, "missing"), dst); err == nil {
		t.Error("expected an error copying a missing source")
	}
}
