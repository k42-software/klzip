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

func TestXflateCompressFileNoNameHasZeroMtime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	want := bytes.Repeat([]byte("hello world\n"), 1000)
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := XflateCompressFile(path, ".gz", 5, true, false, false, false); err != nil {
		t.Fatal(err)
	}

	gz, err := os.ReadFile(path + ".gz")
	if err != nil {
		t.Fatal(err)
	}
	if mtime := gz[4:8]; !bytes.Equal(mtime, []byte{0, 0, 0, 0}) {
		t.Fatalf("mtime bytes = % x, want zero", mtime)
	}

	// The output must still be a valid gzip file.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := DecompressFile(path+".gz", ".gz", false, false, false, false); err != nil {
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
