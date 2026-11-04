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

// The output is named by taking the suffix of the compressed file off, or by
// replacing it with .tar for the suffixes of compressed tar archives.
func TestDecompressFileSuffixes(t *testing.T) {
	tests := []struct {
		name  string
		file  string
		extra string
		want  string
	}{
		{"gz", "a.txt.gz", ".gz", "a.txt"},
		{"hyphen gz", "a-gz", ".gz", "a"},
		{"z", "a.z", ".gz", "a"},
		{"hyphen z", "a-z", ".gz", "a"},
		{"underscore z", "a_z", ".gz", "a"},
		{"upper case Z", "a.Z", ".gz", "a"},
		{"tgz", "a.tgz", ".gz", "a.tar"},
		{"taz", "a.taz", ".gz", "a.tar"},
		{"upper case gz", "A.GZ", ".gz", "A"},
		{"upper case tgz", "A.TGZ", ".gz", "A.tar"},
		{"extra suffix in another case", "a.FOO", ".foo", "a"},
		{"extra suffix", "a.foo", ".foo", "a"},
		{"extra suffix keeps the list", "a.tgz", ".foo", "a.tar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.file)
			if err := os.WriteFile(path, gzipBytes(t, "contents\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			if err := DecompressFile(path, tt.extra, true, false, false, nil); err != nil {
				t.Fatal(err)
			}

			got, err := os.ReadFile(filepath.Join(dir, tt.want))
			if err != nil || string(got) != "contents\n" {
				t.Errorf("%s = %q, %v; want the decompressed contents", tt.want, got, err)
			}
		})
	}
}

// A file whose suffix says nothing about it being compressed is an error, and
// is not touched.
func TestDecompressFileUnknownSuffix(t *testing.T) {
	for _, file := range []string{"a.txt", "a", "a.zip"} {
		t.Run(file, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, file)
			before := gzipBytes(t, "contents\n")
			if err := os.WriteFile(path, before, 0o644); err != nil {
				t.Fatal(err)
			}

			for _, force := range []bool{false, true} {
				err := DecompressFile(path, ".gz", false, force, false, nil)
				if err == nil || err.Error() != "unknown suffix" {
					t.Errorf("force=%v: error = %v, want %q", force, err, "unknown suffix")
				}
			}

			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Errorf("the file was changed or lost: %v", err)
			}
		})
	}
}

// A name stored in the file is used with -N, and then the suffix does not
// matter.
func TestDecompressFileNamedIgnoresUnknownSuffix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.foo")
	writeNamedGzip(t, path, "stored.txt", "contents\n")

	if err := DecompressFile(path, ".gz", true, false, true, nil); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "stored.txt"))
	if err != nil || string(got) != "contents\n" {
		t.Errorf("stored.txt = %q, %v; want the decompressed contents", got, err)
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

	err = DecompressFile(gz, ".gz", false, true, false, nil)
	if err == nil || err.Error() != "output would overwrite the input file" {
		t.Fatalf("error = %v, want %q", err, "output would overwrite the input file")
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
