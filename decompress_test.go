// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	stdgzip "compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDecompressFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	want := bytes.Repeat([]byte("hello world\n"), 1000)
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CompressFile(path, ".gz", 5, false, false, true, nil); err != nil {
		t.Fatal(err)
	}
	if err := DecompressFile(path+".gz", ".gz", false, false, false, nil); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("round trip content mismatch")
	}
}

// Decompressing a file that lacks the suffix must not replace or delete it,
// even with force.
func TestDecompressFileWithoutSuffixKeepsInput(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(src, []byte("some data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CompressFile(src, ".gz", 5, false, false, true, nil); err != nil {
		t.Fatal(err)
	}

	// A gzip file whose name does not end in .gz.
	path := filepath.Join(dir, "foo.txt")
	if err := os.Rename(src+".gz", path); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, force := range []bool{false, true} {
		if err := DecompressFile(path, ".gz", false, force, false, nil); err == nil {
			t.Fatalf("force=%v: expected an error", force)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("force=%v: input file lost: %v", force, err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("force=%v: input file modified", force)
		}
	}
}

func TestDecompressNamedRestoresHeaderNameAndMtime(t *testing.T) {
	headerMtime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	gzMtime := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)

	compressors := map[string]func(string) error{
		"compress": func(p string) error {
			return CompressFile(p, ".gz", 5, true, false, true, nil)
		},
		"xflate": func(p string) error {
			return XflateCompressFile(p, ".gz", 5, true, false, true, nil)
		},
	}

	for name, compress := range compressors {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			orig := filepath.Join(dir, "orig.txt")
			writeInput(t, orig, 0o644, headerMtime)
			if err := compress(orig); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(orig); err != nil {
				t.Fatal(err)
			}

			renamed := filepath.Join(dir, "renamed.gz")
			if err := os.Rename(orig+".gz", renamed); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(renamed, gzMtime, gzMtime); err != nil {
				t.Fatal(err)
			}

			// Without named: suffix removed, mtime from the .gz file.
			if err := DecompressFile(renamed, ".gz", true, false, false, nil); err != nil {
				t.Fatal(err)
			}
			checkMeta(t, filepath.Join(dir, "renamed"), 0o644, gzMtime)
			if _, err := os.Stat(orig); err == nil {
				t.Fatal("stored name restored without named")
			}

			// With named: name and mtime from the header.
			if err := DecompressFile(renamed, ".gz", true, false, true, nil); err != nil {
				t.Fatal(err)
			}
			checkMeta(t, orig, 0o644, headerMtime)
		})
	}
}

func TestHeaderBaseName(t *testing.T) {
	for in, want := range map[string]string{
		"":                 "",
		"file.txt":         "file.txt",
		"../../etc/passwd": "passwd",
		"/abs/path/x":      "x",
		"..":               "",
		".":                "",
		"/":                "",
		"dir/":             "dir",
	} {
		if got := headerBaseName(in); got != want {
			t.Errorf("headerBaseName(%q) = %q, want %q", in, got, want)
		}
	}
}

// An output name that is a hard link to the input must not replace it, even
// with force.
func TestDecompressFileRefusesToOverwriteInputViaLink(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "data")
	if err := os.WriteFile(src, []byte("some data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CompressFile(src, ".gz", 5, false, false, true, nil); err != nil {
		t.Fatal(err)
	}
	gz := src + ".gz"
	if err := os.Link(gz, src); err != nil {
		t.Skipf("hard links unsupported: %v", err)
	}
	before, err := os.ReadFile(gz)
	if err != nil {
		t.Fatal(err)
	}

	if err := DecompressFile(gz, ".gz", false, true, false, nil); err == nil {
		t.Fatal("expected an error")
	}
	after, err := os.ReadFile(gz)
	if err != nil {
		t.Fatalf("input lost: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("input modified")
	}
}

// Without a stored name or mtime, -N falls back to the defaults.
func TestDecompressNamedFallsBackWithoutHeaderFields(t *testing.T) {
	gzMtime := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)

	dir := t.TempDir()
	orig := filepath.Join(dir, "orig.txt")
	writeInput(t, orig, 0o644, time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC))
	if err := CompressFile(orig, ".gz", 5, true, false, false, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(orig); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(orig+".gz", gzMtime, gzMtime); err != nil {
		t.Fatal(err)
	}

	if err := DecompressFile(orig+".gz", ".gz", true, false, true, nil); err != nil {
		t.Fatal(err)
	}
	checkMeta(t, orig, 0o644, gzMtime)
}

// A hostile stored name must not let the output escape the input's directory.
func TestDecompressNamedIgnoresStoredDirectories(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "sub")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "evil.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := stdgzip.NewWriter(f)
	zw.Name = "../escape.txt"
	if _, err := zw.Write([]byte("payload\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := DecompressFile(path, ".gz", true, false, true, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.txt")); err != nil {
		t.Fatalf("output not written inside input directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "escape.txt")); err == nil {
		t.Fatal("output escaped the input directory")
	}
}

// A stored name that resolves to the input file (here via a hard link) must
// not replace it, even with force.
func TestDecompressNamedRefusesToOverwriteInputViaLink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := stdgzip.NewWriter(f)
	zw.Name = "alias"
	if _, err := zw.Write([]byte("payload\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, filepath.Join(dir, "alias")); err != nil {
		t.Skipf("hard links unsupported: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := DecompressFile(path, ".gz", false, true, true, nil); err == nil {
		t.Fatal("expected an error")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("input lost: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("input modified")
	}
}
