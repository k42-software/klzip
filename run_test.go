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

// A file that has the suffix of a compressed file, or the suffix of the
// options, is left alone, unless forced.
func TestRunCompressSkipsFilesWithSuffix(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		suffix    string
		force     bool
		rsyncable bool

		// skippedFor is the suffix that the file is skipped for, or "" if it is
		// compressed.
		skippedFor string
	}{
		{"plain file", "a.txt", ".gz", false, false, ""},
		{"has suffix", "a.txt.gz", ".gz", false, false, ".gz"},
		{"has suffix and rsyncable", "a.txt.gz", ".gz", false, true, ".gz"},
		{"has suffix and force", "a.txt.gz", ".gz", true, false, ""},
		{"tgz", "a.tgz", ".gz", false, false, ".tgz"},
		{"z", "a.z", ".gz", false, false, ".z"},
		{"custom suffix", "a.foo", ".foo", false, false, ".foo"},
		{"custom suffix keeps the list", "a.gz", ".foo", false, false, ".gz"},
		{"custom suffix keeps the list for tgz", "a.tgz", ".foo", false, false, ".tgz"},
		{"upper case Z", "a.Z", ".gz", false, false, ".Z"},
		{"upper case gz", "a.GZ", ".gz", false, false, ".GZ"},
		{"mixed case tgz", "a.Tgz", ".gz", false, false, ".Tgz"},
		{"extra suffix in another case", "a.FOO", ".foo", false, false, ".FOO"},
		{"taz", "a.taz", ".gz", false, false, ".taz"},
		{"unknown suffix", "a.zip", ".gz", false, false, ""},
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
			if created, wantCreated := err == nil, tt.skippedFor == ""; created != wantCreated {
				t.Errorf("output created = %v, want %v", created, wantCreated)
			}

			want := ""
			if tt.skippedFor != "" {
				want = "klzip: " + path + ": already has " + tt.skippedFor + " suffix -- unchanged\n"
			}
			if errs.String() != want {
				t.Errorf("stderr = %q, want %q", errs.String(), want)
			}
		})
	}
}
