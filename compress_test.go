// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCompressFileNoNameHasZeroMtime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(path, []byte("some data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CompressFile(path, ".gz", 5, true, false, false, nil); err != nil {
		t.Fatal(err)
	}

	gz, err := os.ReadFile(path + ".gz")
	if err != nil {
		t.Fatal(err)
	}
	if mtime := gz[4:8]; !bytes.Equal(mtime, []byte{0, 0, 0, 0}) {
		t.Fatalf("mtime bytes = % x, want zero", mtime)
	}
}
