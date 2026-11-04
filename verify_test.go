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

// writeGzip writes a gzip file in dir and returns its path and contents.
func writeGzip(t *testing.T, dir string) (string, []byte) {
	t.Helper()

	path := filepath.Join(dir, "data.txt")
	data := bytes.Repeat([]byte("hello world\n"), 1000)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CompressFile(path, ".gz", 5, false, false, true, nil); err != nil {
		t.Fatal(err)
	}

	gz, err := os.ReadFile(path + ".gz")
	if err != nil {
		t.Fatal(err)
	}
	return path + ".gz", gz
}

func TestVerifyFile(t *testing.T) {
	tests := []struct {
		name string
		// modify returns the contents to verify, given a valid gzip file.
		modify  func(gz []byte) []byte
		wantErr bool
	}{
		{"valid", func(gz []byte) []byte { return gz }, false},
		{"bad checksum", func(gz []byte) []byte {
			// The checksum is in the first byte of the 8 byte trailer.
			bad := bytes.Clone(gz)
			bad[len(bad)-8] ^= 0xff
			return bad
		}, true},
		{"truncated", func(gz []byte) []byte { return gz[:len(gz)-10] }, true},
		{"not gzip", func([]byte) []byte { return []byte("just some text\n") }, true},
		{"empty", func([]byte) []byte { return nil }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, gz := writeGzip(t, t.TempDir())
			if err := os.WriteFile(path, tt.modify(gz), 0o644); err != nil {
				t.Fatal(err)
			}

			err := VerifyFile(path, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, want error: %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyFileMissing(t *testing.T) {
	if err := VerifyFile(filepath.Join(t.TempDir(), "missing.gz"), nil); err == nil {
		t.Error("expected an error")
	}
}

// Verifying must leave the file as it was.
func TestVerifyFileLeavesInput(t *testing.T) {
	path, gz := writeGzip(t, t.TempDir())

	if err := VerifyFile(path, nil); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gz, after) {
		t.Error("file changed")
	}
}
