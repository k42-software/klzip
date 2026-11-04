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

func TestIsTerminal(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "file"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	// /dev/null is a character device, as a terminal is, but it is not one.
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()

	tests := []struct {
		name   string
		stream any
	}{
		{"buffer", &bytes.Buffer{}},
		{"regular file", file},
		{"null device", null},
		{"nil", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if isTerminal(tt.stream) {
				t.Error("isTerminal = true, want false")
			}
		})
	}
}
