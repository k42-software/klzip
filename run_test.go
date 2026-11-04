// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useStreams replaces the standard streams for the length of a test.  Stdin
// reads from in.  It returns what is written to stdout and to stderr.
func useStreams(t *testing.T, in io.Reader) (out, errs *bytes.Buffer) {
	t.Helper()

	oldIn, oldOut, oldErr := stdin, stdout, stderr
	t.Cleanup(func() { stdin, stdout, stderr = oldIn, oldOut, oldErr })

	out, errs = &bytes.Buffer{}, &bytes.Buffer{}
	stdin, stdout, stderr = in, out, errs

	return out, errs
}

func TestRunUnknownMode(t *testing.T) {
	_, errs := useStreams(t, strings.NewReader(""))
	if Run(Options{Mode: Mode(99), Files: []string{"f"}, Jobs: 1}) {
		t.Error("expected a failure")
	}
	if !strings.Contains(errs.String(), "unknown mode 99") {
		t.Errorf("stderr = %q, want it to mention the unknown mode", errs.String())
	}
}

// A file that already has the suffix is left alone, unless forced.
func TestRunCompressSkipsFilesWithSuffix(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		suffix      string
		force       bool
		rsyncable   bool
		wantSkipped bool
	}{
		{"plain file", "a.txt", ".gz", false, false, false},
		{"has suffix", "a.txt.gz", ".gz", false, false, true},
		{"has suffix and rsyncable", "a.txt.gz", ".gz", false, true, true},
		{"has suffix and force", "a.txt.gz", ".gz", true, false, false},
		{"custom suffix", "a.foo", ".foo", false, false, true},
		{"other suffix", "a.gz", ".foo", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tt.file)
			if err := os.WriteFile(path, []byte("some data\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			_, errs := useStreams(t, strings.NewReader(""))
			ok := Run(Options{
				Mode: ModeCompress, Files: []string{path}, Jobs: 1, Suffix: tt.suffix,
				Level: 5, Keep: true, Force: tt.force, Named: true, Rsyncable: tt.rsyncable,
			})

			if !ok {
				t.Errorf("not ok: %q", errs.String())
			}

			_, err := os.Stat(path + tt.suffix)
			if created := err == nil; created == tt.wantSkipped {
				t.Errorf("output created = %v, want %v", created, !tt.wantSkipped)
			}

			want := ""
			if tt.wantSkipped {
				want = "klzip: " + path + ": already has " + tt.suffix + " suffix -- unchanged\n"
			}
			if errs.String() != want {
				t.Errorf("stderr = %q, want %q", errs.String(), want)
			}
		})
	}
}
