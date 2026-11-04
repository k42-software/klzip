// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeInput(t *testing.T, path string, mode os.FileMode, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("some data\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func checkMeta(t *testing.T, path string, mode os.FileMode, mtime time.Time) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != mode {
		t.Errorf("%s: mode = %o, want %o", filepath.Base(path), got, mode)
	}
	if got := info.ModTime(); !got.Equal(mtime) {
		t.Errorf("%s: mtime = %v, want %v", filepath.Base(path), got, mtime)
	}
}

func TestOutputPreservesModeAndMtime(t *testing.T) {
	const mode = os.FileMode(0o640)
	mtime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)

	compressors := map[string]func(string) error{
		"compress": func(p string) error {
			return CompressFile(p, ".gz", 5, true, false, true, false)
		},
		"xflate": func(p string) error {
			return XflateCompressFile(p, ".gz", 5, true, false, true, false)
		},
	}

	for name, compress := range compressors {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "data.txt")
			writeInput(t, path, mode, mtime)

			if err := compress(path); err != nil {
				t.Fatal(err)
			}
			checkMeta(t, path+".gz", mode, mtime)

			// Give the .gz file different metadata, then decompress it.
			gzMtime := time.Date(2021, 6, 7, 8, 9, 10, 0, time.UTC)
			if err := os.Chmod(path+".gz", 0o604); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(path+".gz", gzMtime, gzMtime); err != nil {
				t.Fatal(err)
			}
			if err := DecompressFile(path+".gz", ".gz", true, true, false); err != nil {
				t.Fatal(err)
			}
			checkMeta(t, path, 0o604, gzMtime)
		})
	}
}
