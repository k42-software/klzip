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

func TestDecompressFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	want := bytes.Repeat([]byte("hello world\n"), 1000)
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := CompressFile(path, ".gz", 5, false, false, true, false); err != nil {
		t.Fatal(err)
	}
	if err := DecompressFile(path+".gz", ".gz", false, false, false); err != nil {
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
	if err := CompressFile(src, ".gz", 5, false, false, true, false); err != nil {
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
		if err := DecompressFile(path, ".gz", false, force, false); err == nil {
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

// An output name that is a hard link to the input must not replace it, even
// with force.
func TestDecompressFileRefusesToOverwriteInputViaLink(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "data")
	if err := os.WriteFile(src, []byte("some data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CompressFile(src, ".gz", 5, false, false, true, false); err != nil {
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

	if err := DecompressFile(gz, ".gz", false, true, false); err == nil {
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
