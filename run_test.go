// @author: Brian Wojtczak
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunUnknownMode(t *testing.T) {
	var out bytes.Buffer
	if Run(Options{Mode: Mode(99), Files: []string{"f"}, Jobs: 1}, &out) {
		t.Error("expected a failure")
	}
	if !strings.Contains(out.String(), "unknown mode 99") {
		t.Errorf("output = %q, want it to mention the unknown mode", out.String())
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

			var out bytes.Buffer
			ok := Run(Options{
				Mode: ModeCompress, Files: []string{path}, Jobs: 1, Suffix: tt.suffix,
				Level: 5, Keep: true, Force: tt.force, Named: true, Rsyncable: tt.rsyncable,
			}, &out)

			if !ok {
				t.Errorf("not ok: %q", out.String())
			}

			_, err := os.Stat(path + tt.suffix)
			if created := err == nil; created == tt.wantSkipped {
				t.Errorf("output created = %v, want %v", created, !tt.wantSkipped)
			}

			want := ""
			if tt.wantSkipped {
				want = "klzip: " + path + ": already has " + tt.suffix + " suffix -- unchanged\n"
			}
			if out.String() != want {
				t.Errorf("output = %q, want %q", out.String(), want)
			}
		})
	}
}
